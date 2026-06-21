package revocation

import (
	"bytes"
	"context"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/crypto/ocsp"

	"github.com/fabianoflorentino/certificate-validate/internal/certificate"
)

// CheckOCSP queries the OCSP responder to verify certificate revocation status.
// Returns the OCSP response status or an error if the query fails.
// Returns RevocationNotReady if no OCSP servers are available.
func CheckOCSP(leaf *x509.Certificate, issuer *x509.Certificate, servers []string) certificate.RevocationStatus {
	if len(servers) == 0 || leaf == nil || issuer == nil {
		return certificate.RevocationNotReady
	}

	reqBytes, err := ocsp.CreateRequest(leaf, issuer, nil)
	if err != nil {
		return certificate.RevocationUnknown
	}

	for _, server := range servers {
		status := tryOCSPServer(context.Background(), server, reqBytes, issuer)
		if status != certificate.RevocationUnknown {
			return status
		}
	}

	return certificate.RevocationUnknown
}

func tryOCSPServer(ctx context.Context, server string, reqBytes []byte, issuer *x509.Certificate) certificate.RevocationStatus {
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := httpPost(checkCtx, server, "application/ocsp-request", reqBytes)
	if err != nil {
		return certificate.RevocationUnknown
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return certificate.RevocationUnknown
	}

	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)

	ocspResp, err := ocsp.ParseResponse(buf.Bytes(), issuer)
	if err != nil {
		return certificate.RevocationUnknown
	}

	switch ocspResp.Status {
	case ocsp.Good:
		return certificate.RevocationGood
	case ocsp.Revoked:
		return certificate.RevocationRevoked
	default:
		return certificate.RevocationUnknown
	}
}

// CheckCRL downloads and parses a CRL to verify if a certificate is revoked.
func CheckCRL(leaf *x509.Certificate, crlURLs []string) certificate.RevocationStatus {
	for _, url := range crlURLs {
		status := tryCRL(context.Background(), leaf, url)
		if status != certificate.RevocationUnknown {
			return status
		}
	}
	return certificate.RevocationUnknown
}

func tryCRL(ctx context.Context, leaf *x509.Certificate, url string) certificate.RevocationStatus {
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(checkCtx, http.MethodGet, url, nil)
	if err != nil {
		return certificate.RevocationUnknown
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return certificate.RevocationUnknown
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return certificate.RevocationUnknown
	}

	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)

	crl, err := x509.ParseCRL(buf.Bytes())
	if err != nil {
		return certificate.RevocationUnknown
	}

	revokedCerts := crl.TBSCertList.RevokedCertificates
	for _, rc := range revokedCerts {
		if rc.SerialNumber.Cmp(leaf.SerialNumber) == 0 {
			return certificate.RevocationRevoked
		}
	}

	return certificate.RevocationGood
}

// httpPost is used by CheckOCSP to POST an OCSP request.
var httpPost = func(ctx context.Context, url, contentType string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	return http.DefaultClient.Do(req)
}

// Check performs OCSP and CRL revocation checks on a certificate.
func Check(leaf *x509.Certificate, issuer *x509.Certificate, ocspServers, crlURLs []string) certificate.RevocationStatus {
	if len(ocspServers) > 0 {
		status := CheckOCSP(leaf, issuer, ocspServers)
		if status != certificate.RevocationNotReady {
			return status
		}
	}

	if len(crlURLs) > 0 {
		return CheckCRL(leaf, crlURLs)
	}

	return certificate.RevocationNotReady
}

// LogRevocation logs the revocation status of a certificate.
func LogRevocation(cert *certificate.Certificate, status certificate.RevocationStatus) {
	if status == certificate.RevocationRevoked {
		slog.Warn("certificate is revoked",
			"hostname", cert.Hostname,
			"port", cert.Port,
			"common_name", cert.CommonName,
		)
	}
}
