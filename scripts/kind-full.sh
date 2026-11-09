#!/usr/bin/env bash
# Complete local Kubernetes example on kind:
#   kind + cert-manager + ingress-nginx (+ kube-prometheus-stack) +
#   certificate-validate dashboard (Helm chart) + k8s monitor + webhook receiver.
#
# Reuses scripts/setup-cluster.sh for the cluster, cert-manager, image build, and
# monitor DaemonSet, then layers the dashboard, ingress/TLS, observability, and
# in-cluster webhook receiver on top.
#
# Requirements: docker, kind, kubectl, helm, go, curl.
set -euo pipefail

REPO_ROOT="${REPO_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
if [[ -z "${STATE_DIR:-}" ]]; then
    STATE_DIR="${REPO_ROOT}/.demo-state/kind-full"
    # The dev container mounts the repo read-only; fall back to a writable dir.
    if ! mkdir -p "${STATE_DIR}" 2>/dev/null; then
        STATE_DIR="${TMPDIR:-/tmp}/cv-kind-full"
    fi
fi
CLUSTER_NAME="${CLUSTER_NAME:-cert-validate}"
APP_NAMESPACE="${APP_NAMESPACE:-certificate-validate}"
APP_HOST="${APP_HOST:-certificate-validate.localhost}"
APP_IMAGE="${APP_IMAGE:-certificate-validate:dev}"
WEBHOOK_IMAGE="${WEBHOOK_IMAGE:-certificate-validate-webhook:dev}"
INGRESS_NAMESPACE="${INGRESS_NAMESPACE:-ingress-nginx}"
MONITORING_NAMESPACE="${MONITORING_NAMESPACE:-monitoring}"
WITH_PROMETHEUS="${WITH_PROMETHEUS:-true}"
ARTIFACTS_DIR="${ARTIFACTS_DIR:-}"

export REPO_ROOT STATE_DIR CLUSTER_NAME
export KUBECONFIG="${KUBECONFIG:-${STATE_DIR}/kubeconfig}"

mkdir -p "${STATE_DIR}"

for command in docker kind kubectl helm go curl; do
    if ! command -v "${command}" >/dev/null 2>&1; then
        echo "ERROR: ${command} is required for the full kind example" >&2
        exit 1
    fi
done

echo "==> Full Kubernetes example (cluster=${CLUSTER_NAME}, state=${STATE_DIR})"

# 1. kind cluster + cert-manager + certificate-validate image + monitor DaemonSet.
MONITOR_IMAGE="${APP_IMAGE}" "${REPO_ROOT}/scripts/setup-cluster.sh"

# 2. ingress-nginx, bound to the kind control-plane host ports 80/443.
echo "==> Installing ingress-nginx"
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx --force-update >/dev/null
helm upgrade --install ingress-nginx ingress-nginx/ingress-nginx \
    --namespace "${INGRESS_NAMESPACE}" \
    --create-namespace \
    --set controller.kind=DaemonSet \
    --set-string controller.nodeSelector.ingress-ready=true \
    --set controller.tolerations[0].key=node-role.kubernetes.io/control-plane \
    --set controller.tolerations[0].operator=Exists \
    --set controller.tolerations[0].effect=NoSchedule \
    --set controller.hostPort.enabled=true \
    --set controller.service.type=NodePort \
    --wait --timeout 5m
kubectl -n "${INGRESS_NAMESPACE}" rollout status daemonset/ingress-nginx-controller --timeout=180s

# 3. kube-prometheus-stack (Prometheus Operator CRDs + Prometheus + Grafana).
if [[ "${WITH_PROMETHEUS}" == "true" ]]; then
    echo "==> Installing kube-prometheus-stack"
    helm repo add prometheus-community https://prometheus-community.github.io/helm-charts --force-update >/dev/null
    helm upgrade --install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
        --namespace "${MONITORING_NAMESPACE}" \
        --create-namespace \
        --set alertmanager.enabled=false \
        --set grafana.enabled=true \
        --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false \
        --set prometheus.prometheusSpec.ruleSelectorNilUsesHelmValues=false \
        --wait --timeout 10m
fi

# 4. Build and load the in-cluster webhook receiver image.
echo "==> Building webhook receiver image"
docker build -f "${REPO_ROOT}/demo/kind/webhook-receiver.Dockerfile" -t "${WEBHOOK_IMAGE}" "${REPO_ROOT}"
kind load docker-image "${WEBHOOK_IMAGE}" --name "${CLUSTER_NAME}"

# 5. selfsigned ClusterIssuer shared by the dashboard and the demo certificate.
kubectl apply -f "${REPO_ROOT}/demo/kind/manifests/selfsigned-clusterissuer.yaml"

# 6. Dashboard (API + embedded UI) from the Helm chart.
echo "==> Deploying certificate-validate dashboard"
app_repo="${APP_IMAGE%:*}"
app_tag="${APP_IMAGE##*:}"
helm upgrade --install certificate-validate "${REPO_ROOT}/chart" \
    --namespace "${APP_NAMESPACE}" \
    --create-namespace \
    -f "${REPO_ROOT}/chart/values-kind.yaml" \
    --set image.repository="${app_repo}" \
    --set image.tag="${app_tag}" \
    --set ingress.hosts[0].host="${APP_HOST}" \
    --set ingress.tls[0].hosts[0]="${APP_HOST}" \
    --wait --timeout 5m

# 7. In-cluster webhook receiver and the demo certificate the monitor discovers.
echo "==> Deploying in-cluster webhook receiver and demo certificate"
workdir="$(mktemp -d)"
trap 'rm -rf "${workdir}"' EXIT
sed "s#image: certificate-validate-webhook:dev#image: ${WEBHOOK_IMAGE}#" \
    "${REPO_ROOT}/demo/kind/manifests/webhook-receiver.yaml" >"${workdir}/webhook-receiver.yaml"
kubectl apply -f "${workdir}/webhook-receiver.yaml"
kubectl apply -f "${REPO_ROOT}/demo/kind/manifests/demo-certificate.yaml"

# 8. Point the monitor at the in-cluster receiver and use demo-friendly timing.
echo "==> Configuring the monitor for the example"
kubectl -n cert-manager create secret generic cert-validate-monitor-webhook \
    --from-literal=url="http://cert-validate-webhook.${APP_NAMESPACE}.svc.cluster.local:9099/alert" \
    --dry-run=client -o yaml | kubectl apply -f -
kubectl -n cert-manager patch daemonset cert-validate-monitor \
    --type=strategic \
    --patch-file "${REPO_ROOT}/demo/kind/manifests/monitor-patch.yaml"
kubectl -n cert-manager rollout status daemonset/cert-validate-monitor --timeout=180s

# setup-cluster.sh applies these only when the CRDs already exist, so re-apply
# them now that kube-prometheus-stack (optionally) installed the operator.
if kubectl get crd servicemonitors.monitoring.coreos.com >/dev/null 2>&1; then
    kubectl apply -f "${REPO_ROOT}/kubernetes/monitor/servicemonitor.yml"
fi
if kubectl get crd prometheusrules.monitoring.coreos.com >/dev/null 2>&1; then
    kubectl apply -f "${REPO_ROOT}/kubernetes/monitor/prometheusrule.yml"
fi

# 9. Wait for every workload and certificate.
echo "==> Waiting for workloads"
kubectl -n "${APP_NAMESPACE}" rollout status deploy/certificate-validate --timeout=180s
kubectl -n "${APP_NAMESPACE}" rollout status deploy/cert-validate-webhook --timeout=180s
kubectl -n default wait --for=condition=Ready certificate/cert-validate-demo --timeout=120s

wait_for_secret() {
    local namespace="$1" name="$2"
    for _ in $(seq 1 60); do
        if kubectl -n "${namespace}" get secret "${name}" >/dev/null 2>&1; then
            return 0
        fi
        sleep 2
    done
    echo "ERROR: secret ${namespace}/${name} was not issued" >&2
    return 1
}
wait_for_secret "${APP_NAMESPACE}" certificate-validate-dashboard-tls

wait_for_metric() {
    local url="$1" pattern="$2"
    for _ in $(seq 1 30); do
        if curl -fsSk --resolve "${APP_HOST}:8443:127.0.0.1" "${url}" 2>/dev/null | grep -q "${pattern}"; then
            return 0
        fi
        sleep 2
    done
    echo "ERROR: ${pattern} not found at ${url}" >&2
    return 1
}

# 10. Assertions.
echo "==> Verifying dashboard over HTTPS"
curl -fsSk --resolve "${APP_HOST}:8443:127.0.0.1" "https://${APP_HOST}:8443/health" | grep -qi 'ok'
curl -fsSk --resolve "${APP_HOST}:8443:127.0.0.1" "https://${APP_HOST}:8443/" | grep -qi 'certificate'
wait_for_metric "https://${APP_HOST}:8443/metrics" 'certificate_days_left'

echo "==> Verifying monitor discovery and metrics"
kubectl -n cert-manager exec daemonset/cert-validate-monitor -- \
    certificate-validate k8s monitor | grep -q 'cert-validate-demo-tls'
kubectl -n cert-manager exec daemonset/cert-validate-monitor -- \
    wget -qO- http://127.0.0.1:9102/metrics | grep -q 'certificate_days_left'

echo "==> Verifying the in-cluster webhook receiver"
webhook_ok=false
for _ in $(seq 1 30); do
    if kubectl -n "${APP_NAMESPACE}" exec deploy/cert-validate-webhook -- cat /data/alerts.jsonl 2>/dev/null | grep -q .; then
        webhook_ok=true
        break
    fi
    sleep 2
done
if [[ "${webhook_ok}" != "true" ]]; then
    echo "ERROR: no webhook alert was received" >&2
    kubectl -n cert-manager logs daemonset/cert-validate-monitor --tail=50 >&2 || true
    exit 1
fi

verify_prometheus() {
    local port=19090
    kubectl -n "${MONITORING_NAMESPACE}" port-forward svc/kube-prometheus-stack-prometheus "${port}:9090" \
        >"${STATE_DIR}/prometheus-port-forward.log" 2>&1 &
    local pf_pid=$!
    trap 'kill "${pf_pid}" 2>/dev/null || true' RETURN

    for _ in $(seq 1 30); do
        if curl -fsS "http://127.0.0.1:${port}/-/ready" >/dev/null 2>&1; then
            break
        fi
        sleep 1
    done

    local result
    for _ in $(seq 1 30); do
        result="$(curl -fsS --get --data-urlencode 'query=certificate_days_left' \
            "http://127.0.0.1:${port}/api/v1/query" 2>/dev/null || true)"
        if [[ "${result}" == *'"status":"success"'* && "${result}" == *certificate_days_left* ]]; then
            return 0
        fi
        sleep 2
    done
    echo "ERROR: Prometheus did not scrape certificate_days_left" >&2
    return 1
}

if [[ "${WITH_PROMETHEUS}" == "true" ]]; then
    echo "==> Verifying Prometheus scraping through the ServiceMonitor"
    verify_prometheus
fi

monitor_metrics="$(kubectl -n cert-manager exec daemonset/cert-validate-monitor -- wget -qO- http://127.0.0.1:9102/metrics | grep '^certificate_' | head -n 20)"
webhook_payload="$(kubectl -n "${APP_NAMESPACE}" exec deploy/cert-validate-webhook -- cat /data/alerts.jsonl | tail -n 1)"

summary_content="$(cat <<EOF
## Full Kubernetes example (kind)

| Check | Result | Detail |
| --- | --- | --- |
| Cluster | ✅ | \`${CLUSTER_NAME}\` |
| Dashboard over HTTPS | ✅ | \`https://${APP_HOST}:8443\` via ingress-nginx + cert-manager |
| Prometheus metrics (app) | ✅ | \`/metrics\` served through the Ingress |
| Monitor discovery | ✅ | demo TLS Secret + Ingress |
| Monitor metrics | ✅ | \`certificate_days_left\` on \`:9102\` |
| Webhook alert | ✅ | delivered to the in-cluster receiver |
| kube-prometheus-stack | $([[ "${WITH_PROMETHEUS}" == "true" ]] && echo "✅" || echo "—") | $([[ "${WITH_PROMETHEUS}" == "true" ]] && echo "Prometheus + Grafana; ServiceMonitor scraped" || echo "skipped (WITH_PROMETHEUS=false)") |

### Monitor metrics (sample)

\`\`\`
${monitor_metrics}
\`\`\`

### Last webhook payload

\`\`\`json
${webhook_payload}
\`\`\`
EOF
)"

if [[ -n "${ARTIFACTS_DIR}" ]]; then
    mkdir -p "${ARTIFACTS_DIR}"
    printf '%s\n' "${summary_content}" >"${ARTIFACTS_DIR}/summary.md"
    printf '%s\n' "${monitor_metrics}" >"${ARTIFACTS_DIR}/monitor-metrics.txt"
    printf '%s\n' "${webhook_payload}" >"${ARTIFACTS_DIR}/webhook-alert.json"
    kubectl -n "${APP_NAMESPACE}" get ingress,deploy,svc -o wide >"${ARTIFACTS_DIR}/workloads.txt" 2>&1 || true
    kubectl -n cert-manager get daemonset,po -o wide >"${ARTIFACTS_DIR}/monitor.txt" 2>&1 || true
fi

printf '\n%s\n' "${summary_content}"
echo
echo "Dashboard:  https://${APP_HOST}:8443  (curl -k --resolve ${APP_HOST}:8443:127.0.0.1 ...)"
if [[ "${WITH_PROMETHEUS}" == "true" ]]; then
    echo "Prometheus: kubectl -n ${MONITORING_NAMESPACE} port-forward svc/kube-prometheus-stack-prometheus 9090:9090"
    echo "Grafana:    kubectl -n ${MONITORING_NAMESPACE} port-forward svc/kube-prometheus-stack-grafana 3000:80"
fi
echo "Alerts:     kubectl -n ${APP_NAMESPACE} exec deploy/cert-validate-webhook -- cat /data/alerts.jsonl"
echo "Cleanup:    kind delete cluster --name ${CLUSTER_NAME}"
