package certificate

import (
	"crypto/x509"
	"encoding/asn1"
	"net"
	"time"
)

// Certificate represents the extracted information from an SSL/TLS certificate.
type Certificate struct {
	CommonName      string   `json:"commonName"`
	SubjectAltNames []string `json:"subjectAltName"`
	Issuer          string   `json:"issuer"`
	Type            string   `json:"type"`
	NotBefore       string   `json:"notBefore"`
	NotAfter        string   `json:"notAfter"`
	DaysLeft        int      `json:"daysLeft"`
	CRLDistributionPoints []string `json:"crl"`
	Hostname        string   `json:"hostname"`
	Port            int      `json:"port"`
}

var oidCertTypes = map[string]string{
	"2.23.140.1.1":                        "Extended Validation (EV) Web Server SSL Digital Certificate",
	"2.16.840.1.114404.1.1.2.4.1":        "Extended Validation (EV) Web Server SSL Digital Certificate",
	"2.23.140.1.2.1":                      "Domain Validation (DV) Web Server SSL Digital Certificate",
	"2.23.140.1.2.2":                      "Organization Validation (OV) Web Server SSL Digital Certificate",
	"2.23.140.1.2.3":                      "Organization Validation (OV) Web Server SSL Digital Certificate",
	"2.23.140.1.4.1":                      "Organization Validation (OV) Code Signing Certificate",
}

// FromX509 builds a Certificate from a crypto/x509 certificate and connection info.
func FromX509(cert *x509.Certificate, hostname string, port int) *Certificate {
	return &Certificate{
		CommonName:      cert.Subject.CommonName,
		SubjectAltNames: append(cert.DNSNames, ipStrings(cert.IPAddresses)...),
		Issuer:          cert.Issuer.CommonName,
		Type:            resolveCertType(cert.PolicyIdentifiers),
		NotBefore:       cert.NotBefore.Format("2006-01-02 15:04:05"),
		NotAfter:        cert.NotAfter.Format("2006-01-02 15:04:05"),
		DaysLeft:        daysUntil(cert.NotAfter),
		CRLDistributionPoints: cert.CRLDistributionPoints,
		Hostname:        hostname,
		Port:            port,
	}
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
