---
title: "certificate-validate"
description: "A modern SSL/TLS certificate validation tool with CLI, HTTP API, and web dashboard"
---

## SSL/TLS certificate validation. CLI, API, and dashboard.

A modern Go tool for monitoring certificates with real-time checks, revocation validation (OCSP/CRL), Prometheus metrics, webhook alerts, and an interactive web dashboard.

### Quickstart

Install via Go:

```bash
go install github.com/fabianoflorentino/certificate-validate/cmd/certificate-validate@latest
certificate-validate --help
```

Or use the Docker image:

```bash
docker pull fabianoflorentino/certificate-validate:latest
docker run --rm fabianoflorentino/certificate-validate --help
```

### Key features

- **CLI Mode** — Check certificates with JSON/table output, watch mode, and filtering
- **HTTP API** — RESTful API with Swagger UI, rate limiting, and optional API key auth
- **Web Dashboard** — Interactive UI with search, sort, history charts, and dark/light themes
- **Export** — Export certificate data to JSON or CSV via CLI or API
- **Watch Mode** — Continuous checking with configurable interval
- **Alert Webhook** — POST alerts when certificates approach expiration
- **History** — Records check results in JSONL with automatic rotation
- **Prometheus Metrics** — Exposes `certificate_days_left` and `certificate_expired` gauges
- **Revocation Checks** — OCSP + CRL validation per certificate
- **Kubernetes Monitoring** — `k8s monitor` scans TLS Secrets + Ingresses, exports metrics with K8s labels, fires webhook alerts, and can auto-renew expiring certs via cert-manager (read-only by default; opt-in via `--renew-threshold`)
- **Hot-Reload** — `SIGHUP` reloads config without restarting the server
- **Self-Signed CAs** — Global and per-host trusted CA certificates
- **Environment Variables** — `CV_` prefix overrides for all config fields
- **Concurrent Processing** — Parallel certificate fetching with semaphore
- **Minimal Image** — Small Docker image (~10MB) with non-root user

### CLI Usage

```bash
# Check all hosts from config
certificate-validate check

# Table output
certificate-validate check -o table

# Single host, no config needed
certificate-validate check --host github.com --port 443

# Filter by days remaining
certificate-validate check --min-days 30

# Continuous watch mode
certificate-validate check --watch

# Export to CSV
certificate-validate export -f csv -o certs.csv

# Start API server
certificate-validate serve

# Start API server with TLS
certificate-validate serve --tls-cert cert.pem --tls-key key.pem

# Kubernetes: single scan of TLS Secrets + Ingresses
certificate-validate k8s monitor

# Kubernetes: watch mode with metrics + webhook alerts
certificate-validate k8s monitor --watch-interval=300 --metrics-addr=:9102 --webhook-url https://hooks.example.com/alert

# Kubernetes: enable auto-renewal of expiring certs via cert-manager
certificate-validate k8s monitor --renew-threshold=15
```

### API Endpoints

| Method | Route | Description |
|--------|-------|-------------|
| GET | `/` | Web Dashboard |
| GET | `/swagger/` | Swagger UI (interactive API docs) |
| GET | `/health` | Health check |
| GET | `/api/v1/cert/info/all` | All certificates |
| GET | `/api/v1/cert/info/{hostname}` | Certificate for a specific host |
| GET | `/api/v1/cert/export/json` | Download as JSON |
| GET | `/api/v1/cert/export/csv` | Download as CSV |
| GET | `/api/v1/cert/history/{hostname}` | Check history |
| GET | `/metrics` | Prometheus metrics |

### Configuration

```yaml
check_time: 30                    # Check interval (seconds)
api_key: "sk-1234"                # Optional API key auth

app_configs:
  - name: 'certificate-validate'
    host: '0.0.0.0'
    port: '5000'

hosts:
  - name: "GitHub"
    url: 'github.com'
    port: '443'
    # ports: [443, 8443]          # Multiple ports
    # timeout: 10                 # Per-host timeout
    # trusted_cas:                # Per-host CAs
    #   - '/certs/internal-ca.pem'

prometheus:
  enabled: false                  # Enable /metrics endpoint

webhook:
  url: 'https://hooks.example.com/alert'
  threshold: 15                   # Alert when days left < threshold
  interval: 1800

history:
  enabled: true
  file_path: "data/history.jsonl"
  max_entries: 10000
  max_days: 90

trusted_cas:
  - '/etc/certificates/my-ca.pem'
```

### Docker & Kubernetes

```bash
# Docker CLI
docker run -v $(pwd)/config:/app/config certificate-validate check

# Docker API server
docker run -p 5000:5000 -v $(pwd)/config:/app/config certificate-validate serve

# Docker Compose
docker-compose up -d
```

Kubernetes manifests for the `k8s monitor` agent (DaemonSet, RBAC, Service,
ServiceMonitor) live in [`kubernetes/monitor/`](https://github.com/fabianoflorentino/certificate-validate/tree/main/kubernetes/monitor).
A disposable dev environment (kind + cert-manager) is documented in
[`dev/README.md`](https://github.com/fabianoflorentino/certificate-validate/blob/main/dev/README.md)
and driven by the `make dev/*` targets.

### Learn more

- [Architecture Documentation](https://github.com/fabianoflorentino/certificate-validate/blob/main/docs/ARCHITECTURE.md)
- [Kubernetes Integration Guide](https://github.com/fabianoflorentino/certificate-validate/blob/main/docs/K8S_INTEGRATION.md)
- [Go Documentation](https://pkg.go.dev/github.com/fabianoflorentino/certificate-validate)
- [Docker Hub](https://hub.docker.com/r/fabianoflorentino/certificate-validate)
- [GitHub Releases](https://github.com/fabianoflorentino/certificate-validate/releases)
