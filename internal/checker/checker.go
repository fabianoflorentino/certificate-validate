package checker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/fabianoflorentino/certificate-validate/internal/certificate"
)

// Host is a certificate check target.
type Host struct {
	Hostname string
	Port     int
	Name     string
	Timeout  time.Duration // per-host dial timeout (0 = use default)
}

// Fetcher fetches certificate info from a host.
// Implementations should handle TLS handshake, certificate extraction,
// and revocation checking.
type Fetcher interface {
	Fetch(ctx context.Context, hostname string, port int) (*certificate.Certificate, error)
}

// Formatter formats certificate data for output.
// Implementations include JSON, table, and CSV formatters.
type Formatter interface {
	Format(cert *certificate.Certificate) ([]byte, error)
}

// CertChecker is the interface for checking certificate expiration.
// Consumers (api, notifier, metrics) depend on this, not on the concrete Checker.
type CertChecker interface {
	Check(ctx context.Context, hostname string, port int) (*certificate.Certificate, error)
	CheckAll(ctx context.Context, hosts []Host, maxParallel int) ([]*certificate.Certificate, []error)
}

// Checker orchestrates fetching and formatting certificates.
// It implements CertChecker and Formatter interfaces.
type Checker struct {
	fetcher   Fetcher
	formatter Formatter
}

// New creates a new Checker with the given dependencies.
// The fetcher retrieves certificates, the formatter formats output.
func New(fetcher Fetcher, formatter Formatter) *Checker {
	return &Checker{
		fetcher:   fetcher,
		formatter: formatter,
	}
}

// Compile-time check: *Checker implements CertChecker.
var _ CertChecker = (*Checker)(nil)

// Check fetches certificate info for a single host.
// Returns the certificate or an error if the fetch fails.
func (c *Checker) Check(ctx context.Context, hostname string, port int) (*certificate.Certificate, error) {
	return c.fetcher.Fetch(ctx, hostname, port)
}

// Format formats a certificate as bytes using the configured formatter.
func (c *Checker) Format(cert *certificate.Certificate) ([]byte, error) {
	return c.formatter.Format(cert)
}

type checkResult struct {
	cert *certificate.Certificate
	err  error
	idx  int
}

// CheckAll fetches certificates for multiple hosts concurrently.
// maxParallel controls the maximum number of concurrent fetches (0 = 10).
// Returns a slice of certificates (nil for failed hosts) and a slice of errors.
func (c *Checker) CheckAll(ctx context.Context, hosts []Host, maxParallel int) ([]*certificate.Certificate, []error) {
	return assembleResults(c.fetchAll(ctx, hosts, maxParallel), hosts)
}

func (c *Checker) fetchAll(ctx context.Context, hosts []Host, maxParallel int) <-chan checkResult {
	if maxParallel <= 0 {
		maxParallel = 10
	}

	sem := make(chan struct{}, maxParallel)
	results := make(chan checkResult, len(hosts))

	var wg sync.WaitGroup
	for i, h := range hosts {
		sem <- struct{}{}
		wg.Add(1)
		i, h := i, h

		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			checkCtx := ctx
			if h.Timeout > 0 {
				var cancel context.CancelFunc
				checkCtx, cancel = context.WithTimeout(ctx, h.Timeout)
				defer cancel()
			}

			cert, err := c.fetcher.Fetch(checkCtx, h.Hostname, h.Port)
			results <- checkResult{cert, err, i}
		}()
	}

	wg.Wait()
	close(results)
	return results
}

func assembleResults(results <-chan checkResult, hosts []Host) ([]*certificate.Certificate, []error) {
	out := make([]*certificate.Certificate, len(hosts))

	var errs []error
	for r := range results {
		if r.err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", hosts[r.idx].Hostname, r.err))
			continue
		}
		out[r.idx] = r.cert
	}

	return out, errs
}
