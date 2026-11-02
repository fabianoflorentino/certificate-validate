#!/usr/bin/env bash
# End-to-end monitoring and alerting demo for the disposable dev cluster.
#
# Starts a local webhook receiver, runs `k8s monitor` in watch mode against the
# kind cluster with metrics enabled, and prints both the exposed Prometheus
# metrics and the alert payload delivered to the webhook.
#
# Intended to run inside the `dev` container after `setup-cluster.sh`.
set -euo pipefail

REPO_ROOT="${REPO_ROOT:-/workspace}"
STATE_DIR="${STATE_DIR:-/data}"
export KUBECONFIG="${KUBECONFIG:-${STATE_DIR}/kubeconfig}"
CLUSTER_NAME="${CLUSTER_NAME:-cert-validate}"
NAMESPACE="${NAMESPACE:-default}"
CERT_NAME="${CERT_NAME:-cert-validate-demo}"
WEBHOOK_THRESHOLD="${WEBHOOK_THRESHOLD:-365}"
WEBHOOK_PORT="${WEBHOOK_PORT:-9099}"
METRICS_ADDR="${METRICS_ADDR:-127.0.0.1:9102}"
ALERT_FILE="${ALERT_FILE:-/tmp/webhook-alerts.jsonl}"
MONITOR_LOG="${MONITOR_LOG:-/tmp/certificate-validate-monitor.log}"

fail() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

command -v kubectl >/dev/null || fail "kubectl not found; run this inside the dev container"
command -v go >/dev/null || fail "go toolchain not found; run this inside the dev container"

if ! kind get clusters 2>/dev/null | grep -qx "${CLUSTER_NAME}"; then
  fail "kind cluster '${CLUSTER_NAME}' not found; run setup-cluster.sh first"
fi

echo "==> Ensuring demo certificate ${NAMESPACE}/${CERT_NAME} exists"
kubectl apply -f - >/dev/null <<EOF
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: ${CERT_NAME}-selfsigned
  namespace: ${NAMESPACE}
spec:
  selfSigned: {}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: ${CERT_NAME}
  namespace: ${NAMESPACE}
spec:
  secretName: ${CERT_NAME}-tls
  duration: 2160h
  renewBefore: 360h
  privateKey:
    algorithm: RSA
    encoding: PKCS1
    size: 2048
  dnsNames:
    - example.com
  issuerRef:
    name: ${CERT_NAME}-selfsigned
    kind: Issuer
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ${CERT_NAME}
  namespace: ${NAMESPACE}
spec:
  tls:
    - hosts: [example.com]
      secretName: ${CERT_NAME}-tls
  rules:
    - host: example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: ${CERT_NAME}
                port:
                  number: 80
EOF

kubectl wait --for=condition=Ready "certificate/${CERT_NAME}" -n "${NAMESPACE}" --timeout=90s >/dev/null \
  || fail "demo certificate did not become Ready"

echo "==> Building monitor and webhook receiver"
(
  cd "${REPO_ROOT}"
  go build -o /tmp/certificate-validate ./cmd/certificate-validate
  go build -o /tmp/webhook-receiver ./scripts/webhook-receiver
)

rm -f "${ALERT_FILE}" "${MONITOR_LOG}"

/tmp/webhook-receiver -addr "127.0.0.1:${WEBHOOK_PORT}" -out "${ALERT_FILE}" &
receiver_pid=$!
monitor_pid=""

cleanup() {
  if [[ -n "${monitor_pid}" ]]; then
    kill "${monitor_pid}" 2>/dev/null || true
    wait "${monitor_pid}" 2>/dev/null || true
  fi
  kill "${receiver_pid}" 2>/dev/null || true
  wait "${receiver_pid}" 2>/dev/null || true
}
trap cleanup EXIT

echo "==> Running k8s monitor (metrics ${METRICS_ADDR}, webhook http://127.0.0.1:${WEBHOOK_PORT}/alert)"
/tmp/certificate-validate k8s monitor \
  --kubeconfig "${KUBECONFIG}" \
  --namespace "${NAMESPACE}" \
  --watch-interval=5 \
  --metrics-addr "${METRICS_ADDR}" \
  --webhook-url "http://127.0.0.1:${WEBHOOK_PORT}/alert" \
  --webhook-threshold "${WEBHOOK_THRESHOLD}" \
  >"${MONITOR_LOG}" 2>&1 &
monitor_pid=$!

for _ in $(seq 1 30); do
  if [[ -s "${ALERT_FILE}" ]]; then
    break
  fi
  if ! kill -0 "${monitor_pid}" 2>/dev/null; then
    cat "${MONITOR_LOG}" >&2 || true
    fail "monitor exited before sending an alert"
  fi
  sleep 1
done

if [[ ! -s "${ALERT_FILE}" ]]; then
  cat "${MONITOR_LOG}" >&2 || true
  fail "no alert received within 30s"
fi

echo
echo "=== Prometheus metrics ==="
curl -fsS "http://${METRICS_ADDR}/metrics" | grep '^certificate_' \
  || fail "metrics endpoint unavailable"

echo
echo "=== Webhook alerts (${ALERT_FILE}) ==="
cat "${ALERT_FILE}"

echo
echo "Demo complete. Monitor log: ${MONITOR_LOG}"
