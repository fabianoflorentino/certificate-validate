# Disposable Kubernetes Development Environment

This environment lets you test the Kubernetes integration (`k8s monitor`) with
**cert-manager** without installing any Kubernetes tooling on your machine
besides Docker.

## Architecture

```
Host machine
└── Docker engine (only host prerequisite)
    └── disposable "dev" container (image versioned in the repo)
        ├── kind (Kubernetes cluster)
        │   └── cert-manager (via Helm)
        │   └── certificate-validate k8s monitor (DaemonSet)
        ├── kubectl
        ├── helm
        └── Go toolchain
```

- The container image contains **all** the tooling (kind, kubectl, helm, Go).
- The dev container uses `network_mode: host` — required because the kind
  kubeconfig points to the API server at `127.0.0.1:<port>` (bound to the host
  loopback), and the container needs host networking to reach it.
- The kind cluster lives in the host's Docker containers, but all configuration
  (`kubeconfig`, cert-manager state) is stored in a disposable **named volume**.
- Nothing is installed on the host besides the Docker engine.

## Prerequisites

- A working Docker engine on the host — **including the `docker compose`
  plugin** (on Arch: `sudo pacman -S docker-compose`).
- Your user must have permission on the Docker socket
  (`sudo usermod -aG docker $USER` + re-login, or run via `newgrp docker`).
- **Working DNS in the Docker daemon.** If the host resolves through a local
  DNS (for example Cloudflare WARP with `127.x`), the daemon may inject
  unreachable nameservers (`8.8.8.8`) into containers and `apk add`/build
  downloads fail with `DNS: transient error`. Disable local DNS tools (WARP)
  **or** restart the daemon (`sudo systemctl restart docker`) so it re-reads the
  host's `/etc/resolv.conf`.

## Usage

From the repository root:

```bash
# 1. Start the dev container (builds the disposable image)
make dev/up

# 2. Open a shell in the container
make dev/shell

# 3. Inside the container: create the kind cluster + cert-manager + deploy the monitor
setup-cluster.sh
```

All Makefile targets:

| Target | Description |
|--------|-------------|
| `make dev/up` | Build and start the dev container |
| `make dev/shell` | Open a shell in the dev container |
| `make dev/setup` | Create kind cluster + cert-manager + deploy the monitor |
| `make dev/scan` | Run a one-off `k8s monitor` scan through the DaemonSet |
| `make dev/alert-demo` | Run the end-to-end metrics + webhook alert demo |
| `make dev/logs` | Stream the monitor logs |
| `make dev/down` | Stop the dev container |
| `make dev/destroy` | Destroy the container and all cluster state |

> For the complete deploy example (dashboard + monitor + ingress/TLS +
> Prometheus/Grafana), run `make demo/kind-full` inside the dev container; it
> reuses this environment's cluster setup and is documented in
> [`demo/README.md`](../demo/README.md#full-kubernetes-example-kind). The script
> falls back to a writable state directory because the repo is mounted
> read-only.

## Useful commands inside the container

```bash
# Check the cluster and cert-manager
kubectl get nodes
kubectl -n cert-manager get pods

# Run a one-off monitor scan
kubectl -n cert-manager exec daemonset/cert-validate-monitor -- \
    certificate-validate k8s monitor

# Stream the monitor logs
monitor-logs.sh
```

## Quick test example

The simplest way to make the monitor find a certificate is to issue one via
**cert-manager** (already installed by `setup-cluster.sh`) and reference it in
an Ingress:

```bash
kubectl apply -f - <<'EOF'
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: selfsigned
  namespace: default
spec:
  selfSigned: {}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: example-tls
  namespace: default
spec:
  secretName: example-tls-secret
  duration: 2160h
  renewBefore: 360h
  privateKey:
    algorithm: RSA
    encoding: PKCS1
    size: 2048
  dnsNames:
    - example.com
  issuerRef:
    name: selfsigned
    kind: Issuer
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: example-ingress
  namespace: default
spec:
  tls:
    - hosts: [example.com]
      secretName: example-tls-secret
  rules:
    - host: example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: example-svc
                port:
                  number: 80
EOF

kubectl wait --for=condition=Ready certificate/example-tls -n default --timeout=60s
```

Then run a scan — the monitor should discover both the TLS Secret **and** the
Ingress:

```bash
make dev/scan
```

> Alternative without cert-manager: `kubectl create secret tls example-tls \
> --cert=/path/to/tls.crt --key=/path/to/tls.key` (requires a real key pair).

## End-to-end monitoring and alerting demo

After `make dev/setup` (and after the quick test above, or with the certificate
created by the demo itself), run:

```bash
make dev/alert-demo
```

The demo starts a local webhook receiver, runs `k8s monitor` in watch mode with
`--metrics-addr` and `--webhook-url`, and prints:

1. the `certificate_*` metrics exposed by the monitor, and
2. the JSON alert payload received by the webhook.

It uses a deliberately high `--webhook-threshold` (365 days by default) so the
90-day demo certificate triggers an alert immediately instead of waiting for
real expiration. Override it with `WEBHOOK_THRESHOLD`, and the ports with
`WEBHOOK_PORT` / `METRICS_ADDR`.

Expected metrics (labels omitted for brevity):

```text
# HELP certificate_days_left Days remaining before certificate expiration
# TYPE certificate_days_left gauge
certificate_days_left{kind="Secret",name="example-tls-secret",namespace="default"} 89
# HELP certificate_expired Whether the certificate has expired (1=expired, 0=valid)
# TYPE certificate_expired gauge
certificate_expired{kind="Secret",name="example-tls-secret",namespace="default"} 0
```

Expected alert payload:

```json
{
  "namespace": "default",
  "name": "example-tls-secret",
  "kind": "Secret",
  "hostname": "example.com",
  "commonName": "example.com",
  "issuer": "cert-manager selfsigned CA",
  "daysLeft": 89,
  "threshold": 365,
  "message": "Certificate default/example-tls-secret (Secret) expires in 89 days"
}
```

## Full cleanup

```bash
make dev/down        # stop the container
make dev/destroy     # remove the container + volumes (zero residue)
```

## Customization

Versions via environment variables (in `setup-cluster.sh`):

- `CLUSTER_NAME` (default `cert-validate`)
- `CERT_MANAGER_VERSION` (default `v1.21.1`)
- `KIND_IMAGE` (default `kindest/node:v1.36.1`)

Base image (`Dockerfile` stage `dev`):

- `KUBECTL_VERSION` (default `v1.36.1`)
- `HELM_VERSION` (default `v3.21.4`)
- `KIND_VERSION` (default `v0.32.0`)

> **Layout:** the dev image is the `dev` target of the **same** `Dockerfile` used
> in production, and the `dev` service lives in the **same** root
> `docker-compose.yml` (profile `dev`, so it does not start by default). The
> scripts (`setup-cluster.sh`, `teardown.sh`, `monitor-logs.sh`, `alert-demo.sh`)
> live in `scripts/` at the repository root and are copied to
> `/usr/local/bin/dev-scripts/` inside the container — that is why
> `setup-cluster.sh` / `monitor-logs.sh` work directly from the container shell.

## Notes

- The monitor manifest in `kubernetes/monitor/` includes read-only discovery
  RBAC plus the permissions required by the Phase 2 auto-renewal path
  (`update` on Secrets and event creation, triggered by the
  `cert-manager.io/force-renew` annotation). Auto-renewal only runs when
  `--renew-threshold` is set; the DaemonSet enables it.
- The `ServiceMonitor` is only applied when the Prometheus Operator is installed
  in the cluster.
