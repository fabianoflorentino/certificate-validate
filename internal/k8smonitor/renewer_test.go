package k8smonitor

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/fabianoflorentino/certificate-validate/internal/certificate"
)

type fakeEmitter struct {
	events []string
}

func (f *fakeEmitter) Event(_ runtime.Object, _ string, reason, _ string) {
	f.events = append(f.events, reason)
}

func TestNeedsRenewal(t *testing.T) {
	r := NewRenewer(nil, NewAnalyzer(false), 15, time.Minute, 80)

	cases := []struct {
		name string
		cert *K8sCertificate
		want bool
	}{
		{"nil cert", nil, false},
		{"expired", &K8sCertificate{Certificate: certificate.Certificate{DaysLeft: 0}}, false},
		{"above threshold", &K8sCertificate{Certificate: certificate.Certificate{DaysLeft: 30}}, false},
		{"at threshold", &K8sCertificate{Certificate: certificate.Certificate{DaysLeft: 15}}, true},
		{"below threshold", &K8sCertificate{Certificate: certificate.Certificate{DaysLeft: 7}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := r.NeedsRenewal(tc.cert); got != tc.want {
				t.Errorf("NeedsRenewal(%+v) = %v; want %v", tc.cert, got, tc.want)
			}
		})
	}
}

func TestRenewSuccess(t *testing.T) {
	ctx := context.Background()
	const ns, name = "default", "app-tls"

	oldBundle := renewCertBundle(t, 10, "app.example.com", 90)
	newBundle := renewCertBundle(t, 20, "app.example.com", 90)
	oldSerial := renewSerial(t, oldBundle)

	renamed := map[string][]byte{"tls.crt": newBundle}
	client := fake.NewSimpleClientset(renewSecret(ns, name, oldBundle, nil))
	client.PrependReactor("get", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, renewSecret(ns, name, renamed["tls.crt"], map[string]string{"cert-manager.io/force-renew": "true"}), nil
	})

	emitter := &fakeEmitter{}
	r := NewRenewer(client, NewAnalyzer(false), 15, 2*time.Second, 80).
		WithEventEmitter(emitter).
		WithRevocationCheck(func(leaf, issuer *x509.Certificate, ocsp, crl []string) certificate.RevocationStatus {
			return certificate.RevocationGood
		}).
		WithRevocation(true)

	out := r.Renew(ctx, ns, name, oldSerial)
	if !out.Renewed {
		t.Fatalf("expected renewal success, got state=%s reason=%s", out.State, out.Reason)
	}
	if out.State != RenewalStateSuccess {
		t.Errorf("got state %q; want %q", out.State, RenewalStateSuccess)
	}
	found := false
	for _, e := range emitter.events {
		if e == "CertRenewed" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected CertRenewed event, got %v", emitter.events)
	}
}

func TestRenewStuckIssuance(t *testing.T) {
	ctx := context.Background()
	const ns, name = "default", "app-tls"

	oldBundle := renewCertBundle(t, 10, "app.example.com", 3)
	oldSerial := renewSerial(t, oldBundle)

	client := fake.NewSimpleClientset(renewSecret(ns, name, oldBundle, nil))
	client.PrependReactor("get", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, renewSecret(ns, name, oldBundle, nil), nil
	})

	emitter := &fakeEmitter{}
	r := NewRenewer(client, NewAnalyzer(false), 15, 300*time.Millisecond, 80).
		WithEventEmitter(emitter)

	out := r.Renew(ctx, ns, name, oldSerial)
	if out.Renewed {
		t.Fatal("expected renewal to fail on stuck issuance")
	}
	if out.State != RenewalStateStuck {
		t.Errorf("got state %q; want %q", out.State, RenewalStateStuck)
	}
	if !strings.Contains(out.Reason, "stuck") {
		t.Errorf("expected stuck reason, got %q", out.Reason)
	}
}

func TestRenewValidationTooShort(t *testing.T) {
	ctx := context.Background()
	const ns, name = "default", "app-tls"

	oldBundle := renewCertBundle(t, 10, "app.example.com", 90)
	newBundle := renewCertBundle(t, 20, "app.example.com", 10) // only 10 days left
	oldSerial := renewSerial(t, oldBundle)

	client := fake.NewSimpleClientset(renewSecret(ns, name, oldBundle, nil))
	client.PrependReactor("get", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, renewSecret(ns, name, newBundle, nil), nil
	})

	r := NewRenewer(client, NewAnalyzer(false), 15, 2*time.Second, 80)
	out := r.Renew(ctx, ns, name, oldSerial)
	if out.Renewed {
		t.Fatal("expected renewal to fail validation")
	}
	if !strings.Contains(out.Reason, "only has") {
		t.Errorf("expected short-validity reason, got %q", out.Reason)
	}
}

func TestRenewValidationRevoked(t *testing.T) {
	ctx := context.Background()
	const ns, name = "default", "app-tls"

	oldBundle := renewCertBundle(t, 10, "app.example.com", 90)
	newBundle := renewCertBundle(t, 20, "app.example.com", 90)
	oldSerial := renewSerial(t, oldBundle)

	client := fake.NewSimpleClientset(renewSecret(ns, name, oldBundle, nil))
	client.PrependReactor("get", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, renewSecret(ns, name, newBundle, nil), nil
	})

	r := NewRenewer(client, NewAnalyzer(false), 15, 2*time.Second, 80).
		WithRevocationCheck(func(leaf, issuer *x509.Certificate, ocsp, crl []string) certificate.RevocationStatus {
			return certificate.RevocationRevoked
		}).
		WithRevocation(true)

	out := r.Renew(ctx, ns, name, oldSerial)
	if out.Renewed {
		t.Fatal("expected renewal to fail revoked validation")
	}
	if !strings.Contains(out.Reason, "revocation status") {
		t.Errorf("expected revoked reason, got %q", out.Reason)
	}
}

func TestRenewAnnotateError(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset() // no secret present

	emitter := &fakeEmitter{}
	r := NewRenewer(client, NewAnalyzer(false), 15, time.Second, 80).WithEventEmitter(emitter)

	out := r.Renew(ctx, "default", "missing", "1234")
	if out.Renewed {
		t.Fatal("expected renewal to fail when secret is missing")
	}
	if !strings.Contains(out.Reason, "annotate") {
		t.Errorf("expected annotate reason, got %q", out.Reason)
	}
	if len(emitter.events) == 0 || emitter.events[0] != "CertRenewFailed" {
		t.Errorf("expected CertRenewFailed event, got %v", emitter.events)
	}
}

func TestRenewNoPreviousSerial(t *testing.T) {
	ctx := context.Background()
	const ns, name = "default", "app-tls"
	client := fake.NewSimpleClientset(renewSecret(ns, name, renewCertBundle(t, 10, "app.example.com", 90), nil))

	r := NewRenewer(client, NewAnalyzer(false), 15, 2*time.Second, 80)
	out := r.Renew(ctx, ns, name, "")
	if out.Renewed {
		t.Fatal("expected renewal to fail without a previous serial")
	}
}

func renewSecret(ns, name string, bundle []byte, annotations map[string]string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   ns,
			Name:        name,
			Annotations: annotations,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{"tls.crt": bundle},
	}
}

func renewCertBundle(t *testing.T, serial int64, cn string, days int) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Duration(days) * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		DNSNames:     []string{cn},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func renewSerial(t *testing.T, bundle []byte) string {
	t.Helper()
	leaf, _, err := NewAnalyzer(false).ParseBundle(bundle)
	if err != nil {
		t.Fatalf("parse bundle: %v", err)
	}
	return leaf.SerialNumber.String()
}
