package fetcher

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
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
	timeout    time.Duration
	rootCAs    *x509.CertPool
	perHostCAs map[string]*x509.CertPool
}

// New creates a new TLS-based Fetcher using the system root certificate pool.
func New(timeout time.Duration) Fetcher {
	return NewWithRootCAs(timeout, nil)
}

// NewWithRootCAs creates a TLS Fetcher using the provided CA certificate pool.
func NewWithRootCAs(timeout time.Duration, rootCAs *x509.CertPool) Fetcher {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &tlsFetcher{timeout: timeout, rootCAs: rootCAs}
}

// NewWithPerHostCAs creates a TLS Fetcher with per-host certificate pools.
// The perHostCAs map key is the host URL (e.g. "internal.example.com").
func NewWithPerHostCAs(timeout time.Duration, rootCAs *x509.CertPool, perHostCAs map[string]*x509.CertPool) Fetcher {
	f := NewWithRootCAs(timeout, rootCAs).(*tlsFetcher)
	f.perHostCAs = perHostCAs
	return f
}

// LoadRootCAs reads one or more PEM-encoded CA certificates and returns a certificate pool.
// When no files are provided, it returns nil to let the system root store be used.
func LoadRootCAs(paths []string) (*x509.CertPool, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	pool := x509.NewCertPool()
	for _, path := range paths {
		pemData, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read root CA %s: %w", path, err)
		}
		if ok := pool.AppendCertsFromPEM(pemData); !ok {
			return nil, fmt.Errorf("failed to parse root CA PEM %s", path)
		}
	}

	return pool, nil
}

func (f *tlsFetcher) Fetch(ctx context.Context, hostname string, port int) (*certificate.Certificate, error) {
	addr := net.JoinHostPort(hostname, strconv.Itoa(port))

	rootCAs := f.rootCAs
	if pool, ok := f.perHostCAs[hostname]; ok {
		rootCAs = pool
	}

	dialer := &net.Dialer{Timeout: f.timeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
		RootCAs:    rootCAs,
		ServerName: hostname,
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
	defer func() {
		_ = conn.Close()
	}()

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
