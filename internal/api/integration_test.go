package api

import (
	"context"
	"encoding/json"
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

func TestIntegration_HealthEndpoint(t *testing.T) {
	srv := newIntegrationServer(nil, nil, false)
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d; want 200", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("got status %q; want ok", body["status"])
	}
	if _, ok := body["hosts"]; !ok {
		t.Error("expected hosts field in health response")
	}
}

func TestIntegration_AllCertificates(t *testing.T) {
	checker := &mockChecker{
		checkAllFunc: func(_ context.Context, hosts []checker.Host, _ int) ([]*certificate.Certificate, []error) {
			certs := make([]*certificate.Certificate, len(hosts))
			for i, h := range hosts {
				certs[i] = &certificate.Certificate{
					Hostname:   h.Hostname,
					Port:       h.Port,
					CommonName: "*.example.com",
					DaysLeft:   120,
				}
			}
			return certs, nil
		},
	}
	srv := newIntegrationServer(checker, &mockStore{recordFunc: func(_ []*certificate.Certificate) {}}, false)
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/api/v1/cert/info/all")
	if err != nil {
		t.Fatalf("GET /api/v1/cert/info/all: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d; want 200", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	certs, ok := result["certificates"].([]interface{})
	if !ok || len(certs) != 1 {
		t.Errorf("expected 1 certificate, got %d", len(certs))
	}

	certMap, ok := certs[0].(map[string]interface{})
	if !ok {
		t.Fatal("cert is not a map")
	}
	if certMap["hostname"] != "example.com" {
		t.Errorf("got hostname %q; want example.com", certMap["hostname"])
	}
}

func TestIntegration_ByHostname(t *testing.T) {
	checker := &mockChecker{
		checkFunc: func(_ context.Context, hostname string, port int) (*certificate.Certificate, error) {
			return &certificate.Certificate{
				Hostname:   hostname,
				Port:       port,
				CommonName: "*.example.com",
				DaysLeft:   90,
			}, nil
		},
	}
	srv := newIntegrationServer(checker, nil, false)
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/api/v1/cert/info/example.com")
	if err != nil {
		t.Fatalf("GET /api/v1/cert/info/example.com: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d; want 200", resp.StatusCode)
	}

	var cert certificate.Certificate
	if err := json.NewDecoder(resp.Body).Decode(&cert); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cert.Hostname != "example.com" {
		t.Errorf("got hostname %q; want example.com", cert.Hostname)
	}
}

func TestIntegration_ByHostname_NotFound(t *testing.T) {
	srv := newIntegrationServer(nil, nil, false)
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/api/v1/cert/info/nonexistent.com")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("got status %d; want 404", resp.StatusCode)
	}
}

func TestIntegration_CommonName(t *testing.T) {
	checker := &mockChecker{
		checkAllFunc: func(_ context.Context, hosts []checker.Host, _ int) ([]*certificate.Certificate, []error) {
			certs := make([]*certificate.Certificate, len(hosts))
			for i, h := range hosts {
				certs[i] = &certificate.Certificate{
					Hostname:   h.Hostname,
					CommonName: "shared.example.com",
				}
			}
			return certs, nil
		},
	}
	srv := newIntegrationServer(checker, &mockStore{recordFunc: func(_ []*certificate.Certificate) {}}, false)
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/api/v1/cert/info/commonName")
	if err != nil {
		t.Fatalf("GET /api/v1/cert/info/commonName: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d; want 200", resp.StatusCode)
	}

	var names map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&names); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if names["example.com"] != "shared.example.com" {
		t.Errorf("got %q; want shared.example.com", names["example.com"])
	}
}

func TestIntegration_SubjectAltName(t *testing.T) {
	checker := &mockChecker{
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
	}
	srv := newIntegrationServer(checker, &mockStore{recordFunc: func(_ []*certificate.Certificate) {}}, false)
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/api/v1/cert/info/subjectAltName")
	if err != nil {
		t.Fatalf("GET /api/v1/cert/info/subjectAltName: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d; want 200", resp.StatusCode)
	}

	var sans map[string][]string
	if err := json.NewDecoder(resp.Body).Decode(&sans); err != nil {
		t.Fatalf("decode: %v", err)
	}
	got, ok := sans["example.com"]
	if !ok || len(got) < 1 {
		t.Fatal("missing example.com SAN entries")
	}
}

func TestIntegration_ExportJSON(t *testing.T) {
	checker := &mockChecker{
		checkAllFunc: func(_ context.Context, hosts []checker.Host, _ int) ([]*certificate.Certificate, []error) {
			return []*certificate.Certificate{
				{Hostname: "example.com", Port: 443, DaysLeft: 100},
			}, nil
		},
	}
	srv := newIntegrationServer(checker, &mockStore{recordFunc: func(_ []*certificate.Certificate) {}}, false)
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/api/v1/cert/export/json")
	if err != nil {
		t.Fatalf("GET /api/v1/cert/export/json: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d; want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("got Content-Type %q; want application/json", ct)
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, "certificates.json") {
		t.Errorf("Content-Disposition missing certificates.json: %q", cd)
	}
}

func TestIntegration_ExportCSV(t *testing.T) {
	checker := &mockChecker{
		checkAllFunc: func(_ context.Context, hosts []checker.Host, _ int) ([]*certificate.Certificate, []error) {
			return []*certificate.Certificate{
				{Hostname: "example.com", Port: 443, CommonName: "*.example.com", DaysLeft: 100},
			}, nil
		},
	}
	srv := newIntegrationServer(checker, &mockStore{recordFunc: func(_ []*certificate.Certificate) {}}, false)
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/api/v1/cert/export/csv")
	if err != nil {
		t.Fatalf("GET /api/v1/cert/export/csv: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d; want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/csv; charset=utf-8" {
		t.Errorf("got Content-Type %q; want text/csv; charset=utf-8", ct)
	}
}

func TestIntegration_History(t *testing.T) {
	checker := &mockChecker{}
	store := &mockStore{
		getHistoryFunc: func(hostname string) ([]history.Entry, error) {
			return []history.Entry{{Host: hostname, DaysLeft: 50}}, nil
		},
	}
	srv := newIntegrationServer(checker, store, false)
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/api/v1/cert/history/example.com")
	if err != nil {
		t.Fatalf("GET /api/v1/cert/history/example.com: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d; want 200", resp.StatusCode)
	}
}

func TestIntegration_HistoryNotEnabled(t *testing.T) {
	srv := newIntegrationServer(nil, nil, false)
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/api/v1/cert/history/example.com")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("got status %d; want 404", resp.StatusCode)
	}
}

func TestIntegration_SecurityHeaders(t *testing.T) {
	srv := newIntegrationServer(nil, nil, false)
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing X-Content-Type-Options header")
	}
	if resp.Header.Get("X-Frame-Options") != "DENY" {
		t.Error("missing X-Frame-Options header")
	}
}

func TestIntegration_404(t *testing.T) {
	srv := newIntegrationServer(nil, nil, false)
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/api/v1/cert/info/nonexistent")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("got status %d; want 404", resp.StatusCode)
	}
}

func TestIntegration_AllWithErrors(t *testing.T) {
	checker := &mockChecker{
		checkAllFunc: func(_ context.Context, _ []checker.Host, _ int) ([]*certificate.Certificate, []error) {
			return nil, []error{assertAnError{t: "timeout"}}
		},
	}
	srv := newIntegrationServer(checker, &mockStore{recordFunc: func(_ []*certificate.Certificate) {}}, false)
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/api/v1/cert/info/all")
	if err != nil {
		t.Fatalf("GET /api/v1/cert/info/all: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d; want 200", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	errs, ok := result["errors"].([]interface{})
	if !ok || len(errs) == 0 {
		t.Error("expected errors in response")
	}
}

func TestIntegration_PrometheusMetrics(t *testing.T) {
	srv := newIntegrationServer(nil, nil, true)
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		t.Error("expected prometheus /metrics endpoint to be registered")
	}
}

// --- helpers ---

func newIntegrationServer(mc checker.CertChecker, ms history.Store, promEnabled bool) *httptest.Server {
	if mc == nil {
		mc = &mockChecker{
			checkAllFunc: func(_ context.Context, _ []checker.Host, _ int) ([]*certificate.Certificate, []error) {
				return nil, nil
			},
		}
	}
	cfg := &config.Config{
		Hosts: []config.HostConfig{
			{Name: "test", URL: "example.com", Port: "443"},
		},
		Prometheus: config.PrometheusConf{Enabled: promEnabled},
	}
	svc := service.NewCertService(mc, ms, nil)
	h := New(svc, cfg)
	return httptest.NewServer(h.Router())
}

// assertAnError implements the error interface for test assertions.
type assertAnError struct {
	t string
}

func (e assertAnError) Error() string { return e.t }

var _ error = assertAnError{}
