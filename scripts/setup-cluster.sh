#!/usr/bin/env bash
# Sets up a disposable kind cluster with cert-manager and deploys the
# certificate-validate `k8s monitor` agent.
#
# Intended to run inside the `dev` container (see docker-compose.yml),
# with the repository mounted read-only at /workspace.
set -euo pipefail

REPO_ROOT="${REPO_ROOT:-/workspace}"
CLUSTER_NAME="${CLUSTER_NAME:-cert-validate}"
CERT_MANAGER_VERSION="${CERT_MANAGER_VERSION:-v1.21.1}"
KIND_IMAGE="${KIND_IMAGE:-kindest/node:v1.36.1}"

# Keep kubeconfig and monitor state in the persistent /data volume.
export KUBECONFIG="${KUBECONFIG:-/data/kubeconfig}"
mkdir -p /data

echo "==> Ensuring kind cluster '${CLUSTER_NAME}'"
if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
  echo "    cluster already exists"
  # Regenerate the kubeconfig so a fresh/empty /data volume gets a valid one.
  kind get kubeconfig --name "${CLUSTER_NAME}" > "${KUBECONFIG}"
else
  # Expose a port for HTTP(S) test apps if needed. Node port 443 is
  # reserved for the ingress controller; we map 8443 access for tests.
  cat >/tmp/kind-config.yaml <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    kubeadmConfigPatches:
      - |
        kind: InitConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "ingress-ready=true"
    extraPortMappings:
      - containerPort: 80
        hostPort: 8080
        protocol: TCP
      - containerPort: 443
        hostPort: 8443
        protocol: TCP
EOF
  kind create cluster \
    --name "${CLUSTER_NAME}" \
    --image "${KIND_IMAGE}" \
    --config /tmp/kind-config.yaml
fi

echo "==> Waiting for control plane readiness"
kubectl wait --for=condition=Ready node --all --timeout=120s

echo "==> Installing cert-manager ${CERT_MANAGER_VERSION}"
helm repo add jetstack https://charts.jetstack.io --force-update >/dev/null 2>&1 || true
helm upgrade --install cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --version "${CERT_MANAGER_VERSION}" \
  --set crds.enabled=true \
  --wait

echo "==> Building local certificate-validate image and loading into kind"
MONITOR_IMAGE="${MONITOR_IMAGE:-certificate-validate:dev}"
pushd "${REPO_ROOT}" >/dev/null
  # Build the Go binary image from the current source (dev build).
  # The project Dockerfile's default CMD is `check`; the DaemonSet overrides
  # it with `k8s monitor` args.
  docker build -f Dockerfile -t "${MONITOR_IMAGE}" .
popd >/dev/null
kind load docker-image "${MONITOR_IMAGE}" --name "${CLUSTER_NAME}"

echo "==> Deploying certificate-validate k8s monitor"
apply_manifests() {
  local dir="${REPO_ROOT}/kubernetes/monitor"
  local workdir
  workdir="$(mktemp -d)"
  trap 'rm -rf "${workdir}"' RETURN

  # /workspace is read-only, so copy manifests and inject the local image.
  for f in rbac.yml daemonset.yml service.yml servicemonitor.yml; do
    if [[ -f "${dir}/${f}" ]]; then
      cp "${dir}/${f}" "${workdir}/${f}"
    fi
  done
  # Point the DaemonSet image at the one we loaded into kind.
  sed -i "s#image: docker.io/fabianoflorentino/certificate-validate:latest#image: ${MONITOR_IMAGE}#" \
    "${workdir}/daemonset.yml" || true

  # Apply in dependency order: RBAC first, then the workload + service.
  kubectl apply -f "${workdir}/rbac.yml"
  kubectl apply -f "${workdir}/daemonset.yml"
  kubectl apply -f "${workdir}/service.yml"
  # ServiceMonitor is optional (requires Prometheus Operator CRDs).
  if kubectl get crd servicemonitors.monitoring.coreos.com >/dev/null 2>&1; then
    kubectl apply -f "${workdir}/servicemonitor.yml"
  else
    echo "    (skipping ServiceMonitor: Prometheus Operator CRDs not present)"
  fi
}
apply_manifests

echo "==> Waiting for monitor daemon set"
kubectl -n cert-manager rollout status daemonset/cert-validate-monitor --timeout=120s || true

echo "==> Cluster ready"
kubectl -n cert-manager get pods,daemonset 2>/dev/null || true
echo
echo "Run a scan now:"
echo "  kubectl -n cert-manager exec daemonset/cert-validate-monitor -- certificate-validate k8s monitor"
echo "Or tail logs:"
echo "  monitor-logs.sh"
