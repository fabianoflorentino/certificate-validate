// Package service provides the application's business logic layer, orchestrating
// certificate checking, history recording, and metrics updates.
//
// The CertService type is the main entry point for certificate operations. It
// coordinates between the checker (fetches certificates), history store (records
// results), and metrics updater (exposes Prometheus metrics).
//
// # Architecture
//
// The service layer follows dependency injection: all dependencies are passed
// via the constructor, making the service easy to test and configure. The
// MetricsUpdater is optional and can be nil when metrics are disabled.
//
// # Usage
//
// Create a service and check all configured hosts:
//
//	svc := service.NewCertService(checker, historyStore, metricsUpdater)
//	result := svc.CheckAll(ctx, cfg.Hosts)
//
//	// result.Certificates contains all fetched certificates
//	// result.Errors contains any errors encountered
package service
