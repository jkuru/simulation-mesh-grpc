#!/usr/bin/env bash
# Final e2e: reference-app + virtualization-framework (no hand-written VS/EF).
#
# Prerequisites: docker, kind, kubectl, Istio on cluster (or install via kind).
#
# Usage (from examples/reference-app-with-framework):
#   ./scripts/e2e.sh
#   CLUSTER=servicemesh KEEP_CLUSTER=1 ./scripts/e2e.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MONO="$(cd "$ROOT/../.." && pwd)"
APP="$MONO/apps/reference-app"
FW="$MONO/apps/virtualization-framework"
cd "$ROOT"

CLUSTER="${CLUSTER:-reference-app-fw}"
SKIP_CLUSTER_CREATE="${SKIP_CLUSTER_CREATE:-0}"
KEEP_CLUSTER="${KEEP_CLUSTER:-0}"
IMAGE_PREFIX="${IMAGE_PREFIX:-reference-app}"

log() { printf '==> %s\n' "$*"; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || die "missing: $1"; }

need docker
need kind
need kubectl
docker info >/dev/null 2>&1 || die "docker not running"

ensure_cluster() {
  if [[ "$SKIP_CLUSTER_CREATE" == "1" ]]; then
    log "using current context: $(kubectl config current-context)"
    return
  fi
  if kind get clusters 2>/dev/null | grep -qx "$CLUSTER"; then
    log "kind cluster ${CLUSTER} exists"
  else
    log "creating kind cluster ${CLUSTER}"
    kind create cluster --name "$CLUSTER"
  fi
  kubectl config use-context "kind-${CLUSTER}" >/dev/null
}

ensure_istio() {
  if kubectl get ns istio-system >/dev/null 2>&1 && \
     kubectl -n istio-system get deploy istiod >/dev/null 2>&1; then
    log "Istio present"
    kubectl -n istio-system rollout status deploy/istiod --timeout=180s
    return
  fi
  # Reuse reference-app mesh-e2e istioctl installer path
  log "Istio missing — install with apps/reference-app mesh-e2e first or istioctl install"
  die "Istio not installed (istiod not found)"
}

build_images() {
  log "building reference-app images"
  make -C "$APP" build-images IMAGE_PREFIX="$IMAGE_PREFIX"
  log "building virtualization-framework image"
  make -C "$FW" image
  if kubectl config current-context | grep -q kind; then
    for s in checkout-gateway fraud-checker external-risk microcks-mock test-client; do
      kind load docker-image "${IMAGE_PREFIX}/${s}:latest" --name "$CLUSTER"
    done
    kind load docker-image virtualization-framework:latest --name "$CLUSTER"
  fi
}

install_framework() {
  log "installing virtualization-framework (webhook + RBAC matrix)"
  make -C "$FW" install
  kubectl -n simulation-system rollout restart deploy/virtualization-framework 2>/dev/null || true
  kubectl -n simulation-system rollout status deploy/virtualization-framework --timeout=180s
}

deploy_app() {
  log "deploying reference-app workloads (NO teaching Istio VS/EF)"
  # Assert we are not applying teaching overlay
  if grep -rE 'kind:\s*(VirtualService|ServiceEntry|EnvoyFilter)' kustomize/ >/dev/null 2>&1; then
    die "kustomize/ must not contain VirtualService/ServiceEntry/EnvoyFilter"
  fi
  kubectl apply -k kustomize/
  kubectl -n poc rollout restart deploy/external-risk deploy/fraud-checker deploy/checkout-gateway 2>/dev/null || true
  kubectl -n simulation-system rollout restart deploy/microcks 2>/dev/null || true
  kubectl -n poc rollout status deploy/external-risk --timeout=180s
  kubectl -n simulation-system rollout status deploy/microcks --timeout=180s
  kubectl -n poc rollout status deploy/fraud-checker --timeout=240s
  kubectl -n poc rollout status deploy/checkout-gateway --timeout=240s
}

apply_manifest() {
  log "applying SimulationManifest (framework generates Istio)"
  kubectl apply -f simulation-manifest.yaml
  # Wait for Ready
  for i in $(seq 1 30); do
    phase="$(kubectl get simm reference-app-simulation -n poc -o jsonpath='{.status.phase}' 2>/dev/null || true)"
    if [[ "$phase" == "Ready" ]]; then
      log "SimulationManifest Ready"
      kubectl get simm -n poc
      kubectl get vs,se,dr,envoyfilter -n poc -l app.kubernetes.io/managed-by=virtualization-framework
      return
    fi
    sleep 2
  done
  kubectl get simm -n poc -o yaml || true
  kubectl -n simulation-system logs deploy/virtualization-framework --tail=40 || true
  die "SimulationManifest did not become Ready"
}

run_proof() {
  log "running proof Job"
  kubectl -n poc delete job reference-app-with-framework-test --ignore-not-found
  kubectl apply -f test-client-job.yaml
  kubectl -n poc wait --for=condition=complete job/reference-app-with-framework-test --timeout=180s
  kubectl -n poc logs job/reference-app-with-framework-test
  kubectl -n poc logs job/reference-app-with-framework-test | grep -q "Virtualization confirmed" \
    || die "proof failed"
  log "✓ reference-app-with-framework e2e passed"
}

assert_no_teaching_overlay() {
  # Ensure teaching overlay path was not applied as the source of VS
  # (operator-managed label must be present on VS)
  managed="$(kubectl get vs -n poc -l app.kubernetes.io/managed-by=virtualization-framework -o name 2>/dev/null | wc -l | tr -d ' ')"
  [[ "$managed" -ge 1 ]] || die "expected operator-managed VirtualService"
}

main() {
  log "e2e: reference-app + virtualization-framework"
  ensure_cluster
  ensure_istio
  build_images
  install_framework
  deploy_app
  apply_manifest
  assert_no_teaching_overlay
  run_proof
  if [[ "$KEEP_CLUSTER" != "1" && "$SKIP_CLUSTER_CREATE" != "1" ]]; then
    log "deleting kind cluster ${CLUSTER}"
    kind delete cluster --name "$CLUSTER" || true
  else
    log "keeping cluster"
  fi
}

main "$@"
