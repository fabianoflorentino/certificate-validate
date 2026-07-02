package api

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fabianoflorentino/certificate-validate/internal/certificate"
	"github.com/fabianoflorentino/certificate-validate/internal/checker"
	"github.com/fabianoflorentino/certificate-validate/internal/config"
	"github.com/fabianoflorentino/certificate-validate/internal/history"
	"github.com/fabianoflorentino/certificate-validate/internal/service"
)

// mockChecker implements checker.CertChecker for api tests.
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

// mockStore implements history.Store for api tests.
type mockStore struct {
	recordFunc     func(results []*certificate.Certificate)
	getHistoryFunc func(hostname string) ([]history.Entry, error)
}

func (m *mockStore) Record(results []*certificate.Certificate) {
	m.recordFunc(results)
}

func (m *mockStore) GetHistory(hostname string) ([]history.Entry, error) {
	return m.getHistoryFunc(hostname)
}

func setupHandler(checker checker.CertChecker, recorder history.Store) *Handler {
	cfg := &config.Config{
		Hosts: []config.HostConfig{
			{Name: "test", URL: "example.com", Port: "443"},
		},
	}
	svc := service.NewCertService(checker, recorder, nil)
	return New(svc, cfg)
}

func TestHandleHealth(t *testing.T) {
	h := setupHandler(&mockChecker{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	h.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d; want %d", w.Code, http.StatusOK)
	}
}

func TestHandleAll_Success(t *testing.T) {
	h := setupHandler(
		&mockChecker{
			checkAllFunc: func(_ context.Context, hosts []checker.Host, _ int) ([]*certificate.Certificate, []error) {
				certs := make([]*certificate.Certificate, len(hosts))
				for i, h := range hosts {
					certs[i] = &certificate.Certificate{Hostname: h.Hostname, DaysLeft: 100}
				}
				return certs, nil
			},
		},
		&mockStore{
			recordFunc: func(results []*certificate.Certificate) {},
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cert/info/all", nil)
	w := httptest.NewRecorder()

	h.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d; want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	certs, ok := resp["certificates"].([]interface{})
	if !ok || len(certs) != 1 {
		t.Errorf("got %d certificates; want 1", len(certs))
	}
}

func TestHandleAll_WithErrors(t *testing.T) {
	h := setupHandler(
		&mockChecker{
			checkAllFunc: func(_ context.Context, hosts []checker.Host, _ int) ([]*certificate.Certificate, []error) {
				return nil, []error{errors.New("timeout")}
			},
		},
		&mockStore{
			recordFunc: func(results []*certificate.Certificate) {},
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cert/info/all", nil)
	w := httptest.NewRecorder()

	h.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d; want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	errs, ok := resp["errors"].([]interface{})
	if !ok || len(errs) != 1 {
		t.Errorf("got %d errors; want 1", len(errs))
	}
}

func TestHandleByHostname_Found(t *testing.T) {
	h := setupHandler(
		&mockChecker{
			checkFunc: func(_ context.Context, hostname string, port int) (*certificate.Certificate, error) {
				return &certificate.Certificate{Hostname: hostname, DaysLeft: 200}, nil
			},
		},
		nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cert/info/example.com", nil)
	w := httptest.NewRecorder()

	h.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d; want 200", w.Code)
	}
}

func TestHandleByHostname_NotFound(t *testing.T) {
	h := setupHandler(&mockChecker{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cert/info/nonexistent.com", nil)
	w := httptest.NewRecorder()

	h.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("got status %d; want 404", w.Code)
	}
}

func TestHandleByHostname_Empty(t *testing.T) {
	h := setupHandler(&mockChecker{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cert/info/", nil)
	w := httptest.NewRecorder()

	h.Router().ServeHTTP(w, req)

	// Go 1.22 mux returns 404 for pattern mismatch (empty {hostname})
	if w.Code != http.StatusNotFound {
		t.Errorf("got status %d; want 404 (mux no match)", w.Code)
	}
}

func TestHandleByHostname_BadGateway(t *testing.T) {
	h := setupHandler(
		&mockChecker{
			checkFunc: func(_ context.Context, hostname string, port int) (*certificate.Certificate, error) {
				return nil, errors.New("connection failed")
			},
		},
		nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cert/info/example.com", nil)
	w := httptest.NewRecorder()

	h.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("got status %d; want 502", w.Code)
	}
}

func TestHandleCommonName(t *testing.T) {
	h := setupHandler(
		&mockChecker{
			checkAllFunc: func(_ context.Context, hosts []checker.Host, _ int) ([]*certificate.Certificate, []error) {
				certs := make([]*certificate.Certificate, len(hosts))
				for i, h := range hosts {
					certs[i] = &certificate.Certificate{Hostname: h.Hostname, CommonName: "*.example.com"}
				}
				return certs, nil
			},
		},
		&mockStore{recordFunc: func(results []*certificate.Certificate) {}},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cert/info/commonName", nil)
	w := httptest.NewRecorder()

	h.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d; want 200", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if _, ok := resp["example.com"]; !ok {
		t.Errorf("expected example.com in response")
	}
}

func TestHandleSubjectAltName(t *testing.T) {
	h := setupHandler(
		&mockChecker{
			checkAllFunc: func(_ context.Context, hosts []checker.Host, _ int) ([]*certificate.Certificate, []error) {
				certs := make([]*certificate.Certificate, len(hosts))
				for i, h := range hosts {
					certs[i] = &certificate.Certificate{
						Hostname:        h.Hostname,
						SubjectAltNames: []string{h.Hostname, "www." + h.Hostname},
					}
				}
				return certs, nil
			},
		},
		&mockStore{recordFunc: func(results []*certificate.Certificate) {}},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cert/info/subjectAltName", nil)
	w := httptest.NewRecorder()

	h.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d; want 200", w.Code)
	}
}

func TestHandleExportJSON(t *testing.T) {
	h := setupHandler(
		&mockChecker{
			checkAllFunc: func(_ context.Context, hosts []checker.Host, _ int) ([]*certificate.Certificate, []error) {
				certs := make([]*certificate.Certificate, len(hosts))
				for i, h := range hosts {
					certs[i] = &certificate.Certificate{Hostname: h.Hostname, DaysLeft: 100}
				}
				return certs, nil
			},
		},
		&mockStore{recordFunc: func(results []*certificate.Certificate) {}},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cert/export/json", nil)
	w := httptest.NewRecorder()

	h.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d; want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("got Content-Type %q; want application/json", ct)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, "certificates.json") {
		t.Errorf("Content-Disposition missing certificates.json: %q", cd)
	}
}

func TestHandleExportCSV(t *testing.T) {
	h := setupHandler(
		&mockChecker{
			checkAllFunc: func(_ context.Context, hosts []checker.Host, _ int) ([]*certificate.Certificate, []error) {
				certs := make([]*certificate.Certificate, len(hosts))
				for i, h := range hosts {
					certs[i] = &certificate.Certificate{
						Hostname:   h.Hostname,
						Port:       h.Port,
						CommonName: "*.example.com",
						DaysLeft:   100,
					}
				}
				return certs, nil
			},
		},
		&mockStore{recordFunc: func(results []*certificate.Certificate) {}},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cert/export/csv", nil)
	w := httptest.NewRecorder()

	h.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d; want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/csv; charset=utf-8" {
		t.Errorf("got Content-Type %q; want text/csv; charset=utf-8", ct)
	}
	if !strings.Contains(w.Body.String(), "example.com") {
		t.Errorf("CSV body missing example.com: %s", w.Body.String())
	}
}

func TestHandleHistory_Found(t *testing.T) {
	h := setupHandler(
		&mockChecker{},
		&mockStore{
			getHistoryFunc: func(hostname string) ([]history.Entry, error) {
				return []history.Entry{{Host: hostname, DaysLeft: 50}}, nil
			},
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cert/history/example.com", nil)
	w := httptest.NewRecorder()

	h.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d; want 200", w.Code)
	}
}

func TestHandleHealth_WithReachableHost(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}

	cfg := &config.Config{
		Hosts: []config.HostConfig{
			{Name: "local", URL: "127.0.0.1", Port: portStr},
		},
	}
	svc := service.NewCertService(&mockChecker{}, nil, nil)
	h := New(svc, cfg)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d; want 200", w.Code)
	}
}

func TestHandleHistory_NotEnabled(t *testing.T) {
	h := setupHandler(&mockChecker{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cert/history/example.com", nil)
	w := httptest.NewRecorder()

	h.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("got status %d; want 404", w.Code)
	}
}

func TestHandleHistory_EmptyHostname(t *testing.T) {
	h := setupHandler(&mockChecker{}, &mockStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cert/history/", nil)
	w := httptest.NewRecorder()

	h.Router().ServeHTTP(w, req)

	// Go 1.22 mux returns 404 for pattern mismatch (empty {hostname})
	if w.Code != http.StatusNotFound {
		t.Errorf("got status %d; want 404 (mux no match)", w.Code)
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusTeapot, map[string]string{"msg": "hello"})
	if w.Code != http.StatusTeapot {
		t.Errorf("got status %d; want 418", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("got Content-Type %q; want application/json", ct)
	}
}

func TestWriteJSON_EncodeError(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]chan int{"ch": make(chan int)})
	if w.Code != http.StatusOK {
		t.Errorf("got status %d; want 200", w.Code)
	}
}

type failWriter struct{}

func (failWriter) Header() http.Header       { return http.Header{} }
func (failWriter) Write([]byte) (int, error) { return 0, errors.New("write error") }
func (failWriter) WriteHeader(int)           {}

func TestHandleExportCSV_WriteError(t *testing.T) {
	h := setupHandler(
		&mockChecker{
			checkAllFunc: func(_ context.Context, hosts []checker.Host, _ int) ([]*certificate.Certificate, []error) {
				certs := make([]*certificate.Certificate, len(hosts))
				for i, h := range hosts {
					certs[i] = &certificate.Certificate{Hostname: h.Hostname, DaysLeft: 100}
				}
				return certs, nil
			},
		},
		&mockStore{recordFunc: func(results []*certificate.Certificate) {}},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cert/export/csv", nil)
	h.Router().ServeHTTP(failWriter{}, req)
	// No assertion - we just verify the handler doesn't panic on write error
}

func TestHandleExportCSV_NilCert(t *testing.T) {
	h := setupHandler(
		&mockChecker{
			checkAllFunc: func(_ context.Context, _ []checker.Host, _ int) ([]*certificate.Certificate, []error) {
				return []*certificate.Certificate{nil, nil}, nil
			},
		},
		&mockStore{recordFunc: func(results []*certificate.Certificate) {}},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cert/export/csv", nil)
	w := httptest.NewRecorder()
	h.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d; want 200", w.Code)
	}
}

func TestHandleHistory_StoreError(t *testing.T) {
	h := setupHandler(
		&mockChecker{},
		&mockStore{
			getHistoryFunc: func(hostname string) ([]history.Entry, error) {
				return nil, errors.New("db error")
			},
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cert/history/example.com", nil)
	w := httptest.NewRecorder()

	h.Router().ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("got status %d; want 500", w.Code)
	}
}

func TestRateLimiter_Exhausted(t *testing.T) {
	rl := newRateLimiter(0, 0) // zero rate, zero burst
	// First call on an empty bucket
	if rl.allow() {
		t.Error("expected false for exhausted rate limiter")
	}
}

func TestRateLimiter_Refill(t *testing.T) {
	rl := newRateLimiter(1000, 2) // high rate, burst 2
	if !rl.allow() {
		t.Error("expected allow (burst available)")
	}
	if !rl.allow() {
		t.Error("expected allow (burst available)")
	}
	if rl.allow() {
		t.Error("expected false after burst exhausted")
	}
}

func TestWithMiddleware_RateLimit(t *testing.T) {
	oldLimiter := defaultLimiter
	defaultLimiter = newRateLimiter(0, 0)
	t.Cleanup(func() { defaultLimiter = oldLimiter })

	h := setupHandler(
		&mockChecker{},
		&mockStore{recordFunc: func(results []*certificate.Certificate) {}},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cert/info/all", nil)
	w := httptest.NewRecorder()
	h.Router().ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("got status %d; want 429", w.Code)
	}
	if ct := w.Header().Get("Retry-After"); ct != "1" {
		t.Errorf("got Retry-After %q; want 1", ct)
	}
}

func TestCORSHeaders(t *testing.T) {
	h := setupHandler(
		&mockChecker{
			checkAllFunc: func(_ context.Context, hosts []checker.Host, _ int) ([]*certificate.Certificate, []error) {
				return nil, nil
			},
		},
		&mockStore{recordFunc: func(results []*certificate.Certificate) {}},
	)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cert/info/all", nil)
	w := httptest.NewRecorder()

	h.Router().ServeHTTP(w, req)

	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing X-Content-Type-Options header")
	}
	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("missing X-Frame-Options header")
	}
}

func TestRouter_RegistersPrometheus(t *testing.T) {
	cfg := &config.Config{
		Prometheus: config.PrometheusConf{Enabled: true},
	}
	svc := service.NewCertService(&mockChecker{}, nil, nil)
	h := New(svc, cfg)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	h.Router().ServeHTTP(w, req)

	if w.Code == http.StatusNotFound {
		t.Error("expected prometheus metrics handler to be registered")
	}
}
