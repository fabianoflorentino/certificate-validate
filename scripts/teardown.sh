#!/usr/bin/env bash
# Tears down the kind cluster created by setup-cluster.sh.
# Leaves the Docker host untouched aside from the removed kind nodes.
set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-cert-validate}"
export KUBECONFIG="${KUBECONFIG:-/data/kubeconfig}"

echo "==> Deleting kind cluster '${CLUSTER_NAME}'"
if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
  kind delete cluster --name "${CLUSTER_NAME}"
else
  echo "    cluster not found; nothing to do"
fi

echo "==> Done. The dev container state volume can be removed with:"
echo "    docker compose --profile dev down -v"
