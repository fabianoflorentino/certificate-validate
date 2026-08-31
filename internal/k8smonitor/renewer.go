package k8smonitor

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"github.com/fabianoflorentino/certificate-validate/internal/certificate"
	"github.com/fabianoflorentino/certificate-validate/internal/revocation"
)

// forceRenewAnnotation is the cert-manager annotation that triggers a new
// issuance for the Secret it is applied to.
const forceRenewAnnotation = "cert-manager.io/force-renew"

// RevocationChecker reports the revocation status of a certificate. It is
// injectable so tests can avoid real OCSP/CRL network calls.
type RevocationChecker func(leaf, issuer *x509.Certificate, ocspServers, crlURLs []string) certificate.RevocationStatus

// EventEmitter records Kubernetes events on cluster resources. Both the
// client-go record.EventRecorderLogger and a test fake satisfy this interface.
type EventEmitter interface {
	Event(object runtime.Object, eventtype, reason, message string)
}

// RenewalOutcome reports the result of an auto-renewal attempt.
type RenewalOutcome struct {
	State   RenewalState
	Reason  string
	Renewed bool
}

// Renewer performs auto-renewal of TLS certificates that are close to
// expiration by annotating the owning Secret with the cert-manager
// force-renew annotation, waiting for the serial to change, then validating
// the renewed certificate. It records progress as Kubernetes events.
type Renewer struct {
	client          kubernetes.Interface
	analyzer        *Analyzer
	threshold       int
	timeout         time.Duration
	minRenewedDays  int
	emitter         EventEmitter
	checkRevocation bool
	revocationCheck RevocationChecker
	now             func() time.Time
}

// NewRenewer builds a Renewer. threshold is the days-left at or below which a
// certificate is considered for renewal. timeout bounds how long to wait for
// cert-manager to re-issue. minRenewedDays is the minimum remaining validity
// the renewed certificate must have to be accepted.
func NewRenewer(client kubernetes.Interface, analyzer *Analyzer, threshold int, timeout time.Duration, minRenewedDays int) *Renewer {
	return &Renewer{
		client:          client,
		analyzer:        analyzer,
		threshold:       threshold,
		timeout:         timeout,
		minRenewedDays:  minRenewedDays,
		revocationCheck: revocation.Check,
		now:             time.Now,
	}
}

// WithEventEmitter sets the Kubernetes event emitter used to record progress.
func (r *Renewer) WithEventEmitter(e EventEmitter) *Renewer {
	r.emitter = e
	return r
}

// WithRevocationCheck overrides the revocation checker (used in tests).
func (r *Renewer) WithRevocationCheck(fn RevocationChecker) *Renewer {
	if fn != nil {
		r.revocationCheck = fn
	}
	return r
}

// WithRevocation enables strict post-renewal revocation validation.
func (r *Renewer) WithRevocation(enabled bool) *Renewer {
	r.checkRevocation = enabled
	return r
}

// NeedsRenewal reports whether a certificate is a candidate for renewal:
// at or below the threshold but not yet expired.
func (r *Renewer) NeedsRenewal(c *K8sCertificate) bool {
	if c == nil {
		return false
	}
	return c.DaysLeft > 0 && c.DaysLeft <= r.threshold
}

// Renew attempts to renew the certificate held by the given TLS Secret.
// previousSerial is the serial observed before renewal, used to detect a
// successful re-issuance.
func (r *Renewer) Renew(ctx context.Context, ns, name, previousSerial string) RenewalOutcome {
	outcome := RenewalOutcome{State: RenewalStateFailed}

	if err := r.annotateSecret(ctx, ns, name); err != nil {
		outcome.Reason = fmt.Sprintf("annotate secret: %v", err)
		r.emit(ns, name, corev1.EventTypeWarning, "CertRenewFailed", outcome.Reason)
		return outcome
	}

	renewed, err := r.waitForRenewal(ctx, ns, name, previousSerial)
	if err != nil {
		outcome.State = RenewalStateStuck
		outcome.Reason = fmt.Sprintf("wait for renewal: %v", err)
		r.emit(ns, name, corev1.EventTypeWarning, "CertRenewFailed", outcome.Reason)
		return outcome
	}

	if err := r.validateRenewed(ctx, ns, name, renewed); err != nil {
		outcome.Reason = fmt.Sprintf("validation: %v", err)
		r.emit(ns, name, corev1.EventTypeWarning, "CertRenewValidation", outcome.Reason)
		return outcome
	}

	outcome.State = RenewalStateSuccess
	outcome.Reason = "certificate renewed and validated"
	outcome.Renewed = true
	r.emit(ns, name, corev1.EventTypeNormal, "CertRenewed", "certificate renewed and validated")

	return outcome
}

// annotateSecret sets the cert-manager force-renew annotation on the Secret to
// trigger a new issuance.
func (r *Renewer) annotateSecret(ctx context.Context, ns, name string) error {
	secret, err := r.client.CoreV1().Secrets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get secret %s/%s: %w", ns, name, err)
	}

	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	secret.Annotations[forceRenewAnnotation] = "true"

	_, err = r.client.CoreV1().Secrets(ns).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("annotate secret %s/%s: %w", ns, name, err)
	}
	return nil
}

// waitForRenewal polls the Secret until its certificate serial differs from
// the previously observed one, or the timeout elapses. A timeout is reported
// as a stuck issuance.
func (r *Renewer) waitForRenewal(ctx context.Context, ns, name, previousSerial string) (map[string][]byte, error) {
	if previousSerial == "" {
		return nil, fmt.Errorf("previous serial unavailable")
	}

	var current map[string][]byte
	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, r.timeout, true, func(ctx context.Context) (bool, error) {
		secret, err := r.client.CoreV1().Secrets(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		current = secret.Data
		newSerial, err := serialOf(current["tls.crt"])
		if err != nil {
			return false, nil
		}
		return newSerial != previousSerial, nil
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("timeout; certificate serial unchanged (stuck issuance?)")
		}
		return nil, err
	}

	return current, nil
}

// validateRenewed verifies the renewed certificate has enough remaining
// validity and, when strict revocation checking is enabled, is not revoked.
func (r *Renewer) validateRenewed(ctx context.Context, ns, name string, data map[string][]byte) error {
	leaf, chain, err := r.analyzer.ParseBundle(data["tls.crt"])
	if err != nil {
		return fmt.Errorf("parse renewed certificate: %w", err)
	}

	if days := certificate.FromX509(leaf, name, 443).DaysLeft; days < r.minRenewedDays {
		return fmt.Errorf("renewed certificate only has %d days (expected at least %d)", days, r.minRenewedDays)
	}

	if r.checkRevocation {
		issuer := findIssuer(chain)
		status := r.revocationCheck(leaf, issuer, leaf.OCSPServer, leaf.CRLDistributionPoints)
		if status != certificate.RevocationGood {
			return fmt.Errorf("renewed certificate revocation status %q (expected good)", status)
		}
	}

	return nil
}

// emit records a Kubernetes event when an emitter is configured.
func (r *Renewer) emit(ns, name, eventType, reason, message string) {
	if r.emitter == nil {
		return
	}
	obj := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
	}
	r.emitter.Event(obj, eventType, reason, message)
	slog.Info("renewer event emitted", "namespace", ns, "name", name, "reason", reason)
}

// serialOf extracts the leaf certificate serial number from a PEM bundle.
func serialOf(pemBytes []byte) (string, error) {
	if len(pemBytes) == 0 {
		return "", fmt.Errorf("empty bundle")
	}
	leaf, _, err := newAnalyzerForSerial().ParseBundle(pemBytes)
	if err != nil {
		return "", err
	}
	return leaf.SerialNumber.String(), nil
}

func newAnalyzerForSerial() *Analyzer {
	return NewAnalyzer(false)
}
