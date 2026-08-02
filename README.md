# Certificate Validate

A modern, extensible SSL/TLS certificate validation tool written in Go. Fetches and inspects certificate information from remote hosts via CLI or HTTP API.

## Features

- **CLI Mode** — Check certificates with output formats (JSON/table), watch mode, filtering
- **HTTP API** — RESTful API with Swagger UI, rate limiting, optional API key auth
- **Export** — Export certificate data to JSON or CSV via CLI or API
- **Watch Mode** — Continuous checking with configurable interval
- **Alert Webhook** — POST alerts when certificates approach expiration
- **History** — Records check results in JSONL with automatic rotation
- **Prometheus Metrics** — Exposes `certificate_days_left` and `certificate_expired` gauges
- **Revocation Checks** — OCSP + CRL validation per certificate
- **Hot-Reload** — `SIGHUP` reloads config without restarting the server
- **Self-Signed CAs** — Global and per-host trusted CA certificates
- **Environment Variables** — `CV_` prefix overrides for all config fields
- **Concurrent Processing** — Parallel certificate fetching with semaphore
- **Minimal Dependencies** — Small Docker image (~10MB)
- **Extensible Architecture** — SOLID principles with clean interfaces

## Quick Start

```bash
# From source
go install github.com/fabianoflorentino/certificate-validate/cmd/certificate-validate@latest

# Build locally
git clone https://github.com/fabianoflorentino/certificate-validate.git
cd certificate-validate
go build -o certificate-validate ./cmd/certificate-validate

# Docker
docker build -t certificate-validate .

# Run CLI check
./certificate-validate check

# Run API server
./certificate-validate serve
```

## Documentation

| Resource | Description |
| --- | --- |
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | Architecture overview with 13 Mermaid flowcharts |
| Swagger UI | Interactive API docs at `http://localhost:5000/swagger/` (when server is running) |
| [`docs/swagger.yaml`](docs/swagger.yaml) | OpenAPI 3.0 specification |

## Architecture

```bash
certificate-validate/
├── cmd/certificate-validate/
│   └── main.go                    # Entry point
├── config/
│   └── settings.yml               # YAML configuration
├── docs/
│   ├── ARCHITECTURE.md            # Architecture & flowcharts
│   └── swagger.yaml               # OpenAPI 3.0 spec
├── internal/
│   ├── api/                       # HTTP handlers + middleware + rate limiter
│   │   ├── api.go                 # Routes, handlers, auth, rate limiting
│   │   └── static/                # Embedded frontend + Swagger UI
│   ├── certificate/               # Domain: Certificate value object
│   │   ├── certificate.go         # FromX509, TLSVersionString, BuildChain
│   │   └── errors.go              # Domain-specific errors
│   ├── checker/                   # Use case: orchestration
│   │   └── checker.go             # Checker (Fetcher + Formatter)
│   ├── cmd/                       # CLI (Cobra)
│   │   ├── root.go                # Root + completion + global flags
│   │   ├── check.go               # Check certificates
│   │   ├── serve.go               # HTTP API server
│   │   ├── export.go              # Export JSON/CSV
│   │   └── version.go             # Version info
│   ├── config/                    # YAML loader + env var overrides
│   │   └── config.go              # Load, Validate, applyEnvOverrides
│   ├── fetcher/                   # TLS fetcher with CA support
│   │   └── fetcher.go             # Fetcher + per-host CAs
│   ├── formatter/                 # Output formatters
│   │   └── formatter.go           # FormatTable, FormatJSON, FormatCSV
│   ├── history/                   # JSONL history recorder
│   │   └── history.go             # Record, rotate, query
│   ├── metrics/                   # Prometheus exposition
│   │   └── metrics.go             # Gauges + /metrics handler
│   ├── notifier/                  # Webhook alerts
│   │   └── notifier.go            # Periodic alert checks
│   ├── revocation/                # OCSP + CRL checks
│   │   └── revocation.go          # CheckOCSP, CheckCRL
│   └── service/                   # Facade layer
│       └── service.go             # CertService
├── Dockerfile
├── docker-compose.yml
├── Makefile
└── README.md
```

## Configuration

Create a `config/settings.yml` file:

```yaml
check_time: 30                    # Interval for watch/history/metrics (seconds)
api_key: "sk-1234"                # API key for X-API-Key auth (optional)

app_configs:
  - name: 'certificate-validate'
    host: '0.0.0.0'
    port: '5000'
    environment: 'production'
    debug: false

hosts:
  - name: "GitHub"
    url: 'github.com'
    port: '443'
    # ports: [443, 8443]          # Multiple ports per host (optional)
    # timeout: 10                  # Per-host dial timeout (seconds)
    # trusted_cas:                 # Per-host trusted CAs
    #   - '/certs/internal-ca.pem'

prometheus:
  enabled: false                   # Enable Prometheus metrics at /metrics
  # address: ':9090'

webhook:
  # url: 'https://hooks.example.com/alert'
  threshold: 15                    # Alert when days left drops below this
  interval: 1800                   # Check interval (seconds)

history:
  enabled: true                    # Record check history to JSONL
  file_path: "data/history.jsonl"
  max_entries: 10000               # Rotate after this many entries
  max_days: 90                     # Remove entries older than this

trusted_cas:
  # - '/etc/certificates/my-ca.pem'
```

### Environment Variables

All config fields can be overridden via environment variables with the `CV_` prefix:

| Variable | Overrides |
| --- | --- |
| `CV_CHECK_TIME` | `check_time` |
| `CV_API_KEY` | `api_key` |
| `CV_APP_HOST` | `app_configs[0].host` |
| `CV_APP_PORT` | `app_configs[0].port` |
| `CV_PROMETHEUS_ENABLED` | `prometheus.enabled` |
| `CV_PROMETHEUS_ADDRESS` | `prometheus.address` |
| `CV_WEBHOOK_URL` | `webhook.url` |
| `CV_WEBHOOK_THRESHOLD` | `webhook.threshold` |
| `CV_WEBHOOK_INTERVAL` | `webhook.interval` |
| `CV_HISTORY_ENABLED` | `history.enabled` |
| `CV_HISTORY_FILE_PATH` | `history.file_path` |
| `CV_HISTORY_MAX_ENTRIES` | `history.max_entries` |
| `CV_HISTORY_MAX_DAYS` | `history.max_days` |
| `CV_TRUSTED_CAS` | `trusted_cas` (comma-separated) |

Example:

```bash
CV_API_KEY="sk-secret" CV_WEBHOOK_URL="https://hooks.example.com/alert" ./certificate-validate serve
```

## CLI Usage

### Global Flags

| Flag | Description |
| --- | --- |
| `-c, --config` | Path to config file (default: `config/settings.yml`) |
| `--log-file` | Write structured logs to file (also written to stderr) |

### `certificate-validate check`

Check certificates from configured hosts or a single host.

```bash
./certificate-validate check                          # All hosts from config
./certificate-validate check -o table                 # Table output
./certificate-validate check --host github.com        # Single host, no config
./certificate-validate check --host github.com --port 8443
./certificate-validate check --min-days 30            # Only show certs ≤30 days
./certificate-validate check --watch                  # Continuous checking
```

| Flag | Description |
| --- | --- |
| `-w, --watch` | Continuously check at the configured interval |
| `-o, --output` | Output format: `json` (default) or `table` |
| `--host` | Check a single host directly (no config needed) |
| `--port` | Port for `--host` (default: 443) |
| `--min-days` | Only show certificates with ≤ this many days remaining |

### `certificate-validate serve`

Start the HTTP API server.

```bash
./certificate-validate serve                                    # HTTP on :5000
./certificate-validate serve --tls-cert cert.pem --tls-key key.pem  # HTTPS
./certificate-validate serve --api-key "sk-1234"                # Require API key
```

| Flag | Description |
| --- | --- |
| `--tls-cert` | Path to TLS certificate file |
| `--tls-key` | Path to TLS private key file |
| `--api-key` | API key (overrides `config/api_key`) |

Send `SIGHUP` to reload configuration without restarting:

```bash
kill -HUP <pid>
```

### `certificate-validate export`

Export certificate data to JSON or CSV.

```bash
./certificate-validate export                            # JSON to stdout
./certificate-validate export -f csv                     # CSV to stdout
./certificate-validate export -f json -o data/certs.json # Write to file
```

| Flag | Description |
| --- | --- |
| `-f, --format` | Output format: `json` (default) or `csv` |
| `-o, --output-file` | Write to file instead of stdout |

### `certificate-validate version`

Print build information.

```bash
$ ./certificate-validate version
certificate-validate v1.0.0
  commit:     abc1234
  built:      2026-07-02T00:00:00Z
  go:         go1.24 linux/amd64
```

### `certificate-validate completion`

Generate shell completion scripts.

```bash
source <(./certificate-validate completion bash)
source <(./certificate-validate completion zsh)
./certificate-validate completion fish | source
```

## API

The API server is available at `http://localhost:5000`. Interactive docs at [http://localhost:5000/swagger/](http://localhost:5000/swagger/).

### API Key Authentication

When `api_key` is configured (via config file, `--api-key` flag, or `CV_API_KEY` env var), all routes except `/health` require the `X-API-Key` header:

```bash
curl -H "X-API-Key: sk-1234" http://localhost:5000/api/v1/cert/info/all
```

### Rate Limiting

All requests (except static files) are rate-limited via a token bucket: **100 req/s**, burst **200**. Exceeded requests receive `429 Too Many Requests` with a `Retry-After: 1` header.

### Endpoints

| Method | Route | Description |
| --- | --- | --- |
| GET | `/` | Dashboard (embedded frontend) |
| GET | `/swagger/` | Swagger UI (interactive API docs) |
| GET | `/swagger.yaml` | OpenAPI 3.0 specification |
| GET | `/health` | Health check (pings all hosts) |
| GET | `/api/v1/cert/info/all` | All certificates from configured hosts |
| GET | `/api/v1/cert/info/{hostname}` | Certificate for a specific host |
| GET | `/api/v1/cert/info/commonName` | Map of hostname → Common Name |
| GET | `/api/v1/cert/info/subjectAltName` | Map of hostname → SANs |
| GET | `/api/v1/cert/export/json` | Download all certificates as JSON file |
| GET | `/api/v1/cert/export/csv` | Download all certificates as CSV file |
| GET | `/api/v1/cert/history/{hostname}` | History of checks for a host |
| GET | `/metrics` | Prometheus metrics (if `prometheus.enabled`) |

### Example Response

```json
{
  "commonName": "github.com",
  "subjectAltName": ["github.com", "www.github.com"],
  "issuer": "Sectigo Public Server Authentication CA DV E36",
  "type": "Domain Validation (DV) Web Server SSL Digital Certificate",
  "notBefore": "2024-01-01 00:00:00",
  "notAfter": "2025-01-01 23:59:59",
  "daysLeft": 365,
  "crl": ["http://crl.example.com/ca.crl"],
  "ocsp": ["http://ocsp.example.com"],
  "revocationStatus": "unknown",
  "hostname": "github.com",
  "port": 443,
  "tlsVersion": "TLS 1.3",
  "cipherSuite": "TLS_AES_128_GCM_SHA256",
  "chain": [
    {
      "subject": "CN=github.com,O=GitHub\\, Inc.,L=San Francisco,ST=California,C=US",
      "issuer": "CN=Sectigo Public Server Authentication CA DV E36",
      "notAfter": "2025-01-01 23:59:59",
      "fingerprint": "a1b2c3d4e5f6..."
    }
  ]
}
```

## Docker

### CLI Mode

```bash
docker run -v $(pwd)/config:/app/config certificate-validate check
docker run -v $(pwd)/config:/app/config certificate-validate check --watch
```

### API Mode

```bash
docker run -p 5000:5000 -v $(pwd)/config:/app/config certificate-validate serve
```

### Docker Compose

```bash
docker-compose up -d
curl http://localhost:5000/api/v1/cert/info/all
curl http://localhost:5000/swagger/
```

## Self-Signed Certificate Monitoring

Add CA certificates to `trusted_cas` for servers using self-signed or privately-signed certificates:

```yaml
trusted_cas:
  - '/etc/certificates/my-ca.pem'
```

Per-host CAs are also supported:

```yaml
hosts:
  - name: "Internal"
    url: 'internal.example.com'
    port: '443'
    trusted_cas:
      - '/etc/certificates/internal-ca.pem'
```

Supported modes:

- `check` and `check --watch` — uses configured CAs
- `serve` — uses CAs for all certificate fetches

## Development

```bash
make test               # Run all tests with race detector
make test/short         # Fast tests (no race)
make test/cover         # Coverage HTML report
make test/verbose       # Verbose output
make lint               # golangci-lint + go vet
make fmt                # gofmt all source
make build              # Build binary
make run                # Build and run
make run ARGS="check"   # Build and run with args
make tidy               # Tidy and verify modules
make clean              # Remove artifacts
```

### Make Targets

```bash
make docker/build        # Build Docker image
make docker/run          # Build and run CLI in container
make docker/run/serve    # Build and run API server in container
make compose/up          # Start Docker Compose
make compose/down        # Stop Docker Compose
make compose/logs        # Follow logs
```

## License

MIT License — see [LICENSE](LICENSE) file for details.

## Author

[Fabiano Florentino](https://github.com/fabianoflorentino)
