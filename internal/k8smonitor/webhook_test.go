package k8smonitor

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fabianoflorentino/certificate-validate/internal/certificate"
)

func TestAlertIfNeeded(t *testing.T) {
	var calls int32
	var received k8sAlertPayload

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	w := newWebhook(srv.URL, 15, 10*time.Minute)

	certs := []*K8sCertificate{
		{ // above threshold -> no alert
			Certificate:  certificate.Certificate{DaysLeft: 40},
			K8sNamespace: "default", K8sName: "healthy", K8sKind: KindSecret,
		},
		{ // at threshold -> alert
			Certificate:  certificate.Certificate{DaysLeft: 10},
			K8sNamespace: "default", K8sName: "expiring", K8sKind: KindSecret,
		},
	}

	w.AlertIfNeeded(certs)

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("got %d webhook calls; want 1", got)
	}
	if received.Name != "expiring" {
		t.Errorf("got alert name %q; want %q", received.Name, "expiring")
	}
	if received.DaysLeft != 10 {
		t.Errorf("got days left %d; want 10", received.DaysLeft)
	}
	if received.Threshold != 15 {
		t.Errorf("got threshold %d; want 15", received.Threshold)
	}

	// Second alert within the min interval should be suppressed.
	w.AlertIfNeeded(certs)
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("got %d webhook calls after re-alert; want 1", got)
	}
}

func TestAlertIfNeededRevokedOrExpired(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	w := newWebhook(srv.URL, 15, time.Minute)
	// Expired cert should alert even with daysLeft 0.
	w.AlertIfNeeded([]*K8sCertificate{{
		Certificate:  certificate.Certificate{DaysLeft: 0},
		K8sNamespace: "default", K8sName: "expired", K8sKind: KindSecret,
	}})

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("got %d webhook calls; want 1", got)
	}
}

func TestAlertIfNeededServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	w := newWebhook(srv.URL, 15, time.Minute)
	w.AlertIfNeeded([]*K8sCertificate{{
		Certificate:  certificate.Certificate{DaysLeft: 5},
		K8sNamespace: "default", K8sName: "failing", K8sKind: KindSecret,
	}})
	// No panic expected; alert state should not be recorded on failure,
	// so a subsequent attempt will retry.
	w.AlertIfNeeded([]*K8sCertificate{{
		Certificate:  certificate.Certificate{DaysLeft: 5},
		K8sNamespace: "default", K8sName: "failing", K8sKind: KindSecret,
	}})
}

func TestAlertIfNeededNilCert(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	w := newWebhook(srv.URL, 15, time.Minute)
	w.AlertIfNeeded([]*K8sCertificate{nil})
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Errorf("got %d webhook calls; want 0", got)
	}
}
