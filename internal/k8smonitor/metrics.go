package k8smonitor

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/fabianoflorentino/certificate-validate/internal/certificate"
)

// registry is a dedicated Prometheus registry for the Kubernetes monitor
// so its metric names do not collide with the core metrics package
// (which registers certificate_days_left / certificate_expired on the
// default registry).
var registry = prometheus.NewRegistry()

var (
	daysLeftGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "certificate_days_left",
			Help: "Days remaining before certificate expiration",
		},
		[]string{"namespace", "name", "kind"},
	)
	expiredGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "certificate_expired",
			Help: "Whether the certificate has expired (1=expired, 0=valid)",
		},
		[]string{"namespace", "name", "kind"},
	)
	revokedGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "certificate_revoked",
			Help: "Whether the certificate is revoked (1=revoked, 0=not revoked)",
		},
		[]string{"namespace", "name", "kind"},
	)
	renewalTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "certificate_renewal_total",
			Help: "Total number of renewal attempts by status (success/failure)",
		},
		[]string{"namespace", "name", "status"},
	)
	renewalAttempts = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "certificate_renewal_attempts",
			Help: "Number of renewal attempts for the current certificate",
		},
		[]string{"namespace", "name"},
	)
	stuckIssuance = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "certificate_stuck_issuance",
			Help: "Whether certificate renewal is stuck (1=stuck, 0=not stuck)",
		},
		[]string{"namespace", "name"},
	)
)

func init() {
	registry.MustRegister(daysLeftGauge)
	registry.MustRegister(expiredGauge)
	registry.MustRegister(revokedGauge)
	registry.MustRegister(renewalTotal)
	registry.MustRegister(renewalAttempts)
	registry.MustRegister(stuckIssuance)
}

// UpdateMetrics updates the Prometheus gauges from a set of K8s certificates.
func UpdateMetrics(certs []*K8sCertificate) {
	for _, c := range certs {
		if c == nil {
			continue
		}
		labels := prometheus.Labels{
			"namespace": c.K8sNamespace,
			"name":      c.K8sName,
			"kind":      string(c.K8sKind),
		}
		daysLeftGauge.With(labels).Set(float64(c.DaysLeft))

		expired := 0.0
		if c.DaysLeft <= 0 {
			expired = 1.0
		}
		expiredGauge.With(labels).Set(expired)

		revoked := 0.0
		if c.RevocationStatus == certificate.RevocationRevoked {
			revoked = 1.0
		}
		revokedGauge.With(labels).Set(revoked)
	}
}

// RecordRenewal increments the renewal counter for a certificate with the
// given status label.
func RecordRenewal(ns, name, status string) {
	renewalTotal.WithLabelValues(ns, name, status).Inc()
}

// SetRenewalAttempts records how many attempts a certificate's renewal has
// taken.
func SetRenewalAttempts(ns, name string, attempts int) {
	renewalAttempts.WithLabelValues(ns, name).Set(float64(attempts))
}

// SetStuckIssuance marks whether a certificate's renewal is stuck.
func SetStuckIssuance(ns, name string, stuck bool) {
	val := 0.0
	if stuck {
		val = 1.0
	}
	stuckIssuance.WithLabelValues(ns, name).Set(val)
}

// MetricsHandler returns an http.Handler serving Prometheus metrics from the
// monitor's dedicated registry.
func MetricsHandler() http.Handler {
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}
