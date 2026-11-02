---
title: "certificate-validate"
description: "Monitor SSL/TLS certificates from the CLI, HTTP API, or Kubernetes with OCSP/CRL checks, Prometheus metrics, and webhook alerts."
---

## Quickstart

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

## Features

<div class="feature-grid">
    <article class="feature-card">
        <span class="feature-kicker">01 · CLI</span>
        <h3>CLI &amp; export</h3>
        <p>Check one host or every configured endpoint. Emit JSON or aligned tables, and export to CSV from the CLI or API.</p>
    </article>
    <article class="feature-card">
        <span class="feature-kicker">02 · API</span>
        <h3>HTTP API &amp; dashboard</h3>
        <p>REST endpoints, Swagger UI, API-key auth, rate limiting, and an embedded dashboard with history charts.</p>
    </article>
    <article class="feature-card">
        <span class="feature-kicker">03 · MONITOR</span>
        <h3>Watch mode</h3>
        <p>Re-check on an interval and surface certificates that cross your days-left threshold.</p>
    </article>
    <article class="feature-card">
        <span class="feature-kicker">04 · REVOCATION</span>
        <h3>OCSP + CRL</h3>
        <p>Verify revocation with OCSP first and fall back to CRL distribution points when needed.</p>
    </article>
    <article class="feature-card">
        <span class="feature-kicker">05 · KUBERNETES</span>
        <h3>Cluster monitor</h3>
        <p>Discover TLS Secrets and Ingresses, analyze chains, export metrics, and auto-renew via cert-manager when enabled.</p>
    </article>
    <article class="feature-card">
        <span class="feature-kicker">06 · METRICS</span>
        <h3>Prometheus ready</h3>
        <p>Expose <code>certificate_days_left</code>, <code>certificate_expired</code>, <code>certificate_revoked</code>, and renewal counters.</p>
    </article>
    <article class="feature-card">
        <span class="feature-kicker">07 · ALERTS</span>
        <h3>Webhook alerts</h3>
        <p>POST structured JSON when a certificate approaches expiry, rate-limited per resource to avoid alert spam.</p>
    </article>
    <article class="feature-card">
        <span class="feature-kicker">08 · OPS</span>
        <h3>History &amp; hot reload</h3>
        <p>Record check results as JSONL with automatic rotation, and reload configuration on <code>SIGHUP</code> without a restart.</p>
    </article>
    <article class="feature-card">
        <span class="feature-kicker">09 · TRUST</span>
        <h3>Private CAs</h3>
        <p>Trust global or per-host CA bundles for internal, self-signed, and privately-signed certificates.</p>
    </article>
    <article class="feature-card">
        <span class="feature-kicker">10 · RUNTIME</span>
        <h3>Concurrent fetching</h3>
        <p>Check many hosts in parallel with a bounded semaphore, keeping scans fast and predictable.</p>
    </article>
    <article class="feature-card">
        <span class="feature-kicker">11 · CONFIG</span>
        <h3>Environment overrides</h3>
        <p>Override any configuration field with <code>CV_</code>-prefixed environment variables.</p>
    </article>
    <article class="feature-card">
        <span class="feature-kicker">12 · IMAGE</span>
        <h3>Minimal footprint</h3>
        <p>Ship a ~10 MB non-root container image with the CLI, API, and Kubernetes monitor in one binary.</p>
    </article>
</div>

## Demo

Real output from live runs of the tool.

<div class="demo-gallery">
    <figure class="demo-shot demo-shot-wide">
        <div class="demo-frame">
            <img src="demo/dashboard.png" alt="certificate-validate dashboard showing ten TLS certificates with days remaining and validation status" loading="lazy">
        </div>
        <figcaption><strong>Dashboard</strong> — search, sort, expiry badges, revocation status, and history in the embedded web UI.</figcaption>
    </figure>
    <div class="demo-grid">
        <figure class="demo-shot">
            <div class="demo-frame">
                <img src="demo/cli.png" alt="certificate-validate CLI table showing ten hosts, days remaining, status, revocation, issuer, and TLS version" loading="lazy">
            </div>
            <figcaption><strong>CLI</strong> — one command, a full table of hosts, expiry, revocation, issuer, and TLS version.</figcaption>
        </figure>
        <figure class="demo-shot">
            <div class="demo-frame">
                <img src="demo/kubernetes.png" alt="certificate-validate k8s monitor showing Prometheus metrics and a webhook alert payload" loading="lazy">
            </div>
            <figcaption><strong>Kubernetes</strong> — TLS Secret and Ingress discovery, Prometheus metrics, and structured webhook alerts.</figcaption>
        </figure>
    </div>
</div>

## CLI Usage

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

## API Endpoints

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

## Observability

Prometheus metrics are exposed at `/metrics` (core server) or on the
`k8s monitor` metrics address, using the `certificate_` prefix:

| Metric | Labels | Meaning |
|--------|--------|---------|
| `certificate_days_left` | `host`, `port` (core); `namespace`, `name`, `kind` (K8s) | Days remaining before expiration |
| `certificate_expired` | same | `1` if expired, `0` if valid |
| `certificate_revoked` | `namespace`, `name`, `kind` | `1` if OCSP/CRL reports revoked |
| `certificate_renewal_total` | `namespace`, `name`, `status` | Renewal attempts (`success`/`failure`) |

```promql
# Certificates expiring within 15 days
certificate_days_left < 15

# Expired certificates
certificate_expired == 1
```

Expiring certificates POST a JSON alert to the configured webhook. A runnable
end-to-end demo (kind + cert-manager + webhook receiver) is available through
`make dev/up`, `make dev/setup`, and `make dev/alert-demo`; see the
[development guide](https://github.com/fabianoflorentino/certificate-validate/blob/main/dev/README.md#end-to-end-monitoring-and-alerting-demo).

## Configuration

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

## Docker & Kubernetes

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

## Learn more

- [Architecture Documentation](https://github.com/fabianoflorentino/certificate-validate/blob/main/docs/ARCHITECTURE.md)
- [Kubernetes Integration Guide](https://github.com/fabianoflorentino/certificate-validate/blob/main/docs/K8S_INTEGRATION.md)
- [Go Documentation](https://pkg.go.dev/github.com/fabianoflorentino/certificate-validate)
- [Docker Hub](https://hub.docker.com/r/fabianoflorentino/certificate-validate)
- [GitHub Releases](https://github.com/fabianoflorentino/certificate-validate/releases)
