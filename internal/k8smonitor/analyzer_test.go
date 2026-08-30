package k8smonitor

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

func TestParseBundle(t *testing.T) {
	pemBytes := newTestCertBundle(t, 30*24*time.Hour)

	a := NewAnalyzer(false)
	leaf, chain, err := a.ParseBundle(pemBytes)
	if err != nil {
		t.Fatalf("ParseBundle returned error: %v", err)
	}
	if leaf == nil {
		t.Fatal("expected leaf certificate, got nil")
	}
	if leaf.Subject.CommonName != "test.example.com" {
		t.Errorf("got CN %q; want %q", leaf.Subject.CommonName, "test.example.com")
	}
	if len(chain) != 0 {
		t.Errorf("got chain length %d; want 0", len(chain))
	}
}

func TestParseBundleInvalid(t *testing.T) {
	a := NewAnalyzer(false)
	_, _, err := a.ParseBundle([]byte("not a valid pem"))
	if err == nil {
		t.Fatal("expected error for invalid bundle, got nil")
	}
}

func TestParseBundleEmpty(t *testing.T) {
	a := NewAnalyzer(false)
	_, _, err := a.ParseBundle(nil)
	if err == nil {
		t.Fatal("expected error for empty bundle, got nil")
	}
}

func TestAnalyze(t *testing.T) {
	pemBytes := newTestCertBundle(t, 30*24*time.Hour)
	a := NewAnalyzer(false)
	leaf, chain, err := a.ParseBundle(pemBytes)
	if err != nil {
		t.Fatalf("ParseBundle returned error: %v", err)
	}

	k8sCert := a.Analyze(leaf, chain, "default", "app-tls", KindSecret,
		map[string]string{"app": "demo"}, nil)

	if k8sCert.K8sNamespace != "default" {
		t.Errorf("got namespace %q; want %q", k8sCert.K8sNamespace, "default")
	}
	if k8sCert.K8sName != "app-tls" {
		t.Errorf("got name %q; want %q", k8sCert.K8sName, "app-tls")
	}
	if k8sCert.K8sKind != KindSecret {
		t.Errorf("got kind %q; want %q", k8sCert.K8sKind, KindSecret)
	}
	if k8sCert.Hostname != "app-tls" {
		t.Errorf("got hostname %q; want %q", k8sCert.Hostname, "app-tls")
	}
	if k8sCert.DaysLeft <= 0 || k8sCert.DaysLeft > 31 {
		t.Errorf("got days left %d; want between 1 and 31", k8sCert.DaysLeft)
	}
	if k8sCert.SerialNumber == "" {
		t.Error("expected non-empty serial number")
	}
	if k8sCert.RenewalState != RenewalStateNone {
		t.Errorf("got renewal state %q; want %q", k8sCert.RenewalState, RenewalStateNone)
	}
}

func TestAnalyzeWithChain(t *testing.T) {
	leafPEM := newTestCertBundle(t, 30*24*time.Hour)

	// Build a bundle containing leaf + intermediate
	intermediatePEM := newTestCertBundleWithCN(t, 365*24*time.Hour, "intermediate.example.com")
	bundle := append(leafPEM, intermediatePEM...)

	a := NewAnalyzer(false)
	leaf, chain, err := a.ParseBundle(bundle)
	if err != nil {
		t.Fatalf("ParseBundle returned error: %v", err)
	}
	if len(chain) != 1 {
		t.Fatalf("got chain length %d; want 1", len(chain))
	}

	k8sCert := a.Analyze(leaf, chain, "default", "app-tls", KindSecret, nil, nil)
	if len(k8sCert.Chain) != 2 {
		t.Errorf("got chain entries %d; want 2", len(k8sCert.Chain))
	}
}

func newTestCertBundle(t *testing.T, validity time.Duration) []byte {
	return newTestCertBundleWithCN(t, validity, "test.example.com")
}

func newTestCertBundleWithCN(t *testing.T, validity time.Duration, cn string) []byte {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(validity),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"test.example.com"},
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}
