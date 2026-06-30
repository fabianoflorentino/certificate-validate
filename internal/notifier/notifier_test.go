package notifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fabianoflorentino/certificate-validate/internal/certificate"
	"github.com/fabianoflorentino/certificate-validate/internal/checker"
)

type mockCertChecker struct {
	checkFunc    func(ctx context.Context, hostname string, port int) (*certificate.Certificate, error)
	checkAllFunc func(ctx context.Context, hosts []checker.Host, maxParallel int) ([]*certificate.Certificate, []error)
}

func (m *mockCertChecker) Check(ctx context.Context, hostname string, port int) (*certificate.Certificate, error) {
	return m.checkFunc(ctx, hostname, port)
}

func (m *mockCertChecker) CheckAll(ctx context.Context, hosts []checker.Host, maxParallel int) ([]*certificate.Certificate, []error) {
	return m.checkAllFunc(ctx, hosts, maxParallel)
}

func TestNew(t *testing.T) {
	cfg := Config{
		URL:       "http://example.com/webhook",
		Threshold: 15,
		Interval:  5 * time.Minute,
	}
	hosts := []checker.Host{
		{Hostname: "a.com", Port: 443, Name: "A"},
	}
	mock := &mockCertChecker{}

	n := New(cfg, mock, hosts)

	if n == nil {
		t.Fatal("New() returned nil")
	}
	if n.cfg.URL != cfg.URL {
		t.Errorf("cfg.URL = %q; want %q", n.cfg.URL, cfg.URL)
	}
	if n.cfg.Threshold != cfg.Threshold {
		t.Errorf("cfg.Threshold = %d; want %d", n.cfg.Threshold, cfg.Threshold)
	}
	if n.cfg.Interval != cfg.Interval {
		t.Errorf("cfg.Interval = %v; want %v", n.cfg.Interval, cfg.Interval)
	}
	if len(n.hosts) != len(hosts) {
		t.Errorf("len(hosts) = %d; want %d", len(n.hosts), len(hosts))
	}
	if n.checker != mock {
		t.Error("checker was not set correctly")
	}
	if n.lastAlerted == nil {
		t.Error("lastAlerted map was not initialized")
	}
}

func TestSendAlert_Success(t *testing.T) {
	var gotPayload alertPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &gotPayload); err != nil {
			t.Errorf("failed to unmarshal payload: %v", err)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q; want %q", r.Header.Get("Content-Type"), "application/json")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := New(Config{URL: srv.URL, Threshold: 10}, nil, nil)
	cert := &certificate.Certificate{
		Hostname:   "test.example.com",
		Port:       443,
		CommonName: "test.example.com",
		Issuer:     "Test CA",
		DaysLeft:   7,
	}

	if err := n.sendAlert(cert); err != nil {
		t.Fatalf("sendAlert() unexpected error: %v", err)
	}
	if gotPayload.Hostname != cert.Hostname {
		t.Errorf("payload.Hostname = %q; want %q", gotPayload.Hostname, cert.Hostname)
	}
	if gotPayload.DaysLeft != cert.DaysLeft {
		t.Errorf("payload.DaysLeft = %d; want %d", gotPayload.DaysLeft, cert.DaysLeft)
	}
}

func TestSendAlert_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	n := New(Config{URL: srv.URL, Threshold: 10}, nil, nil)
	cert := &certificate.Certificate{
		Hostname: "fail.example.com",
		Port:     443,
		DaysLeft: 3,
	}

	err := n.sendAlert(cert)
	if err == nil {
		t.Fatal("sendAlert() expected error, got nil")
	}
}

func TestCheckAndAlert_CallsSendAlert(t *testing.T) {
	var called atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	mock := &mockCertChecker{
		checkFunc: func(_ context.Context, hostname string, port int) (*certificate.Certificate, error) {
			return &certificate.Certificate{
				Hostname: hostname,
				Port:     port,
				DaysLeft: 5,
			}, nil
		},
	}
	n := New(Config{URL: srv.URL, Threshold: 10, Interval: time.Hour}, mock, []checker.Host{
		{Hostname: "alert.com", Port: 443},
	})

	n.checkAndAlert(context.Background())

	if !called.Load() {
		t.Error("expected sendAlert to be called")
	}
}

func TestCheckAndAlert_SkipsWhenAboveThreshold(t *testing.T) {
	var called atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	mock := &mockCertChecker{
		checkFunc: func(_ context.Context, hostname string, port int) (*certificate.Certificate, error) {
			return &certificate.Certificate{
				Hostname: hostname,
				Port:     port,
				DaysLeft: 20,
			}, nil
		},
	}
	n := New(Config{URL: srv.URL, Threshold: 10, Interval: time.Hour}, mock, []checker.Host{
		{Hostname: "safe.com", Port: 443},
	})

	n.checkAndAlert(context.Background())

	if called.Load() {
		t.Error("expected sendAlert NOT to be called when DaysLeft > Threshold")
	}
}

func TestCheckAndAlert_DedupWithinInterval(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	mock := &mockCertChecker{
		checkFunc: func(_ context.Context, hostname string, port int) (*certificate.Certificate, error) {
			return &certificate.Certificate{
				Hostname: hostname,
				Port:     port,
				DaysLeft: 5,
			}, nil
		},
	}
	n := New(Config{URL: srv.URL, Threshold: 10, Interval: time.Hour}, mock, []checker.Host{
		{Hostname: "dedup.com", Port: 443},
	})

	n.checkAndAlert(context.Background())
	n.checkAndAlert(context.Background())

	if callCount.Load() != 1 {
		t.Errorf("expected 1 alert call, got %d", callCount.Load())
	}
}

func TestCheckAndAlert_ReAlertAfterInterval(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	mock := &mockCertChecker{
		checkFunc: func(_ context.Context, hostname string, port int) (*certificate.Certificate, error) {
			return &certificate.Certificate{
				Hostname: hostname,
				Port:     port,
				DaysLeft: 5,
			}, nil
		},
	}
	n := New(Config{URL: srv.URL, Threshold: 10, Interval: 10 * time.Millisecond}, mock, []checker.Host{
		{Hostname: "realert.com", Port: 443},
	})

	n.checkAndAlert(context.Background())
	time.Sleep(50 * time.Millisecond)
	n.checkAndAlert(context.Background())

	if callCount.Load() != 2 {
		t.Errorf("expected 2 alert calls, got %d", callCount.Load())
	}
}

func TestCheckAndAlert_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when context is cancelled")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	mock := &mockCertChecker{
		checkFunc: func(_ context.Context, hostname string, port int) (*certificate.Certificate, error) {
			t.Error("checker should not be called when context is cancelled")
			return nil, nil
		},
	}
	n := New(Config{URL: srv.URL, Threshold: 10, Interval: time.Hour}, mock, []checker.Host{
		{Hostname: "cancel.com", Port: 443},
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	n.checkAndAlert(ctx)
}

func TestStart_BeginsCheckLoop(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	mock := &mockCertChecker{
		checkFunc: func(_ context.Context, hostname string, port int) (*certificate.Certificate, error) {
			return &certificate.Certificate{
				Hostname: hostname,
				Port:     port,
				DaysLeft: 5,
			}, nil
		},
	}
	n := New(Config{URL: srv.URL, Threshold: 10, Interval: 10 * time.Millisecond}, mock, []checker.Host{
		{Hostname: "start.com", Port: 443},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	cancel()

	if callCount.Load() == 0 {
		t.Fatal("expected sendAlert to be called at least once")
	}
}
