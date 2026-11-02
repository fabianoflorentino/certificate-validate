#!/usr/bin/env bash
# Runs the Kubernetes demo on a local or CI host:
#   kind + cert-manager + certificate-validate k8s monitor + webhook alerts.
#
# Requires docker, kind, kubectl, helm, and Go. The dev container has all of
# them; on a bare host install them first (CI does this with actions).
set -euo pipefail

REPO_ROOT="${REPO_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
STATE_DIR="${STATE_DIR:-${REPO_ROOT}/.demo-state/kind}"

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

echo
echo "Kubernetes demo complete."
echo "Monitor log:  /tmp/certificate-validate-monitor.log"
echo "Alerts file:  /tmp/webhook-alerts.jsonl"
