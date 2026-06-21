package service

import (
	"context"

	"github.com/fabianoflorentino/certificate-validate/internal/certificate"
	"github.com/fabianoflorentino/certificate-validate/internal/checker"
	"github.com/fabianoflorentino/certificate-validate/internal/config"
	"github.com/fabianoflorentino/certificate-validate/internal/history"
)

// MetricsUpdater is a function type for updating metrics from certificate results.
type MetricsUpdater func([]*certificate.Certificate)

// CheckResult holds the certificates fetched and any errors encountered.
type CheckResult struct {
	Certificates []*certificate.Certificate
	Errors       []string
}

// CertService orchestrates certificate checking, history recording, and metrics.
type CertService struct {
	checker  checker.CertChecker
	recorder history.Store
	metrics  MetricsUpdater
}

// NewCertService creates a CertService with the given dependencies.
// The metrics updater is optional (can be nil).
func NewCertService(c checker.CertChecker, recorder history.Store, metrics MetricsUpdater) *CertService {
	return &CertService{
		checker:  c,
		recorder: recorder,
		metrics:  metrics,
	}
}

// CheckAll fetches all certificates from configured hosts, filters nils,
// records history if enabled, and updates Prometheus metrics if enabled.
func (s *CertService) CheckAll(ctx context.Context, cfgHosts []config.HostConfig) CheckResult {
	hosts := config.ToCheckerHosts(cfgHosts)
	certs, errs := s.checker.CheckAll(ctx, hosts, 10)

	filtered := make([]*certificate.Certificate, 0, len(certs))
	for _, c := range certs {
		if c != nil {
			filtered = append(filtered, c)
		}
	}

	errMessages := make([]string, 0, len(errs))
	for _, err := range errs {
		errMessages = append(errMessages, err.Error())
	}

	if s.metrics != nil {
		s.metrics(filtered)
	}

	if s.recorder != nil {
		s.recorder.Record(filtered)
	}

	return CheckResult{
		Certificates: filtered,
		Errors:       errMessages,
	}
}

// CheckSingle fetches a single certificate by hostname.
func (s *CertService) CheckSingle(ctx context.Context, hostname string, port int) (*certificate.Certificate, error) {
	return s.checker.Check(ctx, hostname, port)
}

// GetHistory returns history entries for a hostname.
func (s *CertService) GetHistory(hostname string) ([]history.Entry, error) {
	if s.recorder == nil {
		return nil, nil
	}
	return s.recorder.GetHistory(hostname)
}
