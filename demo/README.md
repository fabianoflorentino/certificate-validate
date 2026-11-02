# Demo Environment

Two deterministic demos exercise the full product without external DNS or
public certificate authorities:

- **Compose demo** — TLS origin, API server, Prometheus, and webhook receiver.
- **Kubernetes demo** — kind + cert-manager + `k8s monitor` + webhook alerts.

Both start from a short-lived **self-signed certificate**. Switching the compose
demo to real certificates only requires dropping the certificate files in
`demo/compose/certs/` and setting `DEMO_HOST`; see
[Using real certificates](#using-real-certificates).

## Compose demo

### Architecture

```
docker compose
├── tls-origin           nginx :443, self-signed or provided certificate
├── certificate-validate API :5000, /metrics, webhook notifier
├── webhook-receiver     local JSONL receiver on :9099
└── prometheus           scrapes certificate-validate:5000/metrics
```

### Quick start

Requirements: Docker with Compose v2, `curl`, and `jq`.

```bash
make demo/up      # build and start the stack
make demo/verify  # run all assertions
make demo/down    # stop and remove volumes
```

`make demo/test` runs start + verify in one command. `make demo/logs` follows
the stack logs.

### What `demo/compose/verify.sh` checks

1. `/health` reports `ok` for the demo host.
2. `/api/v1/cert/info/{host}` returns the expected certificate and `daysLeft`.
3. `/metrics` exposes `certificate_days_left{host="...",port="..."}`.
4. The webhook receiver received an expiring-certificate alert.
5. Prometheus can query `certificate_days_left` for the host.
6. `certificate-validate check -c /tmp/settings.yml` works inside the container.

The verify script writes diagnostics to `ARTIFACTS_DIR` when set; CI uses this
to upload the certificate JSON, metrics, webhook payload, PromQL result, and
verification log.

### Environment variables

Copy `.env.example` to `.env` to override defaults:

| Variable | Default | Meaning |
| --- | --- | --- |
| `DEMO_HOST` | `tls-origin` | Hostname used by the app and certificate |
| `DEMO_PORT` | `443` | TLS port on the origin |
| `TLS_DAYS` | `14` | Lifetime of the generated self-signed certificate |
| `CHECK_TIME` | `5` | Metrics/check interval in seconds |
| `ALERT_THRESHOLD` | `30` | Webhook threshold in days |
| `ALERT_INTERVAL` | `5` | Minimum seconds between repeated alerts |
| `APP_PORT` | `5000` | Host port for the API |
| `TLS_PORT` | `8443` | Host port for the TLS origin |
| `PROMETHEUS_PORT` | `9090` | Host port for Prometheus |

The app renders its config from `demo/compose/config/settings.template.yml` at
startup, so no configuration is edited by hand.

## Using real certificates

The demo maps `DEMO_HOST` to the `tls-origin` container inside the compose
network, so it can validate any DNS name without changing `/etc/hosts`.

1. Put the certificate and key in `demo/compose/certs/`:

   ```text
   demo/compose/certs/server.crt   # full chain, leaf first
   demo/compose/certs/server.key
   ```

2. Set `DEMO_HOST` to a DNS name present in the certificate:

   ```bash
   cp demo/compose/.env.example demo/compose/.env
   sed -i 's/^DEMO_HOST=.*/DEMO_HOST=tls.example.com/' demo/compose/.env
   ```

3. If the certificate is issued by a private CA (or is self-signed), also add
   the CA bundle:

   ```text
   demo/compose/certs/ca.crt
   ```

   The app trusts `ca.crt` automatically. Without `ca.crt`, provided
   certificates use the system root store, which is the correct behavior for
   publicly trusted certificates.

4. Restart the stack:

   ```bash
   make demo/down
   make demo/up
   make demo/verify
   ```

The TLS origin reuses `/certs/server.crt` and `/certs/server.key` when present;
otherwise it generates a self-signed certificate valid for `DEMO_HOST`,
`localhost`, and `127.0.0.1`. Files under `demo/compose/certs/` are gitignored.

To inspect the origin from the host:

```bash
# self-signed default
curl -k https://localhost:8443/

# real certificate for DEMO_HOST
curl --resolve "${DEMO_HOST}:8443:127.0.0.1" "https://${DEMO_HOST}:8443/"
```

## Kubernetes demo

The kind demo reuses the existing dev scripts: it creates a kind cluster,
installs cert-manager, builds and loads the monitor image, deploys the
DaemonSet, then runs `scripts/alert-demo.sh` against a short-lived certificate.

Requirements: Docker, `kind`, `kubectl`, `helm`, and Go. The dev container has
all of them (`make dev/up` then `make dev/shell`).

```bash
make demo/kind
```

This is the same flow CI runs. To destroy the cluster afterwards:

```bash
kind delete cluster --name cert-validate
```

## CI

`.github/workflows/demo.yml` runs:

- **compose-demo** on pull requests and pushes: starts the compose stack, runs
  `demo/compose/verify.sh`, uploads diagnostics, and tears the stack down.
- **kind-demo** on pushes to `main`, pull requests, and manual runs: creates a
  kind cluster, installs cert-manager, runs `demo/kind/run.sh`, collects logs,
  and deletes the cluster.

The demos are deterministic: no Let's Encrypt, no public DNS, and no external
certificate authority is required.
