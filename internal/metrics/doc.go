// Package metrics provides Prometheus metrics exposition for certificate
// validation results.
//
// The package defines Prometheus gauges for tracking certificate expiration
// and exposes an HTTP handler for the /metrics endpoint. It also provides
// a background updater that periodically fetches certificates and updates
// the metrics.
//
// # Metrics
//
// The following metrics are exported:
//
//   - certificate_days_left{host, port}: days remaining before expiration
//   - certificate_expired{host, port}: whether the certificate has expired (0 or 1)
//
// # Usage
//
// Update metrics from certificate results:
//
//	metrics.Update(certs)
//
// Start a background updater:
//
//	metrics.StartUpdater(ctx, checker, hosts, 5*time.Minute)
//
// Serve metrics on an HTTP endpoint:
//
//	http.Handle("/metrics", metrics.Handler())
package metrics
