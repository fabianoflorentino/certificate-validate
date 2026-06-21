package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/fabianoflorentino/certificate-validate/internal/certificate"
	"github.com/fabianoflorentino/certificate-validate/internal/checker"
	"github.com/fabianoflorentino/certificate-validate/internal/config"
	"github.com/fabianoflorentino/certificate-validate/internal/history"
)

// mockChecker implements checker.CertChecker for testing.
type mockChecker struct {
	checkFunc    func(ctx context.Context, hostname string, port int) (*certificate.Certificate, error)
	checkAllFunc func(ctx context.Context, hosts []checker.Host, maxParallel int) ([]*certificate.Certificate, []error)
}

func (m *mockChecker) Check(ctx context.Context, hostname string, port int) (*certificate.Certificate, error) {
	return m.checkFunc(ctx, hostname, port)
}

func (m *mockChecker) CheckAll(ctx context.Context, hosts []checker.Host, maxParallel int) ([]*certificate.Certificate, []error) {
	return m.checkAllFunc(ctx, hosts, maxParallel)
}

// mockStore implements history.Store for testing.
type mockStore struct {
	recordFunc    func(results []*certificate.Certificate)
	getHistoryFunc func(hostname string) ([]history.Entry, error)
}

func (m *mockStore) Record(results []*certificate.Certificate) {
	m.recordFunc(results)
}

func (m *mockStore) GetHistory(hostname string) ([]history.Entry, error) {
	return m.getHistoryFunc(hostname)
}

func TestNewCertService(t *testing.T) {
	svc := NewCertService(nil, nil, nil)
	if svc == nil {
		t.Fatal("NewCertService returned nil")
	}
}

func TestCheckAll_FiltersNils(t *testing.T) {
	cert1 := &certificate.Certificate{Hostname: "example.com", DaysLeft: 100}
	var recorded []*certificate.Certificate

	svc := NewCertService(
		&mockChecker{
			checkAllFunc: func(_ context.Context, hosts []checker.Host, _ int) ([]*certificate.Certificate, []error) {
				return []*certificate.Certificate{cert1, nil}, nil
			},
		},
		&mockStore{
			recordFunc: func(results []*certificate.Certificate) {
				recorded = results
			},
		},
		nil,
	)

	result := svc.CheckAll(context.Background(), []config.HostConfig{
		{Name: "test", URL: "example.com", Port: "443"},
	})

	if len(result.Certificates) != 1 {
		t.Errorf("got %d certificates; want 1", len(result.Certificates))
	}
	if result.Certificates[0].Hostname != "example.com" {
		t.Errorf("got hostname %q; want %q", result.Certificates[0].Hostname, "example.com")
	}
	if len(recorded) != 1 {
		t.Errorf("recorded %d; want 1", len(recorded))
	}
}

func TestCheckAll_ReturnsErrors(t *testing.T) {
	svc := NewCertService(
		&mockChecker{
			checkAllFunc: func(_ context.Context, hosts []checker.Host, _ int) ([]*certificate.Certificate, []error) {
				return nil, []error{errors.New("connection failed")}
			},
		},
		&mockStore{
			recordFunc: func(results []*certificate.Certificate) {},
		},
		nil,
	)

	result := svc.CheckAll(context.Background(), []config.HostConfig{
		{Name: "test", URL: "example.com", Port: "443"},
	})

	if len(result.Errors) != 1 {
		t.Errorf("got %d errors; want 1", len(result.Errors))
	}
	if result.Errors[0] != "connection failed" {
		t.Errorf("got error %q; want %q", result.Errors[0], "connection failed")
	}
}

func TestCheckAll_MetricsUpdater(t *testing.T) {
	var updated []*certificate.Certificate
	mu := sync.Mutex{}

	svc := NewCertService(
		&mockChecker{
			checkAllFunc: func(_ context.Context, hosts []checker.Host, _ int) ([]*certificate.Certificate, []error) {
				return []*certificate.Certificate{
					{Hostname: "a.com", DaysLeft: 50},
				}, nil
			},
		},
		&mockStore{
			recordFunc: func(results []*certificate.Certificate) {},
		},
		func(certs []*certificate.Certificate) {
			mu.Lock()
			updated = certs
			mu.Unlock()
		},
	)

	svc.CheckAll(context.Background(), []config.HostConfig{
		{Name: "a", URL: "a.com", Port: "443"},
	})

	if len(updated) != 1 {
		t.Errorf("metrics updated %d certs; want 1", len(updated))
	}
}

func TestCheckAll_NilMetricsUpdater(t *testing.T) {
	svc := NewCertService(
		&mockChecker{
			checkAllFunc: func(_ context.Context, hosts []checker.Host, _ int) ([]*certificate.Certificate, []error) {
				return []*certificate.Certificate{{Hostname: "a.com"}}, nil
			},
		},
		&mockStore{
			recordFunc: func(results []*certificate.Certificate) {},
		},
		nil, // no metrics updater
	)

	result := svc.CheckAll(context.Background(), []config.HostConfig{
		{Name: "a", URL: "a.com", Port: "443"},
	})

	if len(result.Certificates) != 1 {
		t.Errorf("got %d certificates; want 1", len(result.Certificates))
	}
}

func TestCheckAll_NilRecorder(t *testing.T) {
	svc := NewCertService(
		&mockChecker{
			checkAllFunc: func(_ context.Context, hosts []checker.Host, _ int) ([]*certificate.Certificate, []error) {
				return []*certificate.Certificate{{Hostname: "a.com"}}, nil
			},
		},
		nil, // no recorder
		nil,
	)

	result := svc.CheckAll(context.Background(), []config.HostConfig{
		{Name: "a", URL: "a.com", Port: "443"},
	})

	if len(result.Certificates) != 1 {
		t.Errorf("got %d certificates; want 1", len(result.Certificates))
	}
}

func TestCheckSingle(t *testing.T) {
	want := &certificate.Certificate{Hostname: "example.com", DaysLeft: 90}

	svc := NewCertService(
		&mockChecker{
			checkFunc: func(_ context.Context, hostname string, port int) (*certificate.Certificate, error) {
				if hostname == "example.com" && port == 443 {
					return want, nil
				}
				return nil, errors.New("unexpected call")
			},
		},
		nil, nil,
	)

	got, err := svc.CheckSingle(context.Background(), "example.com", 443)
	if err != nil {
		t.Fatalf("CheckSingle() unexpected error: %v", err)
	}
	if got.Hostname != "example.com" {
		t.Errorf("got hostname %q; want %q", got.Hostname, "example.com")
	}
}

func TestGetHistory_WithRecorder(t *testing.T) {
	entries := []history.Entry{{Host: "example.com", DaysLeft: 50}}

	svc := NewCertService(
		nil,
		&mockStore{
			getHistoryFunc: func(hostname string) ([]history.Entry, error) {
				if hostname == "example.com" {
					return entries, nil
				}
				return nil, nil
			},
		},
		nil,
	)

	got, err := svc.GetHistory("example.com")
	if err != nil {
		t.Fatalf("GetHistory() unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d entries; want 1", len(got))
	}
	if got[0].Host != "example.com" {
		t.Errorf("got host %q; want %q", got[0].Host, "example.com")
	}
}

func TestGetHistory_NoRecorder(t *testing.T) {
	svc := NewCertService(nil, nil, nil)

	got, err := svc.GetHistory("example.com")
	if err != nil {
		t.Fatalf("GetHistory() unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("GetHistory without recorder returned %v; want nil", got)
	}
}
