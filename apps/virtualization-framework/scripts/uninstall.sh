#!/usr/bin/env bash
# Uninstall virtualization-framework resources (including webhook).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
NS="${SYSTEM_NAMESPACE:-simulation-system}"

log() { printf '==> %s\n' "$*"; }

log "deleting webhook configurations"
kubectl delete validatingwebhookconfiguration virtualization-framework-validating --ignore-not-found
kubectl delete mutatingwebhookconfiguration virtualization-framework-mutating --ignore-not-found

log "deleting kustomize resources"
kubectl delete -k "${ROOT}/config" --ignore-not-found || true

log "deleting webhook TLS secret"
kubectl -n "${NS}" delete secret virtualization-framework-webhook-certs --ignore-not-found

log "✓ uninstall requested (namespace ${NS} left in place)"
