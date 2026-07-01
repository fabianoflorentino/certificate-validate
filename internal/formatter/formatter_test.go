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

func TestFormatTable(t *testing.T) {
	certs := []*certificate.Certificate{
		{Hostname: "github.com", Port: 443, DaysLeft: 180, Issuer: "Sectigo Public Server Authentication CA", TLSVersion: "TLS 1.3"},
		{Hostname: "gitlab.com", Port: 443, DaysLeft: 12, Issuer: "Let's Encrypt Authority X3", TLSVersion: "TLS 1.3"},
		{Hostname: "expired.biz", Port: 443, DaysLeft: 0, Issuer: "Short CA", TLSVersion: "TLS 1.2"},
	}

	data, err := FormatTable(certs)
	if err != nil {
		t.Fatalf("FormatTable() error = %v", err)
	}

	output := string(data)
	if !strings.Contains(output, "github.com") {
		t.Error("output missing github.com")
	}
	if !strings.Contains(output, "gitlab.com") {
		t.Error("output missing gitlab.com")
	}
	if !strings.Contains(output, "critical") {
		t.Error("expected 'critical' status for expired.biz (daysLeft=0)")
	}
	if !strings.Contains(output, "warning") {
		t.Error("expected 'warning' status for gitlab.com (daysLeft=12)")
	}
	if !strings.Contains(output, "good") {
		t.Error("expected 'good' status for github.com (daysLeft=180)")
	}
}

func TestFormatTable_SkipsNil(t *testing.T) {
	certs := []*certificate.Certificate{
		{Hostname: "github.com", Port: 443, DaysLeft: 100},
		nil,
		{Hostname: "gitlab.com", Port: 443, DaysLeft: 50},
	}

	data, err := FormatTable(certs)
	if err != nil {
		t.Fatalf("FormatTable() error = %v", err)
	}

	output := string(data)
	if !strings.Contains(output, "github.com") {
		t.Error("output missing github.com")
	}
	if !strings.Contains(output, "gitlab.com") {
		t.Error("output missing gitlab.com")
	}
}

func TestFormatTable_Empty(t *testing.T) {
	data, err := FormatTable([]*certificate.Certificate{})
	if err != nil {
		t.Fatalf("FormatTable() error = %v", err)
	}

	output := string(data)
	parts := splitLines(output)
	if len(parts) < 2 {
		t.Error("expected at least header and separator lines")
	}
}

func TestFormatTable_StatusThresholds(t *testing.T) {
	tests := []struct {
		days   int
		expect string
	}{
		{0, "critical"},
		{7, "critical"},
		{8, "warning"},
		{30, "warning"},
		{31, "good"},
		{365, "good"},
	}

	for _, tt := range tests {
		certs := []*certificate.Certificate{
			{Hostname: "test.com", Port: 443, DaysLeft: tt.days},
		}
		data, err := FormatTable(certs)
		if err != nil {
			t.Fatalf("days=%d: FormatTable() error = %v", tt.days, err)
		}
		if !strings.Contains(string(data), tt.expect) {
			t.Errorf("days=%d: expected status %q in output", tt.days, tt.expect)
		}
	}
}

func TestFormatTable_IssuerTruncation(t *testing.T) {
	longIssuer := "A really long issuer name that should definitely be truncated to fit in the table because it exceeds forty eight characters total"
	certs := []*certificate.Certificate{
		{Hostname: "test.com", Port: 443, DaysLeft: 100, Issuer: longIssuer},
	}

	data, err := FormatTable(certs)
	if err != nil {
		t.Fatalf("FormatTable() error = %v", err)
	}

	if !strings.Contains(string(data), "...") {
		t.Error("expected issuer to be truncated with ...")
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
