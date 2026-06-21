package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/fabianoflorentino/certificate-validate/internal/certificate"
	"github.com/fabianoflorentino/certificate-validate/internal/checker"
)

func setupTestMetrics() *prometheus.Registry {
	registry := prometheus.NewRegistry()
	daysLeftGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "certificate_days_left",
			Help: "Days remaining before certificate expiration",
		},
		[]string{"host", "port"},
	)
	expiredGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "certificate_expired",
			Help: "Whether the certificate has expired (1=expired, 0=valid)",
		},
		[]string{"host", "port"},
	)
	registry.MustRegister(daysLeftGauge, expiredGauge)
	return registry
}

func TestUpdate(t *testing.T) {
	setupTestMetrics()

	certs := []*certificate.Certificate{
		{Hostname: "a.com", Port: 443, DaysLeft: 10},
		{Hostname: "b.com", Port: 80, DaysLeft: -2},
	}
	Update(certs)

	dlA, _ := daysLeftGauge.GetMetricWithLabelValues("a.com", "443")
	if testutil.ToFloat64(dlA) != 10 {
		t.Errorf("daysLeft for a.com = %v; want 10", testutil.ToFloat64(dlA))
	}
	expA, _ := expiredGauge.GetMetricWithLabelValues("a.com", "443")
	if testutil.ToFloat64(expA) != 0 {
		t.Errorf("expired for a.com = %v; want 0", testutil.ToFloat64(expA))
	}

	dlB, _ := daysLeftGauge.GetMetricWithLabelValues("b.com", "80")
	if testutil.ToFloat64(dlB) != -2 {
		t.Errorf("daysLeft for b.com = %v; want -2", testutil.ToFloat64(dlB))
	}
	expB, _ := expiredGauge.GetMetricWithLabelValues("b.com", "80")
	if testutil.ToFloat64(expB) != 1 {
		t.Errorf("expired for b.com = %v; want 1", testutil.ToFloat64(expB))
	}
}

func TestUpdate_NilCert(t *testing.T) {
	setupTestMetrics()

	certs := []*certificate.Certificate{
		nil,
		{Hostname: "only.com", Port: 443, DaysLeft: 5},
	}
	Update(certs)

	dl, _ := daysLeftGauge.GetMetricWithLabelValues("only.com", "443")
	if testutil.ToFloat64(dl) != 5 {
		t.Errorf("daysLeft for only.com = %v; want 5", testutil.ToFloat64(dl))
	}
	// nil cert should not cause panic
}

func TestHandler(t *testing.T) {
	h := Handler()
	if h == nil {
		t.Fatal("Handler() returned nil")
	}
}

func TestSetGauges_PositiveDaysLeft(t *testing.T) {
	setupTestMetrics()

	setGauges("valid.com", 443, 30)

	dl, _ := daysLeftGauge.GetMetricWithLabelValues("valid.com", "443")
	if testutil.ToFloat64(dl) != 30 {
		t.Errorf("daysLeft = %v; want 30", testutil.ToFloat64(dl))
	}
	exp, _ := expiredGauge.GetMetricWithLabelValues("valid.com", "443")
	if testutil.ToFloat64(exp) != 0 {
		t.Errorf("expired = %v; want 0", testutil.ToFloat64(exp))
	}
}

func TestSetGauges_ZeroDaysLeft(t *testing.T) {
	setupTestMetrics()

	setGauges("zero.com", 443, 0)

	dl, _ := daysLeftGauge.GetMetricWithLabelValues("zero.com", "443")
	if testutil.ToFloat64(dl) != 0 {
		t.Errorf("daysLeft = %v; want 0", testutil.ToFloat64(dl))
	}
	exp, _ := expiredGauge.GetMetricWithLabelValues("zero.com", "443")
	if testutil.ToFloat64(exp) != 1 {
		t.Errorf("expired = %v; want 1", testutil.ToFloat64(exp))
	}
}

func TestSetGauges_NegativeDaysLeft(t *testing.T) {
	setupTestMetrics()

	setGauges("neg.com", 443, -3)

	dl, _ := daysLeftGauge.GetMetricWithLabelValues("neg.com", "443")
	if testutil.ToFloat64(dl) != -3 {
		t.Errorf("daysLeft = %v; want -3", testutil.ToFloat64(dl))
	}
	exp, _ := expiredGauge.GetMetricWithLabelValues("neg.com", "443")
	if testutil.ToFloat64(exp) != 1 {
		t.Errorf("expired = %v; want 1", testutil.ToFloat64(exp))
	}
}

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

func TestUpdateFromChecker(t *testing.T) {
	setupTestMetrics()

	mock := &mockCertChecker{
		checkAllFunc: func(_ context.Context, hosts []checker.Host, maxParallel int) ([]*certificate.Certificate, []error) {
			return []*certificate.Certificate{
				{Hostname: "mock.com", Port: 443, DaysLeft: 42},
			}, nil
		},
	}

	updateFromChecker(context.Background(), mock, []checker.Host{
		{Hostname: "mock.com", Port: 443},
	})

	dl, _ := daysLeftGauge.GetMetricWithLabelValues("mock.com", "443")
	if testutil.ToFloat64(dl) != 42 {
		t.Errorf("daysLeft = %v; want 42", testutil.ToFloat64(dl))
	}
	exp, _ := expiredGauge.GetMetricWithLabelValues("mock.com", "443")
	if testutil.ToFloat64(exp) != 0 {
		t.Errorf("expired = %v; want 0", testutil.ToFloat64(exp))
	}
}

func TestStartUpdater(t *testing.T) {
	setupTestMetrics()

	mock := &mockCertChecker{
		checkAllFunc: func(_ context.Context, hosts []checker.Host, maxParallel int) ([]*certificate.Certificate, []error) {
			return []*certificate.Certificate{
				{Hostname: "up.com", Port: 443, DaysLeft: 100},
			}, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	StartUpdater(ctx, mock, []checker.Host{{Hostname: "up.com", Port: 443}}, 10*time.Millisecond)

	// Give it time to run at least once
	time.Sleep(50 * time.Millisecond)
	cancel()

	// If we reach here without deadlock or panic, updater started and cancelled successfully
}
