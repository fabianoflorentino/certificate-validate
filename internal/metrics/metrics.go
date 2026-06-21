package metrics

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/fabianoflorentino/certificate-validate/internal/certificate"
	"github.com/fabianoflorentino/certificate-validate/internal/checker"
)

var (
	daysLeftGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "certificate_days_left",
			Help: "Days remaining before certificate expiration",
		},
		[]string{"host", "port"},
	)
	expiredGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "certificate_expired",
			Help: "Whether the certificate has expired (1=expired, 0=valid)",
		},
		[]string{"host", "port"},
	)
)

// Update updates Prometheus gauges from certificate results.
func Update(certs []*certificate.Certificate) {
	for _, c := range certs {
		if c == nil {
			continue
		}
		setGauges(c.Hostname, c.Port, c.DaysLeft)
	}
}

// StartUpdater periodically fetches certificates in the background and updates Prometheus gauges.
func StartUpdater(ctx context.Context, c checker.CertChecker, hosts []checker.Host, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		updateFromChecker(ctx, c, hosts)
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				updateFromChecker(ctx, c, hosts)
			}
		}
	}()
}

func updateFromChecker(ctx context.Context, c checker.CertChecker, hosts []checker.Host) {
	checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	certs, errs := c.CheckAll(checkCtx, hosts, 10)
	if len(errs) > 0 {
		for _, err := range errs {
			slog.Error("metrics fetch error", "error", err)
		}
	}
	Update(certs)
}

func setGauges(hostname string, port, daysLeft int) {
	portStr := strconv.Itoa(port)
	daysLeftGauge.WithLabelValues(hostname, portStr).Set(float64(daysLeft))
	v := 0.0
	if daysLeft <= 0 {
		v = 1.0
	}
	expiredGauge.WithLabelValues(hostname, portStr).Set(v)
}

// Handler returns an http.Handler that serves Prometheus metrics.
func Handler() http.Handler {
	return promhttp.Handler()
}
