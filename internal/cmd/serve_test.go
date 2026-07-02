package cmd

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/fabianoflorentino/certificate-validate/internal/config"
)

// buildDeps returns a valid handler and deps for a minimal config.
func TestBuildDeps_ValidConfig(t *testing.T) {
	cfg := &config.Config{
		Hosts: []config.HostConfig{
			{Name: "test", URL: "example.com", Port: "443"},
		},
	}
	handler, deps, err := buildDeps(cfg)
	if err != nil {
		t.Fatalf("buildDeps() error = %v; want nil", err)
	}
	if handler == nil {
		t.Fatal("buildDeps() returned nil handler")
	}
	if deps == nil {
		t.Fatal("buildDeps() returned nil deps")
	}
	if deps.checker == nil {
		t.Fatal("buildDeps() returned nil checker")
	}
}

// buildDeps handler returns 404 for unknown API hostname (no network call).
func TestBuildDeps_UnknownHostname(t *testing.T) {
	cfg := &config.Config{
		Hosts: []config.HostConfig{
			{Name: "test", URL: "example.com", Port: "443"},
		},
	}
	handler, _, err := buildDeps(cfg)
	if err != nil {
		t.Fatalf("buildDeps() error = %v", err)
	}

	req, _ := http.NewRequest("GET", "/api/v1/cert/info/nonexistent", nil)
	rr := newResponseRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("/api/v1/cert/info/nonexistent status = %d; want %d", rr.Code, http.StatusNotFound)
	}
}

// buildDeps with history enabled sets deps.registry.
func TestBuildDeps_WithHistory(t *testing.T) {
	cfg := &config.Config{
		Hosts: []config.HostConfig{
			{Name: "test", URL: "example.com", Port: "443"},
		},
		History: config.HistoryConf{
			Enabled:  true,
			FilePath: filepath.Join(t.TempDir(), "history.jsonl"),
		},
	}
	_, deps, err := buildDeps(cfg)
	if err != nil {
		t.Fatalf("buildDeps() error = %v; want nil", err)
	}
	if deps.registry == nil {
		t.Fatal("buildDeps() with history enabled returned nil registry")
	}
}

// buildDeps with Prometheus enabled succeeds (gauges are registered at import time).
func TestBuildDeps_WithPrometheus(t *testing.T) {
	cfg := &config.Config{
		Hosts: []config.HostConfig{
			{Name: "test", URL: "example.com", Port: "443"},
		},
		Prometheus: config.PrometheusConf{Enabled: true, Address: ":9090"},
	}
	handler, deps, err := buildDeps(cfg)
	if err != nil {
		t.Fatalf("buildDeps() error = %v; want nil", err)
	}
	if handler == nil {
		t.Fatal("buildDeps() with Prometheus returned nil handler")
	}
	if deps == nil {
		t.Fatal("buildDeps() with Prometheus returned nil deps")
	}
}

// buildDeps with valid trusted CAs succeeds.
func TestBuildDeps_WithTrustedCAs(t *testing.T) {
	caPath := filepath.Join(t.TempDir(), "ca.pem")
	generateCACert(t, caPath)

	cfg := &config.Config{
		Hosts: []config.HostConfig{
			{Name: "test", URL: "example.com", Port: "443"},
		},
		TrustedCAs: []string{caPath},
	}
	_, _, err := buildDeps(cfg)
	if err != nil {
		t.Fatalf("buildDeps() with trusted CAs error = %v; want nil", err)
	}
}

// buildDeps with invalid trusted CAs returns error.
func TestBuildDeps_InvalidTrustedCAs(t *testing.T) {
	cfg := &config.Config{
		Hosts: []config.HostConfig{
			{Name: "test", URL: "example.com", Port: "443"},
		},
		TrustedCAs: []string{"/nonexistent/ca.pem"},
	}
	_, _, err := buildDeps(cfg)
	if err == nil {
		t.Fatal("buildDeps() with invalid CAs should return error, got nil")
	}
}

// buildDeps with per-host trusted CAs succeeds.
func TestBuildDeps_WithPerHostCAs(t *testing.T) {
	caPath := filepath.Join(t.TempDir(), "ca.pem")
	generateCACert(t, caPath)

	cfg := &config.Config{
		Hosts: []config.HostConfig{
			{Name: "test", URL: "example.com", Port: "443", TrustedCAs: []string{caPath}},
		},
	}
	_, _, err := buildDeps(cfg)
	if err != nil {
		t.Fatalf("buildDeps() with per-host CAs error = %v; want nil", err)
	}
}

// startBackground with no features enabled returns immediately and respects context cancellation.
func TestStartBackground_NoFeatures(t *testing.T) {
	cfg := &config.Config{
		CheckTime: 3600,
		Hosts: []config.HostConfig{
			{Name: "test", URL: "example.com", Port: "443"},
		},
	}
	handler, deps, err := buildDeps(cfg)
	if err != nil {
		t.Fatalf("buildDeps() error = %v", err)
	}
	_ = handler

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		startBackground(ctx, cfg, deps)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("startBackground did not stop after context cancellation")
	}
}

// startBackground with history enabled starts and stops cleanly.
func TestStartBackground_WithHistory(t *testing.T) {
	cfg := &config.Config{
		CheckTime: 3600,
		Hosts: []config.HostConfig{
			{Name: "test", URL: "example.com", Port: "443"},
		},
		History: config.HistoryConf{
			Enabled:  true,
			FilePath: filepath.Join(t.TempDir(), "history.jsonl"),
		},
	}
	handler, deps, err := buildDeps(cfg)
	if err != nil {
		t.Fatalf("buildDeps() error = %v", err)
	}
	_ = handler

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		startBackground(ctx, cfg, deps)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("startBackground with history did not stop after context cancellation")
	}
}

// startBackground with already-cancelled context does not panic.
func TestStartBackground_CancelledContext(t *testing.T) {
	cfg := &config.Config{
		CheckTime: 3600,
		Hosts: []config.HostConfig{
			{Name: "test", URL: "example.com", Port: "443"},
		},
	}
	handler, deps, err := buildDeps(cfg)
	if err != nil {
		t.Fatalf("buildDeps() error = %v", err)
	}
	_ = handler

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		startBackground(ctx, cfg, deps)
		close(done)
	}()

	select {
	case <-done:
		// success — no panic
	case <-time.After(2 * time.Second):
		t.Fatal("startBackground with cancelled context did not return")
	}
}

// responseRecorder is a non-exported wrapper around httptest.ResponseRecorder
// that avoids importing httptest in the cmd package's test file.
// We embed *http.Response as a simplified stand-in.
type responseRecorder struct {
	Code     int
	header   http.Header
	body     []byte
}

func newResponseRecorder() *responseRecorder {
	return &responseRecorder{
		Code:   http.StatusOK,
		header: make(http.Header),
	}
}

func (r *responseRecorder) Header() http.Header { return r.header }

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body = append(r.body, b...)
	return len(b), nil
}

func (r *responseRecorder) WriteHeader(code int) { r.Code = code }
