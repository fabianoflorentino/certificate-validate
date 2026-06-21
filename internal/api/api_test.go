package api

import (
	"context"
	"encoding/json"
	"errors"
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
	json.NewDecoder(w.Body).Decode(&resp)

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
	json.NewDecoder(w.Body).Decode(&resp)

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
						Hostname:       h.Hostname,
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
