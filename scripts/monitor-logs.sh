#!/usr/bin/env bash
# Streams the certificate-validate k8s monitor logs.
set -euo pipefail

NAMESPACE="${NAMESPACE:-cert-manager}"
DAEMONSET="${DAEMONSET:-cert-validate-monitor}"

POD="$(kubectl -n "${NAMESPACE}" get pods -l app.kubernetes.io/name=certificate-validate \
  -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"

if [[ -z "${POD}" ]]; then
  echo "No monitor pod found in namespace '${NAMESPACE}'. Is setup-cluster.sh run?" >&2
  exit 1
fi

echo "==> Streaming logs from pod ${POD}"
kubectl -n "${NAMESPACE}" logs -f "${POD}"
