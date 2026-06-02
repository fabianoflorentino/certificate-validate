package checker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/fabianoflorentino/certificate-validate/internal/certificate"
)

// Host is a certificate check target.
type Host struct {
	Hostname string
	Port     int
	Name     string
}

// Fetcher fetches certificate info from a host.
type Fetcher interface {
	Fetch(ctx context.Context, hostname string, port int) (*certificate.Certificate, error)
}

// Formatter formats certificate data for output.
type Formatter interface {
	Format(cert *certificate.Certificate) ([]byte, error)
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

// Check fetches certificate info for a single host.
func (c *Checker) Check(ctx context.Context, hostname string, port int) (*certificate.Certificate, error) {
	return c.fetcher.Fetch(ctx, hostname, port)
}

// Format formats a certificate as bytes.
func (c *Checker) Format(cert *certificate.Certificate) ([]byte, error) {
	return c.formatter.Format(cert)
}

// CheckAll fetches and formats certificates for multiple hosts concurrently.
func (c *Checker) CheckAll(ctx context.Context, hosts []Host, maxParallel int) ([][]byte, []error) {
	if maxParallel <= 0 {
		maxParallel = 10
	}

	type res struct {
		data []byte
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
			cert, err := c.fetcher.Fetch(ctx, h.Hostname, h.Port)
			if err != nil {
				log.Printf("ERROR: %s:%d - %v", h.Hostname, h.Port, err)
				results <- res{nil, err, i}
				return
			}
			data, err := c.formatter.Format(cert)
			results <- res{data, err, i}
		}()
	}

	for i := 0; i < cap(sem); i++ {
		sem <- struct{}{}
	}
	close(results)

	out := make([][]byte, len(hosts))
	var errs []error
	for r := range results {
		if r.err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", hosts[r.idx].Hostname, r.err))
			continue
		}
		out[r.idx] = r.data
	}

	return out, errs
}

// RunWatchLoop checks all hosts periodically until the context is cancelled.
func (c *Checker) RunWatchLoop(ctx context.Context, hosts []Host, checkTime time.Duration) {
	for {
		select {
		case <-ctx.Done():
			log.Println("Watch loop stopped.")
			return
		default:
			results, _ := c.CheckAll(ctx, hosts, 0)
			for _, data := range results {
				if data != nil {
					fmt.Println(string(data))
				}
			}
			log.Printf("Waiting %s before next check...", checkTime)
			time.Sleep(checkTime)
		}
	}
}
