package formatter

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/fabianoflorentino/certificate-validate/internal/certificate"
)

func TestNew(t *testing.T) {
	f := New()
	if f == nil {
		t.Fatal("New() returned nil")
	}
	if f.indent != "  " {
		t.Errorf("indent = %q; want %q", f.indent, "  ")
	}
}

func TestFormat(t *testing.T) {
	cert := &certificate.Certificate{
		CommonName:      "example.com",
		SubjectAltNames: []string{"example.com", "www.example.com"},
		Issuer:          "Test CA",
		Type:            "Domain Validation (DV) Web Server SSL Digital Certificate",
		NotBefore:       "2024-01-01 00:00:00",
		NotAfter:        "2025-01-01 00:00:00",
		DaysLeft:        365,
		CRLDistributionPoints: []string{"http://crl.example.com/ca.crl"},
		Hostname:        "example.com",
		Port:            443,
		TLSVersion:      "TLS 1.3",
		CipherSuite:     "TLS_AES_256_GCM_SHA384",
		Chain: []certificate.ChainEntry{
			{
				Subject:     "CN=example.com,O=Example Inc",
				Issuer:      "CN=Test CA,O=Test Org",
				NotAfter:    "2025-01-01 00:00:00",
				Fingerprint: "aabbccdd11223344",
			},
		},
	}

	f := New()
	data, err := f.Format(cert)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	if len(data) == 0 {
		t.Fatal("Format() returned empty data")
	}

	// Verify valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Format() returned invalid JSON: %v", err)
	}

	// Verify expected fields are present
	if result["commonName"] != "example.com" {
		t.Errorf("commonName = %v; want %v", result["commonName"], "example.com")
	}

	if result["issuer"] != "Test CA" {
		t.Errorf("issuer = %v; want %v", result["issuer"], "Test CA")
	}

	if result["type"] != "Domain Validation (DV) Web Server SSL Digital Certificate" {
		t.Errorf("type = %v; want %v", result["type"], "Domain Validation (DV) Web Server SSL Digital Certificate")
	}

	if result["hostname"] != "example.com" {
		t.Errorf("hostname = %v; want %v", result["hostname"], "example.com")
	}

	if result["port"] != float64(443) {
		t.Errorf("port = %v; want %v", result["port"], 443)
	}

	if result["daysLeft"] != float64(365) {
		t.Errorf("daysLeft = %v; want %v", result["daysLeft"], 365)
	}

	if result["tlsVersion"] != "TLS 1.3" {
		t.Errorf("tlsVersion = %v; want %v", result["tlsVersion"], "TLS 1.3")
	}

	// Verify indentation (default is two spaces)
	lines := strings.Split(string(data), "\n")
	if len(lines) < 2 {
		t.Fatal("expected multiple lines of indented JSON")
	}
	foundIndented := false
	for _, line := range lines {
		if strings.HasPrefix(line, "  ") {
			foundIndented = true
			break
		}
	}
	if !foundIndented {
		t.Error("expected indented JSON output")
	}
}

func TestFormat_EmptyCertificate(t *testing.T) {
	cert := &certificate.Certificate{}

	f := New()
	data, err := f.Format(cert)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Format() returned invalid JSON: %v", err)
	}

	if result["commonName"] != "" {
		t.Errorf("commonName = %v; want empty string", result["commonName"])
	}

	if result["daysLeft"] != float64(0) {
		t.Errorf("daysLeft = %v; want 0", result["daysLeft"])
	}
}

func TestFormat_NilCertificate(t *testing.T) {
	f := New()
	data, err := f.Format(nil)
	if err != nil {
		t.Fatalf("Format(nil) error = %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Format(nil) returned invalid JSON: %v", err)
	}

	// nil pointer should produce a JSON object with zero values
	if result["commonName"] != nil {
		t.Errorf("commonName = %v; want nil", result["commonName"])
	}
}

func TestFormatterInterfaceCompliance(t *testing.T) {
	var _ Formatter = New()
	var _ Formatter = (*JSONFormatter)(nil)
}
