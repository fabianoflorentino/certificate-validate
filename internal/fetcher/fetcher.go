package fetcher

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/fabianoflorentino/certificate-validate/internal/certificate"
	"github.com/fabianoflorentino/certificate-validate/internal/revocation"
)

// Fetcher fetches SSL/TLS certificate information from a host.
type Fetcher interface {
	Fetch(ctx context.Context, hostname string, port int) (*certificate.Certificate, error)
}

type tlsFetcher struct {
	timeout time.Duration
}

// New creates a new TLS-based Fetcher.
func New(timeout time.Duration) Fetcher {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &tlsFetcher{timeout: timeout}
}

func (f *tlsFetcher) Fetch(ctx context.Context, hostname string, port int) (*certificate.Certificate, error) {
	addr := net.JoinHostPort(hostname, strconv.Itoa(port))

	dialer := &net.Dialer{Timeout: f.timeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) {
			return nil, fmt.Errorf("%w: %s", certificate.ErrInvalidHostname, hostname)
		}
		return nil, fmt.Errorf("%w: %s:%d", certificate.ErrHostUnreachable, hostname, port)
	}
	defer conn.Close()

	cs := conn.ConnectionState()
	certs := cs.PeerCertificates
	if len(certs) == 0 {
		return nil, fmt.Errorf("%w: %s:%d", certificate.ErrNoCertificate, hostname, port)
	}

	cert := certificate.FromX509(certs[0], hostname, port)
	cert.TLSVersion = certificate.TLSVersionString(cs.Version)
	cert.CipherSuite = tls.CipherSuiteName(cs.CipherSuite)
	cert.Chain = certificate.BuildChain(certs)

	// Perform best-effort revocation check.
	var issuer *x509.Certificate
	if len(certs) > 1 {
		issuer = certs[1]
	}
	status := revocation.Check(certs[0], issuer, certs[0].OCSPServer, certs[0].CRLDistributionPoints)
	cert.RevocationStatus = status
	revocation.LogRevocation(cert, status)

	return cert, nil
}
