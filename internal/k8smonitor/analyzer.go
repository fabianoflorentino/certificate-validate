package k8smonitor

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"github.com/fabianoflorentino/certificate-validate/internal/certificate"
	"github.com/fabianoflorentino/certificate-validate/internal/revocation"
)

// Analyzer parses and validates Kubernetes TLS certificates.
type Analyzer struct {
	checkRevocation bool
}

// NewAnalyzer creates an Analyzer. When checkRevocation is true, OCSP/CRL
// checks are performed using the configured short timeout.
func NewAnalyzer(checkRevocation bool) *Analyzer {
	return &Analyzer{checkRevocation: checkRevocation}
}

// ParseBundle parses the tls.crt PEM bytes and returns the leaf certificate
// and its chain.
func (a *Analyzer) ParseBundle(pemBytes []byte) (leaf *x509.Certificate, chain []*x509.Certificate, err error) {
	certs, err := parseCertificates(pemBytes)
	if err != nil {
		return nil, nil, err
	}
	if len(certs) == 0 {
		return nil, nil, fmt.Errorf("no certificates found in bundle")
	}
	leaf = certs[0]
	chain = certs[1:]
	return leaf, chain, nil
}

// Analyze builds a K8sCertificate from a leaf certificate and Kubernetes metadata.
func (a *Analyzer) Analyze(leaf *x509.Certificate, chain []*x509.Certificate, ns, name string, kind Kind, labels, annotations map[string]string) *K8sCertificate {
	cert := certificate.FromX509(leaf, name, 443)
	// Override hostname to the resource name for clarity in metrics.
	cert.Hostname = name
	cert.Chain = certificate.BuildChain(append([]*x509.Certificate{leaf}, chain...))

	k8sCert := &K8sCertificate{
		Certificate:    *cert,
		K8sNamespace:   ns,
		K8sName:        name,
		K8sKind:        kind,
		K8sAnnotations: annotations,
		K8sLabels:      labels,
		RenewalState:   RenewalStateNone,
		SerialNumber:   leaf.SerialNumber.String(),
	}

	if a.checkRevocation {
		status := revocation.Check(leaf, findIssuer(chain), leaf.OCSPServer, leaf.CRLDistributionPoints)
		k8sCert.RevocationStatus = status
	}

	return k8sCert
}

func findIssuer(chain []*x509.Certificate) *x509.Certificate {
	if len(chain) > 0 {
		return chain[0]
	}
	return nil
}

func parseCertificates(pemBytes []byte) ([]*x509.Certificate, error) {
	var certs []*x509.Certificate
	rest := pemBytes
	for {
		block, remaining := pem.Decode(rest)
		rest = remaining
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		c, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse certificate: %w", err)
		}
		certs = append(certs, c)
	}
	return certs, nil
}
