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
)

func init() {
	registry.MustRegister(daysLeftGauge)
	registry.MustRegister(expiredGauge)
	registry.MustRegister(revokedGauge)
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

// MetricsHandler returns an http.Handler serving Prometheus metrics from the
// monitor's dedicated registry.
func MetricsHandler() http.Handler {
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}
