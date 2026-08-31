package k8smonitor

import (
	"context"
	"crypto/x509"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/fabianoflorentino/certificate-validate/internal/certificate"
)

func TestNewMonitorWithKubeconfig(t *testing.T) {
	kcfg := writeMinimalKubeconfig(t)

	m, err := NewMonitor(Config{Kubeconfig: kcfg})
	if err != nil {
		t.Fatalf("NewMonitor returned error: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil monitor")
	}
	if m.renewer != nil {
		t.Error("expected no renewer when renew-threshold is zero")
	}
}

func TestNewMonitorWithRenewer(t *testing.T) {
	kcfg := writeMinimalKubeconfig(t)

	m, err := NewMonitor(Config{
		Kubeconfig:     kcfg,
		RenewThreshold: 15,
	})
	if err != nil {
		t.Fatalf("NewMonitor returned error: %v", err)
	}
	if m.renewer == nil {
		t.Fatal("expected renewer to be configured when renew-threshold is set")
	}
}

func TestNewMonitorWebhookConfigured(t *testing.T) {
	kcfg := writeMinimalKubeconfig(t)

	m, err := NewMonitor(Config{Kubeconfig: kcfg, WebhookURL: "https://example.com/hook"})
	if err != nil {
		t.Fatalf("NewMonitor returned error: %v", err)
	}
	if m.webhook == nil {
		t.Fatal("expected webhook to be configured")
	}
}

func TestMonitorScan(t *testing.T) {
	client := fake.NewSimpleClientset(scanTLSecret(t), scanIngress())
	m := &Monitor{
		client:     &Client{Clientset: client},
		analyzer:   NewAnalyzer(false),
		namespaces: nil,
	}

	certs, err := m.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	var sawSecret, sawIngress bool
	for _, c := range certs {
		switch c.K8sKind {
		case KindSecret:
			sawSecret = true
		case KindIngress:
			sawIngress = true
		}
	}
	if !sawSecret {
		t.Error("expected a Secret-kind certificate from scan")
	}
	if !sawIngress {
		t.Error("expected an Ingress-kind certificate from scan")
	}
}

func TestMonitorScanNamespaced(t *testing.T) {
	client := fake.NewSimpleClientset(scanTLSecret(t))
	m := &Monitor{
		client:     &Client{Clientset: client},
		analyzer:   NewAnalyzer(false),
		namespaces: []string{"default"},
	}

	certs, err := m.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	if len(certs) == 0 {
		t.Fatal("expected at least one certificate from namespaced scan")
	}
}

func TestMonitorRunSingleScan(t *testing.T) {
	client := fake.NewSimpleClientset(scanTLSecret(t))
	m := &Monitor{
		client:     &Client{Clientset: client},
		analyzer:   NewAnalyzer(false),
		namespaces: nil,
	}

	if err := m.Run(context.Background(), 0); err != nil {
		t.Fatalf("Run single scan returned error: %v", err)
	}
}

func TestMonitorWatchLoopStopsOnCancel(t *testing.T) {
	client := fake.NewSimpleClientset(scanTLSecret(t))
	m := &Monitor{
		client:     &Client{Clientset: client},
		analyzer:   NewAnalyzer(false),
		namespaces: nil,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := m.Run(ctx, 10*time.Millisecond); err != nil {
			t.Errorf("watchLoop returned error: %v", err)
		}
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()
	wg.Wait()
}

func TestMonitorRenewCerts(t *testing.T) {
	const ns, name = "default", "app-tls"
	oldBundle := renewCertBundle(t, 10, "app.example.com", 3)  // needs renewal
	newBundle := renewCertBundle(t, 20, "app.example.com", 90) // renewed, valid
	oldSerial := renewSerial(t, oldBundle)

	client := fake.NewSimpleClientset(scanTLSecret(t))
	client.PrependReactor("get", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, renewSecret(ns, name, newBundle, nil), nil
	})

	emitter := &fakeEmitter{}
	renewer := NewRenewer(client, NewAnalyzer(false), 15, 2*time.Second, 80).
		WithEventEmitter(emitter).
		WithRevocationCheck(func(leaf, issuer *x509.Certificate, ocsp, crl []string) certificate.RevocationStatus {
			return certificate.RevocationGood
		})

	m := &Monitor{
		client:     &Client{Clientset: client},
		analyzer:   NewAnalyzer(false),
		namespaces: nil,
		renewer:    renewer,
	}

	certs := []*K8sCertificate{{
		Certificate:  certificate.Certificate{DaysLeft: 3},
		K8sNamespace: ns,
		K8sName:      name,
		K8sKind:      KindSecret,
		SerialNumber: oldSerial,
	}}
	m.renewCerts(context.Background(), certs)
}

func scanTLSecret(t *testing.T) *corev1.Secret {
	t.Helper()
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "app-tls",
			Labels:    map[string]string{"app": "demo"},
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{"tls.crt": renewCertBundle(t, 10, "app.example.com", 90)},
	}
}

func scanIngress() *networkingv1.Ingress {
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "app-ing",
		},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{{
				Hosts:      []string{"app.example.com"},
				SecretName: "app-tls",
			}},
		},
	}
}

func writeMinimalKubeconfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	kcfg := filepath.Join(dir, "kubeconfig")
	content := `apiVersion: v1
kind: Config
clusters:
- name: test
  cluster:
    server: https://127.0.0.1:1
contexts:
- name: test
  context:
    cluster: test
    user: test
current-context: test
users:
- name: test
  user: {}
`
	if err := os.WriteFile(kcfg, []byte(content), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	return kcfg
}
