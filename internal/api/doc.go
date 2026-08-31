// Package api provides the HTTP API server for certificate validation.
//
// The package implements a RESTful API with endpoints for checking certificates,
// exporting data, and viewing history. It includes rate limiting, optional API
// key authentication, and serves an embedded web dashboard.
//
// # Endpoints
//
//   - GET /: Web Dashboard (embedded)
//   - GET /swagger/: Swagger UI (interactive API docs)
//   - GET /health: Health check
//   - GET /api/v1/cert/info/all: All certificates
//   - GET /api/v1/cert/info/{hostname}: Certificate for a specific host
//   - GET /api/v1/cert/export/json: Download as JSON
//   - GET /api/v1/cert/export/csv: Download as CSV
//   - GET /api/v1/cert/history/{hostname}: Check history
//   - GET /metrics: Prometheus metrics
//
// # Security
//
// The API supports optional API key authentication via the X-API-Key header.
// Rate limiting is enforced per-IP using a token bucket algorithm.
//
// # Usage
//
// Create and start the API server:
//
//	server := api.NewServer(cfg, svc)
//	if err := server.Start(ctx); err != nil {
//		return err
//	}
package api
