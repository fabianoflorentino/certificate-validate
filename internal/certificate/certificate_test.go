package certificate

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/json"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"
)

func generateTestCert(t *testing.T, template *x509.Certificate) *x509.Certificate {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}

	return cert
}

func generateCACert(t *testing.T, subject pkix.Name) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate CA key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      subject,
		NotBefore:    time.Now().Add(-24 * time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:         true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("failed to create CA certificate: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("failed to parse CA certificate: %v", err)
	}

	return cert, priv
}

func generateLeafCert(t *testing.T, caCert *x509.Certificate, caKey *ecdsa.PrivateKey, template *x509.Certificate) *x509.Certificate {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate leaf key: %v", err)
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, &priv.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed to create leaf certificate: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("failed to parse leaf certificate: %v", err)
	}

	return cert
}

type policyInformation struct {
	PolicyIdentifier asn1.ObjectIdentifier
}

func TestFromX509(t *testing.T) {
	caCert, caKey := generateCACert(t, pkix.Name{CommonName: "Test CA"})

	notBefore := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	notAfter := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	policyOID := asn1.ObjectIdentifier{2, 23, 140, 1, 2, 1}
	policySeq, err := asn1.Marshal([]policyInformation{{PolicyIdentifier: policyOID}})
	if err != nil {
		t.Fatalf("failed to marshal policy extension: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName: "test.example.com",
		},
		DNSNames:              []string{"test.example.com", "www.test.example.com"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		PolicyIdentifiers:     []asn1.ObjectIdentifier{policyOID},
		CRLDistributionPoints: []string{"http://crl.example.com/ca.crl"},
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		ExtraExtensions: []pkix.Extension{
			{
				Id:       asn1.ObjectIdentifier{2, 5, 29, 32},
				Critical: false,
				Value:    policySeq,
			},
		},
	}

	cert := generateLeafCert(t, caCert, caKey, template)
	result := FromX509(cert, "test.example.com", 443)

	if result.CommonName != "test.example.com" {
		t.Errorf("CommonName = %q; want %q", result.CommonName, "test.example.com")
	}

	wantSANs := []string{"test.example.com", "www.test.example.com", "127.0.0.1", "::1"}
	if len(result.SubjectAltNames) != len(wantSANs) {
		t.Errorf("SubjectAltNames length = %d; want %d", len(result.SubjectAltNames), len(wantSANs))
	}
	for i, want := range wantSANs {
		if result.SubjectAltNames[i] != want {
			t.Errorf("SubjectAltNames[%d] = %q; want %q", i, result.SubjectAltNames[i], want)
		}
	}

	if result.Issuer != "Test CA" {
		t.Errorf("Issuer = %q; want %q", result.Issuer, "Test CA")
	}

	if result.Type != "Domain Validation (DV) Web Server SSL Digital Certificate" {
		t.Errorf("Type = %q; want %q", result.Type, "Domain Validation (DV) Web Server SSL Digital Certificate")
	}

	if result.NotBefore != "2024-01-01 00:00:00" {
		t.Errorf("NotBefore = %q; want %q", result.NotBefore, "2024-01-01 00:00:00")
	}

	if result.NotAfter != "2025-01-01 00:00:00" {
		t.Errorf("NotAfter = %q; want %q", result.NotAfter, "2025-01-01 00:00:00")
	}

	if result.Hostname != "test.example.com" {
		t.Errorf("Hostname = %q; want %q", result.Hostname, "test.example.com")
	}

	if result.Port != 443 {
		t.Errorf("Port = %d; want %d", result.Port, 443)
	}

	if len(result.CRLDistributionPoints) != 1 || result.CRLDistributionPoints[0] != "http://crl.example.com/ca.crl" {
		t.Errorf("CRLDistributionPoints = %v; want [%q]", result.CRLDistributionPoints, "http://crl.example.com/ca.crl")
	}
}

func TestFromX509_EmptyCert(t *testing.T) {
	notBefore := time.Now().Add(-24 * time.Hour)
	notAfter := time.Now().Add(24 * time.Hour)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	cert := generateTestCert(t, template)
	result := FromX509(cert, "localhost", 8443)

	if result.CommonName != "" {
		t.Errorf("CommonName = %q; want empty", result.CommonName)
	}

	if len(result.SubjectAltNames) != 0 {
		t.Errorf("SubjectAltNames = %v; want empty", result.SubjectAltNames)
	}

	if result.Type != "Type not found for this certificate!" {
		t.Errorf("Type = %q; want %q", result.Type, "Type not found for this certificate!")
	}

	if result.Hostname != "localhost" {
		t.Errorf("Hostname = %q; want %q", result.Hostname, "localhost")
	}

	if result.Port != 8443 {
		t.Errorf("Port = %d; want %d", result.Port, 8443)
	}
}

func TestBuildChain(t *testing.T) {
	notAfter := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	rootCA2, rootKey2 := generateCACert(t, pkix.Name{CommonName: "Root CA"})

	interPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate intermediate key: %v", err)
	}
	interTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(5),
		Subject:      pkix.Name{CommonName: "Intermediate CA", Organization: []string{"Test Org"}},
		NotBefore:    time.Now().Add(-24 * time.Hour),
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:         true,
	}
	interDER, err := x509.CreateCertificate(rand.Reader, interTmpl, rootCA2, &interPriv.PublicKey, rootKey2)
	if err != nil {
		t.Fatalf("failed to create intermediate: %v", err)
	}
	interCert2, err := x509.ParseCertificate(interDER)
	if err != nil {
		t.Fatalf("failed to parse intermediate: %v", err)
	}

	leafPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate leaf key: %v", err)
	}
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(6),
		Subject:      pkix.Name{CommonName: "Leaf Cert"},
		NotBefore:    time.Now().Add(-24 * time.Hour),
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, interCert2, &leafPriv.PublicKey, interPriv)
	if err != nil {
		t.Fatalf("failed to create leaf: %v", err)
	}
	leafCert, err := x509.ParseCertificate(leafDER)
	if err != nil {
		t.Fatalf("failed to parse leaf: %v", err)
	}

	peerCerts := []*x509.Certificate{leafCert, interCert2}
	chain := BuildChain(peerCerts)

	if len(chain) != 2 {
		t.Fatalf("chain length = %d; want 2", len(chain))
	}

	if !strings.Contains(chain[0].Subject, "Leaf Cert") {
		t.Errorf("chain[0].Subject = %q; want to contain %q", chain[0].Subject, "Leaf Cert")
	}

	if !strings.Contains(chain[0].Issuer, "Intermediate CA") {
		t.Errorf("chain[0].Issuer = %q; want to contain %q", chain[0].Issuer, "Intermediate CA")
	}

	if !strings.Contains(chain[1].Subject, "Intermediate CA") {
		t.Errorf("chain[1].Subject = %q; want to contain %q", chain[1].Subject, "Intermediate CA")
	}

	if !strings.Contains(chain[1].Issuer, "Root CA") {
		t.Errorf("chain[1].Issuer = %q; want to contain %q", chain[1].Issuer, "Root CA")
	}

	if chain[0].NotAfter != "2025-06-01 00:00:00" {
		t.Errorf("chain[0].NotAfter = %q; want %q", chain[0].NotAfter, "2025-06-01 00:00:00")
	}

	if chain[0].Fingerprint == "" {
		t.Error("chain[0].Fingerprint is empty")
	}

	if len(chain[0].Fingerprint) != 64 {
		t.Errorf("chain[0].Fingerprint length = %d; want 64", len(chain[0].Fingerprint))
	}
}

func TestBuildChain_Empty(t *testing.T) {
	chain := BuildChain([]*x509.Certificate{})
	if len(chain) != 0 {
		t.Errorf("chain length = %d; want 0", len(chain))
	}
}

func TestTLSVersionString(t *testing.T) {
	tests := []struct {
		name    string
		version uint16
		want    string
	}{
		{"TLS 1.3", tls.VersionTLS13, "TLS 1.3"},
		{"TLS 1.2", tls.VersionTLS12, "TLS 1.2"},
		{"TLS 1.1", tls.VersionTLS11, "TLS 1.1"},
		{"TLS 1.0", tls.VersionTLS10, "TLS 1.0"},
		{"unknown 0x0300", 0x0300, "TLS 0x0300"},
		{"unknown 0x0000", 0x0000, "TLS 0x0000"},
		{"unknown 0xFFFF", 0xFFFF, "TLS 0xFFFF"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TLSVersionString(tt.version)
			if got != tt.want {
				t.Errorf("TLSVersionString(0x%04X) = %q; want %q", tt.version, got, tt.want)
			}
		})
	}
}

func TestHasExpired(t *testing.T) {
	t.Run("expired", func(t *testing.T) {
		template := &x509.Certificate{
			SerialNumber: big.NewInt(7),
			Subject:      pkix.Name{CommonName: "expired"},
			NotBefore:    time.Now().Add(-48 * time.Hour),
			NotAfter:     time.Now().Add(-24 * time.Hour),
			KeyUsage:     x509.KeyUsageDigitalSignature,
		}
		cert := generateTestCert(t, template)
		if !HasExpired(cert) {
			t.Error("HasExpired = false; want true for expired cert")
		}
	})

	t.Run("not expired", func(t *testing.T) {
		template := &x509.Certificate{
			SerialNumber: big.NewInt(8),
			Subject:      pkix.Name{CommonName: "valid"},
			NotBefore:    time.Now(),
			NotAfter:     time.Now().Add(24 * time.Hour),
			KeyUsage:     x509.KeyUsageDigitalSignature,
		}
		cert := generateTestCert(t, template)
		if HasExpired(cert) {
			t.Error("HasExpired = true; want false for valid cert")
		}
	})
}

func TestDaysUntil(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		t    time.Time
		want int
	}{
		{
			name: "future 3 days",
			t:    now.Add(73 * time.Hour),
			want: 3,
		},
		{
			name: "past date",
			t:    now.Add(-24 * time.Hour),
			want: 0,
		},
		{
			name: "exact now",
			t:    now,
			want: 0,
		},
		{
			name: "one day",
			t:    now.Add(25 * time.Hour),
			want: 1,
		},
		{
			name: "zero seconds future",
			t:    now.Add(1 * time.Second),
			want: 0,
		},
		{
			name: "47 hours",
			t:    now.Add(47 * time.Hour),
			want: 1,
		},
		{
			name: "49 hours",
			t:    now.Add(49 * time.Hour),
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := daysUntil(tt.t)
			if got != tt.want {
				t.Errorf("daysUntil(%v) = %d; want %d", tt.t, got, tt.want)
			}
		})
	}
}

func TestResolveCertType(t *testing.T) {
	tests := []struct {
		name string
		oids []asn1.ObjectIdentifier
		want string
	}{
		{
			name: "DV OID",
			oids: []asn1.ObjectIdentifier{{2, 23, 140, 1, 2, 1}},
			want: "Domain Validation (DV) Web Server SSL Digital Certificate",
		},
		{
			name: "EV OID first",
			oids: []asn1.ObjectIdentifier{{2, 23, 140, 1, 1}},
			want: "Extended Validation (EV) Web Server SSL Digital Certificate",
		},
		{
			name: "EV OID second",
			oids: []asn1.ObjectIdentifier{{2, 16, 840, 1, 114404, 1, 1, 2, 4, 1}},
			want: "Extended Validation (EV) Web Server SSL Digital Certificate",
		},
		{
			name: "OV OID first",
			oids: []asn1.ObjectIdentifier{{2, 23, 140, 1, 2, 2}},
			want: "Organization Validation (OV) Web Server SSL Digital Certificate",
		},
		{
			name: "OV OID second",
			oids: []asn1.ObjectIdentifier{{2, 23, 140, 1, 2, 3}},
			want: "Organization Validation (OV) Web Server SSL Digital Certificate",
		},
		{
			name: "OV Code Signing",
			oids: []asn1.ObjectIdentifier{{2, 23, 140, 1, 4, 1}},
			want: "Organization Validation (OV) Code Signing Certificate",
		},
		{
			name: "unknown OID",
			oids: []asn1.ObjectIdentifier{{1, 2, 3, 4, 5}},
			want: "Type not found for this certificate!",
		},
		{
			name: "empty OIDs",
			oids: []asn1.ObjectIdentifier{},
			want: "Type not found for this certificate!",
		},
		{
			name: "mixed known and unknown",
			oids: []asn1.ObjectIdentifier{{1, 2, 3}, {2, 23, 140, 1, 2, 1}},
			want: "Domain Validation (DV) Web Server SSL Digital Certificate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveCertType(tt.oids)
			if got != tt.want {
				t.Errorf("resolveCertType(%v) = %q; want %q", tt.oids, got, tt.want)
			}
		})
	}
}

func TestIPStrings(t *testing.T) {
	tests := []struct {
		name string
		ips  []net.IP
		want []string
	}{
		{
			name: "IPv4 and IPv6",
			ips:  []net.IP{net.ParseIP("192.168.1.1"), net.ParseIP("10.0.0.1")},
			want: []string{"192.168.1.1", "10.0.0.1"},
		},
		{
			name: "empty",
			ips:  []net.IP{},
			want: []string{},
		},
		{
			name: "nil",
			ips:  nil,
			want: []string{},
		},
		{
			name: "IPv6",
			ips:  []net.IP{net.ParseIP("2001:db8::1")},
			want: []string{"2001:db8::1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ipStrings(tt.ips)
			if len(got) != len(tt.want) {
				t.Errorf("ipStrings() length = %d; want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ipStrings()[%d] = %q; want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFingerprintSHA256(t *testing.T) {
	template := &x509.Certificate{
		SerialNumber: big.NewInt(9),
		Subject:      pkix.Name{CommonName: "fingerprint-test"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	cert := generateTestCert(t, template)

	fp := fingerprintSHA256(cert)
	if fp == "" {
		t.Fatal("fingerprintSHA256 returned empty string")
	}

	if len(fp) != 64 {
		t.Errorf("fingerprint length = %d; want 64", len(fp))
	}

	// Verify it's a valid hex string
	for _, r := range fp {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			t.Errorf("fingerprint contains invalid character: %q", r)
		}
	}

	// Verify consistency
	fp2 := fingerprintSHA256(cert)
	if fp != fp2 {
		t.Error("fingerprintSHA256 is not deterministic")
	}
}

func TestCertificate_JSONSerialization(t *testing.T) {
	cert := &Certificate{
		CommonName:      "example.com",
		SubjectAltNames: []string{"example.com", "www.example.com"},
		Issuer:          "Test CA",
		Type:            "DV",
		NotBefore:       "2024-01-01 00:00:00",
		NotAfter:        "2025-01-01 00:00:00",
		DaysLeft:        365,
		Hostname:        "example.com",
		Port:            443,
		TLSVersion:      "TLS 1.3",
		CipherSuite:     "TLS_AES_128_GCM_SHA256",
		Chain: []ChainEntry{
			{
				Subject:     "CN=example.com",
				Issuer:      "CN=Test CA",
				NotAfter:    "2025-01-01 00:00:00",
				Fingerprint: "abcd1234",
			},
		},
	}

	data, err := json.Marshal(cert)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if result["commonName"] != "example.com" {
		t.Errorf("commonName = %v; want %v", result["commonName"], "example.com")
	}
}
