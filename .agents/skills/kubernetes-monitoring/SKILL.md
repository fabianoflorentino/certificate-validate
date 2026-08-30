---
name: kubernetes-monitoring
description: Patterns for Kubernetes manifests and TLS monitoring on this project, including read-only RBAC, DaemonSet agents, Prometheus ServiceMonitor scraping, and cert-manager integration. Use when working on the k8s monitor, kubernetes/monitor manifests, or future K8s phases.
origin: ECC
---

# Kubernetes Monitoring

Guidance for the project's Kubernetes integration (see `docs/K8S_INTEGRATION.md`
for the full phase roadmap). The `k8s monitor` agent runs as a DaemonSet that
scans TLS Secrets and Ingresses and exposes Prometheus metrics, with **read-only**
permissions in Phase 1.

## When to Activate

- Editing manifests under `kubernetes/monitor/`
- Working on the `k8s monitor` CLI or its metrics
- Planning or implementing later K8s phases (auto-renew, alerts, dashboards)
- Securing RBAC and roles for the agent

## Core Principles

### 1. Read-Only by Default

Phase 1 only `get/list/watch`es Secrets and Ingresses. Grant the least privilege
needed; permissions to `update` Secrets, create Events, or touch cert-manager
CRDs are deliberately deferred to Phase 2. Keep RBAC scoped and explicit:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: certificate-monitor
rules:
  - apiGroups: [""]
    resources: ["secrets", "ingresses"]
    verbs: ["get", "list", "watch"]
```

### 2. DaemonSet for Cluster-Wide Scanning

A DaemonSet places the agent on every node, so certificates are scanned
regardless of where resources live. Mount only the kubeconfig/credentials it
needs, and keep the container `imagePullPolicy` and resource requests sensible.

### 3. Prometheus Scraping via ServiceMonitor

Expose metrics on a dedicated port and describe them with a `ServiceMonitor`
(label selector must match the `Service`) so the Prometheus Operator scrapes them:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: certificate-monitor
spec:
  selector:
    matchLabels:
      app: certificate-monitor
  endpoints:
    - port: metrics
      interval: 30s
```

Keep agent metrics on a **dedicated registry** with labels like `namespace`,
`name`, and `kind` to avoid colliding with the core metrics package. Metric names
stay `certificate_`-prefixed (e.g. `certificate_days_left`).

### 4. Metrics Contract

- Expose `certificate_days_left` (days until expiry), `certificate_expired`, and
  `certificate_revoked` consistently.
- Use labels that let operators slice by namespace, name, and resource kind.
- Document any new metric when adding it, and keep README/docs in sync.

## Future Phases

- **Phase 2** will add `update` permissions for auto-renewal via cert-manager
  annotations, post-renewal validation, and Events. Only then may RBAC grow.
- **Phase 3** adds predictive alerts and a Grafana dashboard; plan dashboards
  against the stable metric names above.

## Local Dev

Use the disposable dev environment (`make dev/*`) with kind + cert-manager to
validate manifests end-to-end before applying to a real cluster. The dev compose
profile relies on `network_mode: host` and a `KUBECONFIG` pointing at the host
kind cluster — preserve both when changing the dev setup.

**Remember**: least-privilege RBAC, DaemonSet for coverage, ServiceMonitor-driven
scraping, and a stable, documented metric contract.
