#!/usr/bin/env bash
# End-to-end: reference-app on Istio (design v1).
#
# Creates a kind cluster (if needed), installs Istio, builds/loads images,
# applies kube teaching manifests, runs the virtualization proof job.
#
# Usage (from apps/reference-app):
#   ./scripts/mesh-e2e.sh
#   CLUSTER=my-cluster ./scripts/mesh-e2e.sh
#   SKIP_CLUSTER_CREATE=1 ./scripts/mesh-e2e.sh   # use existing cluster
#   KEEP_CLUSTER=1 ./scripts/mesh-e2e.sh          # do not delete kind on success
#
# Requires: docker, kind, kubectl. Installs istioctl to a local cache if missing.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

CLUSTER="${CLUSTER:-reference-app-mesh}"
IMAGE_PREFIX="${IMAGE_PREFIX:-reference-app}"
ISTIO_VERSION="${ISTIO_VERSION:-1.24.2}"
ISTIOCTL_CACHE="${ISTIOCTL_CACHE:-$HOME/.cache/reference-app/istio-${ISTIO_VERSION}}"
SKIP_CLUSTER_CREATE="${SKIP_CLUSTER_CREATE:-0}"
KEEP_CLUSTER="${KEEP_CLUSTER:-0}"
SERVICES=(payment-gateway fraud-checker external-risk microcks-mock test-client)

log() { printf '==> %s\n' "$*"; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

need() { command -v "$1" >/dev/null 2>&1 || die "missing required tool: $1"; }

need docker
need kind
need kubectl
docker info >/dev/null 2>&1 || die "docker daemon not running"

ensure_istioctl() {
  if command -v istioctl >/dev/null 2>&1; then
    ISTIOCTL=(istioctl)
    log "using system istioctl: $(istioctl version --remote=false 2>/dev/null | head -1 || true)"
    return
  fi
  local bin="${ISTIOCTL_CACHE}/bin/istioctl"
  if [[ -x "$bin" ]]; then
    ISTIOCTL=("$bin")
    log "using cached istioctl: $bin"
    return
  fi
  log "installing istioctl ${ISTIO_VERSION} into ${ISTIOCTL_CACHE}"
  mkdir -p "$ISTIOCTL_CACHE"
  local os arch url tmp
  # Istio release assets use "osx" not "darwin".
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$os" in
    darwin) os=osx ;;
    linux) os=linux ;;
    *) die "unsupported OS: $os" ;;
  esac
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    *) die "unsupported arch: $arch" ;;
  esac
  # Prefer slim istioctl tarball; fall back to full distro archive.
  url="https://github.com/istio/istio/releases/download/${ISTIO_VERSION}/istioctl-${ISTIO_VERSION}-${os}-${arch}.tar.gz"
  tmp="$(mktemp -d)"
  if ! curl -fsSL "$url" | tar -xz -C "$tmp"; then
    url="https://github.com/istio/istio/releases/download/${ISTIO_VERSION}/istio-${ISTIO_VERSION}-${os}-${arch}.tar.gz"
    curl -fsSL "$url" | tar -xz -C "$tmp"
    mkdir -p "${ISTIOCTL_CACHE}/bin"
    cp "${tmp}/istio-${ISTIO_VERSION}/bin/istioctl" "$bin"
  else
    mkdir -p "${ISTIOCTL_CACHE}/bin"
    # istioctl tarball extracts the binary at top level or in a dir
    if [[ -f "${tmp}/istioctl" ]]; then
      cp "${tmp}/istioctl" "$bin"
    else
      cp "$(find "$tmp" -type f -name istioctl | head -1)" "$bin"
    fi
  fi
  chmod +x "$bin"
  rm -rf "$tmp"
  ISTIOCTL=("$bin")
}

create_cluster() {
  if [[ "$SKIP_CLUSTER_CREATE" == "1" ]]; then
    log "SKIP_CLUSTER_CREATE=1 — using current kube-context: $(kubectl config current-context)"
    return
  fi
  if kind get clusters 2>/dev/null | grep -qx "$CLUSTER"; then
    log "kind cluster ${CLUSTER} already exists"
  else
    log "creating kind cluster ${CLUSTER}"
    kind create cluster --name "$CLUSTER"
  fi
  kubectl cluster-info --context "kind-${CLUSTER}" >/dev/null
  kubectl config use-context "kind-${CLUSTER}" >/dev/null
}

install_istio() {
  ensure_istioctl
  if kubectl get ns istio-system >/dev/null 2>&1 && \
     kubectl -n istio-system get deploy istiod >/dev/null 2>&1; then
    log "Istio already installed (istiod present)"
  else
    log "installing Istio ${ISTIO_VERSION} (demo profile)"
    "${ISTIOCTL[@]}" install --set profile=demo -y
  fi
  log "waiting for istiod"
  kubectl -n istio-system rollout status deploy/istiod --timeout=180s
}

build_and_load_images() {
  log "building service images (IMAGE_PREFIX=${IMAGE_PREFIX})"
  make build-images IMAGE_PREFIX="$IMAGE_PREFIX"
  if [[ "$SKIP_CLUSTER_CREATE" == "1" ]] && ! kubectl config current-context | grep -q kind; then
    log "non-kind context — skipping kind load (ensure images are pullable)"
    return
  fi
  for s in "${SERVICES[@]}"; do
    log "kind load ${IMAGE_PREFIX}/${s}:latest → ${CLUSTER}"
    kind load docker-image "${IMAGE_PREFIX}/${s}:latest" --name "$CLUSTER"
  done
}

deploy_stack() {
  log "applying mesh overlay"
  # EnvoyFilters are best-effort (Lua APIs vary); core proof is VS + mesh mode app.
  if ! kubectl apply -k kube/kustomize/overlays/dev; then
    die "kustomize apply failed"
  fi
  # :latest tag does not change — force pods to pick up newly loaded images.
  log "restarting deployments to pull freshly loaded images"
  kubectl -n poc rollout restart deploy/external-risk deploy/fraud-checker deploy/payment-gateway
  kubectl -n simulation-system rollout restart deploy/microcks
  log "waiting for workloads"
  kubectl -n poc rollout status deploy/external-risk --timeout=180s
  kubectl -n simulation-system rollout status deploy/microcks --timeout=180s
  kubectl -n poc rollout status deploy/fraud-checker --timeout=240s
  kubectl -n poc rollout status deploy/payment-gateway --timeout=240s
  kubectl -n poc get pods -o wide
  kubectl -n simulation-system get pods -o wide
}

run_proof() {
  log "running mesh virtualization proof job"
  kubectl -n poc delete job reference-app-mesh-test --ignore-not-found
  kubectl apply -f kube/kustomize/overlays/dev/test-client-job.yaml
  # Wait for completion
  if ! kubectl -n poc wait --for=condition=complete job/reference-app-mesh-test --timeout=180s; then
    log "job did not complete — dumping logs"
    kubectl -n poc logs job/reference-app-mesh-test || true
    kubectl -n poc describe job/reference-app-mesh-test || true
    kubectl -n poc get pods -l job-name=reference-app-mesh-test -o yaml || true
    die "mesh proof job failed"
  fi
  log "test-client output:"
  kubectl -n poc logs job/reference-app-mesh-test
  # Ensure success message present
  kubectl -n poc logs job/reference-app-mesh-test | grep -q "Virtualization confirmed" \
    || die "proof output missing 'Virtualization confirmed'"
  log "✓ mesh virtualization confirmed"
}

cleanup() {
  if [[ "$KEEP_CLUSTER" == "1" ]] || [[ "$SKIP_CLUSTER_CREATE" == "1" ]]; then
    log "keeping cluster (KEEP_CLUSTER or SKIP_CLUSTER_CREATE set)"
    return
  fi
  log "deleting kind cluster ${CLUSTER}"
  kind delete cluster --name "$CLUSTER" || true
}

main() {
  log "reference-app mesh e2e (cluster=${CLUSTER})"
  create_cluster
  install_istio
  build_and_load_images
  deploy_stack
  run_proof
  log "SUCCESS — reference-app on mesh (v1) e2e passed"
  if [[ "$KEEP_CLUSTER" != "1" && "$SKIP_CLUSTER_CREATE" != "1" ]]; then
    cleanup
  else
    log "cluster retained — re-run test: kubectl apply -f kube/kustomize/overlays/dev/test-client-job.yaml"
  fi
}

main "$@"
