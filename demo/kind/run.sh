#!/usr/bin/env bash
# Runs the Kubernetes demo on a local or CI host:
#   kind + cert-manager + certificate-validate k8s monitor + webhook alerts.
#
# Requires docker, kind, kubectl, helm, and Go. The dev container has all of
# them; on a bare host install them first (CI does this with actions).
set -euo pipefail

REPO_ROOT="${REPO_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
STATE_DIR="${STATE_DIR:-${REPO_ROOT}/.demo-state/kind}"
ARTIFACTS_DIR="${ARTIFACTS_DIR:-}"

export REPO_ROOT
export STATE_DIR
export CLUSTER_NAME="${CLUSTER_NAME:-cert-validate}"
export KUBECONFIG="${KUBECONFIG:-${STATE_DIR}/kubeconfig}"

mkdir -p "${STATE_DIR}"

for command in docker kind kubectl helm go; do
    if ! command -v "${command}" >/dev/null 2>&1; then
        echo "ERROR: ${command} is required for the kind demo" >&2
        exit 1
    fi
done

echo "==> Kubernetes demo (cluster=${CLUSTER_NAME:-cert-validate}, state=${STATE_DIR})"
"${REPO_ROOT}/scripts/setup-cluster.sh"
"${REPO_ROOT}/scripts/alert-demo.sh"

alert_rows=""
if command -v jq >/dev/null 2>&1 && [[ -s /tmp/webhook-alerts.jsonl ]]; then
    alert_rows="$(jq -r '.payload | "| \(.kind) | \(.namespace)/\(.name) | \(.daysLeft) | \(.threshold) |"' /tmp/webhook-alerts.jsonl)"
fi
if [[ -z "${alert_rows}" ]]; then
    alert_rows="| — | See webhook-alerts.jsonl | — | — |"
fi

summary_content="$(cat <<EOF
## Kubernetes demo

| Check | Result | Detail |
| --- | --- | --- |
| Cluster | ✅ | \`${CLUSTER_NAME}\` |
| cert-manager | ✅ | installed via Helm |
| Monitor scan | ✅ | TLS Secret + Ingress discovered |
| Prometheus metrics | ✅ | \`certificate_days_left\` exposed |
| Webhook alert | ✅ | payload delivered |

### Alerts

| Kind | Resource | Days left | Threshold |
| --- | --- | --- | --- |
${alert_rows}
EOF
)"

if [[ -n "${ARTIFACTS_DIR}" ]]; then
    mkdir -p "${ARTIFACTS_DIR}"
    cp /tmp/webhook-alerts.jsonl "${ARTIFACTS_DIR}/webhook-alerts.jsonl" 2>/dev/null || true
    cp /tmp/certificate-validate-monitor.log "${ARTIFACTS_DIR}/monitor.log" 2>/dev/null || true
    printf '%s\n' "${summary_content}" > "${ARTIFACTS_DIR}/summary.md"
fi

printf '\n%s\n' "${summary_content}"
echo
echo "Monitor log:  /tmp/certificate-validate-monitor.log"
echo "Alerts file:  /tmp/webhook-alerts.jsonl"
