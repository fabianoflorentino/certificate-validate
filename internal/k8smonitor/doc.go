// Package k8smonitor implements Kubernetes certificate monitoring, analyzing
// TLS certificates stored in Secrets and Ingresses, and optionally auto-renewing
// expiring certificates via cert-manager.
//
// The package provides a Monitor that discovers TLS resources in a Kubernetes
// cluster, analyzes each certificate for validity, chain, and revocation status,
// and exports Prometheus metrics with Kubernetes-specific labels (namespace, name,
// kind). It can also fire webhook alerts for certificates approaching expiration.
//
// # Architecture
//
// The monitor consists of several components:
//
//   - Client: wraps the Kubernetes clientset for cluster access
//   - Discovery: lists TLS Secrets and Ingresses across namespaces
//   - Analyzer: parses certificate bundles and builds K8sCertificate models
//   - Monitor: orchestrates scanning, analysis, alerting, and renewal
//   - Renewer: handles auto-renewal via cert-manager annotation
//   - Webhook: rate-limited alert posting to external endpoints
//
// # Auto-Renewal (Phase 2)
//
// When --renew-threshold is set (non-zero), the monitor can auto-renew expiring
// certificates by annotating the owning Secret with cert-manager.io/force-renew.
// The Renewer waits for cert-manager to rotate the certificate serial, then
// validates the renewed certificate (minimum validity, optional revocation check).
// Stuck issuance (serial unchanged after timeout) is detected and reported.
//
// # Metrics
//
// The package uses a dedicated Prometheus registry to avoid metric name collisions
// with the core metrics package. Exported metrics include:
//
//   - certificate_days_left{namespace, name, kind}
//   - certificate_expired{namespace, name, kind}
//   - certificate_revoked{namespace, name, kind}
//   - certificate_renewal_total{namespace, name, status}
//   - certificate_renewal_attempts{namespace, name}
//   - certificate_stuck_issuance{namespace, name}
//
// # Usage
//
// Create and run a monitor:
//
//	cfg := k8smonitor.Config{
//		Kubeconfig:      "",  // in-cluster or default kubeconfig
//		Namespaces:      []string{"default", "cert-manager"},
//		CheckRevocation: true,
//		WatchInterval:   5 * time.Minute,
//		RenewThreshold:  15,  // auto-renew at or below 15 days
//	}
//	monitor, err := k8smonitor.NewMonitor(cfg)
//	if err != nil {
//		return err
//	}
//	return monitor.Run(ctx, cfg.WatchInterval)
package k8smonitor
