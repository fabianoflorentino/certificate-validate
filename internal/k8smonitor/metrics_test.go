package k8smonitor

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fabianoflorentino/certificate-validate/internal/certificate"
)

func TestUpdateMetrics(t *testing.T) {
	certs := []*K8sCertificate{
		{ // healthy
			Certificate:  certificate.Certificate{DaysLeft: 40, RevocationStatus: certificate.RevocationUnknown},
			K8sNamespace: "default", K8sName: "a", K8sKind: KindSecret,
		},
		{ // expired
			Certificate:  certificate.Certificate{DaysLeft: 0, RevocationStatus: certificate.RevocationUnknown},
			K8sNamespace: "default", K8sName: "b", K8sKind: KindSecret,
		},
		{ // revoked
			Certificate:  certificate.Certificate{DaysLeft: 10, RevocationStatus: certificate.RevocationRevoked},
			K8sNamespace: "default", K8sName: "c", K8sKind: KindIngress,
		},
		(*K8sCertificate)(nil),
	}
	UpdateMetrics(certs)

	out := gatherMetrics(t)
	for _, want := range []string{
		`certificate_days_left{kind="Secret",name="a",namespace="default"} 40`,
		`certificate_expired{kind="Secret",name="b",namespace="default"} 1`,
		`certificate_revoked{kind="Ingress",name="c",namespace="default"} 1`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("metrics output missing %q", want)
		}
	}
}

func TestRecordRenewalAndState(t *testing.T) {
	RecordRenewal("default", "a", "success")
	RecordRenewal("default", "a", "failure")
	SetRenewalAttempts("default", "a", 2)
	SetStuckIssuance("default", "a", true)

	out := gatherMetrics(t)
	for _, want := range []string{
		`certificate_renewal_total{name="a",namespace="default",status="success"} 1`,
		`certificate_renewal_total{name="a",namespace="default",status="failure"} 1`,
		`certificate_renewal_attempts{name="a",namespace="default"} 2`,
		`certificate_stuck_issuance{name="a",namespace="default"} 1`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("metrics output missing %q", want)
		}
	}
}

func TestSetStuckIssuanceFalse(t *testing.T) {
	SetStuckIssuance("default", "b", false)
	out := gatherMetrics(t)
	if !strings.Contains(out, `certificate_stuck_issuance{name="b",namespace="default"} 0`) {
		t.Errorf("expected stuck gauge to be 0, got %s", out)
	}
}

func TestMetricsHandler(t *testing.T) {
	RecordRenewal("default", "h", "success")
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	MetricsHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("metrics handler returned status %d; want 200", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), "certificate_renewal_total") {
		t.Error("metrics response did not include renewal metric")
	}
}

func gatherMetrics(t *testing.T) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	MetricsHandler().ServeHTTP(rec, req)
	body, _ := io.ReadAll(rec.Body)
	return string(body)
}
