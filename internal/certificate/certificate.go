package certificate

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/asn1"
	"fmt"
	"net"
	"time"
)

// ChainEntry represents a single certificate in the chain.
type ChainEntry struct {
	Subject     string `json:"subject"`
	Issuer      string `json:"issuer"`
	NotAfter    string `json:"notAfter"`
	Fingerprint string `json:"fingerprint"`
}

// Certificate represents the extracted information from an SSL/TLS certificate.
type Certificate struct {
	CommonName            string       `json:"commonName"`
	SubjectAltNames       []string     `json:"subjectAltName"`
	Issuer                string       `json:"issuer"`
	Type                  string       `json:"type"`
	NotBefore             string       `json:"notBefore"`
	NotAfter              string       `json:"notAfter"`
	DaysLeft              int          `json:"daysLeft"`
	CRLDistributionPoints []string     `json:"crl"`
	Hostname              string       `json:"hostname"`
	Port                  int          `json:"port"`
	TLSVersion            string       `json:"tlsVersion"`
	CipherSuite           string       `json:"cipherSuite"`
	Chain                 []ChainEntry `json:"chain"`
}

var oidCertTypes = map[string]string{
	"2.23.140.1.1":                "Extended Validation (EV) Web Server SSL Digital Certificate",
	"2.16.840.1.114404.1.1.2.4.1": "Extended Validation (EV) Web Server SSL Digital Certificate",
	"2.23.140.1.2.1":              "Domain Validation (DV) Web Server SSL Digital Certificate",
	"2.23.140.1.2.2":              "Organization Validation (OV) Web Server SSL Digital Certificate",
	"2.23.140.1.2.3":              "Organization Validation (OV) Web Server SSL Digital Certificate",
	"2.23.140.1.4.1":              "Organization Validation (OV) Code Signing Certificate",
}

// FromX509 builds a Certificate from a crypto/x509 certificate and connection info.
func FromX509(cert *x509.Certificate, hostname string, port int) *Certificate {
	return &Certificate{
		CommonName:            cert.Subject.CommonName,
		SubjectAltNames:       append(cert.DNSNames, ipStrings(cert.IPAddresses)...),
		Issuer:                cert.Issuer.CommonName,
		Type:                  resolveCertType(cert.PolicyIdentifiers),
		NotBefore:             cert.NotBefore.Format("2006-01-02 15:04:05"),
		NotAfter:              cert.NotAfter.Format("2006-01-02 15:04:05"),
		DaysLeft:              daysUntil(cert.NotAfter),
		CRLDistributionPoints: cert.CRLDistributionPoints,
		Hostname:              hostname,
		Port:                  port,
	}
}

// BuildChain builds a ChainEntry slice from peer certificates.
func BuildChain(peerCerts []*x509.Certificate) []ChainEntry {
	chain := make([]ChainEntry, 0, len(peerCerts))
	for _, c := range peerCerts {
		chain = append(chain, ChainEntry{
			Subject:     c.Subject.String(),
			Issuer:      c.Issuer.String(),
			NotAfter:    c.NotAfter.Format("2006-01-02 15:04:05"),
			Fingerprint: fingerprintSHA256(c),
		})
	}
	return chain
}

// TLSVersionString converts a tls version constant to a readable string.
func TLSVersionString(version uint16) string {
	switch version {
	case tls.VersionTLS13:
		return "TLS 1.3"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS10:
		return "TLS 1.0"
	default:
		return fmt.Sprintf("TLS 0x%04X", version)
	}
}

func fingerprintSHA256(cert *x509.Certificate) string {
	h := sha256.Sum256(cert.Raw)
	return fmt.Sprintf("%x", h)
}

// HasExpired checks whether the certificate has already expired.
func HasExpired(cert *x509.Certificate) bool {
	return time.Now().After(cert.NotAfter)
}

func daysUntil(t time.Time) int {
	remaining := time.Until(t)
	if remaining < 0 {
		return 0
	}
	return int(remaining.Hours() / 24)
}

func resolveCertType(policyOIDs []asn1.ObjectIdentifier) string {
	for _, oid := range policyOIDs {
		oidStr := oid.String()
		if desc, ok := oidCertTypes[oidStr]; ok {
			return desc
		}
	}
	return "Type not found for this certificate!"
}

func ipStrings(ips []net.IP) []string {
	s := make([]string, 0, len(ips))
	for _, ip := range ips {
		s = append(s, ip.String())
	}
	return s
}
