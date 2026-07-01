package checker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/fabianoflorentino/certificate-validate/internal/certificate"
)

// Host is a certificate check target.
type Host struct {
	Hostname   string
	Port       int
	Name       string
	Timeout    time.Duration // per-host dial timeout (0 = use default)
}

// Fetcher fetches certificate info from a host.
type Fetcher interface {
	Fetch(ctx context.Context, hostname string, port int) (*certificate.Certificate, error)
}

// Formatter formats certificate data for output.
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
type Checker struct {
	fetcher   Fetcher
	formatter Formatter
}

// New creates a new Checker with the given dependencies.
func New(fetcher Fetcher, formatter Formatter) *Checker {
	return &Checker{
		fetcher:   fetcher,
		formatter: formatter,
	}
}

// Compile-time check: *Checker implements CertChecker.
var _ CertChecker = (*Checker)(nil)

// Check fetches certificate info for a single host.
func (c *Checker) Check(ctx context.Context, hostname string, port int) (*certificate.Certificate, error) {
	return c.fetcher.Fetch(ctx, hostname, port)
}

// Format formats a certificate as bytes.
func (c *Checker) Format(cert *certificate.Certificate) ([]byte, error) {
	return c.formatter.Format(cert)
}

// CheckAll fetches certificates for multiple hosts concurrently.
func (c *Checker) CheckAll(ctx context.Context, hosts []Host, maxParallel int) ([]*certificate.Certificate, []error) {
	if maxParallel <= 0 {
		maxParallel = 10
	}

	type res struct {
		cert *certificate.Certificate
		err  error
		idx  int
	}

	sem := make(chan struct{}, maxParallel)
	results := make(chan res, len(hosts))

	for i, h := range hosts {
		sem <- struct{}{}
		i, h := i, h
		go func() {
			defer func() { <-sem }()
			checkCtx := ctx
			if h.Timeout > 0 {
				var cancel context.CancelFunc
				checkCtx, cancel = context.WithTimeout(ctx, h.Timeout)
				defer cancel()
			}
			cert, err := c.fetcher.Fetch(checkCtx, h.Hostname, h.Port)
			if err != nil {
				slog.Error("check failed", "host", h.Hostname, "port", h.Port, "error", err)
				results <- res{nil, err, i}
				return
			}
			results <- res{cert, nil, i}
		}()
	}

	for i := 0; i < cap(sem); i++ {
		sem <- struct{}{}
	}
	close(results)

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


