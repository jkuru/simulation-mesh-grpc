#!/usr/bin/env bash
# Check cluster Istio version against the framework support matrix.
# Usage:
#   ./scripts/check-istio-version.sh           # exit 1 if unsupported / missing
#   ./scripts/check-istio-version.sh --warn    # always exit 0 (log warning)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WARN=0
[[ "${1:-}" == "--warn" ]] && WARN=1

log() { printf '==> %s\n' "$*"; }
warn() { printf 'warning: %s\n' "$*" >&2; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

fail() {
  if [[ "$WARN" == "1" ]]; then
    warn "$*"
    exit 0
  fi
  die "$*"
}

need() { command -v "$1" >/dev/null 2>&1 || fail "missing $1"; }
need kubectl

# Claimed matrix (keep in sync with internal/istiosupport + docs/SUPPORT_MATRIX.md)
MIN_MINOR=20
MAX_MINOR=23
VERIFIED="1.22"

image="$(kubectl -n istio-system get deploy istiod -o jsonpath='{.spec.template.spec.containers[0].image}' 2>/dev/null || true)"
if [[ -z "$image" ]]; then
  fail "istiod not found in istio-system (install Istio ${MIN_MINOR}.x–${MAX_MINOR}.x first)"
fi

# Parse major.minor.patch from image tag
ver="$(printf '%s' "$image" | sed -nE 's/.*:v?([0-9]+\.[0-9]+(\.[0-9]+)?).*/\1/p')"
if [[ -z "$ver" ]]; then
  fail "cannot parse Istio version from image: $image"
fi
major="$(printf '%s' "$ver" | cut -d. -f1)"
minor="$(printf '%s' "$ver" | cut -d. -f2)"

log "detected Istio ${ver} (image ${image})"
log "support matrix: 1.${MIN_MINOR} – 1.${MAX_MINOR} (verified e2e: ${VERIFIED})"

if [[ "$major" != "1" ]] || (( minor < MIN_MINOR || minor > MAX_MINOR )); then
  fail "Istio ${ver} is outside supported range 1.${MIN_MINOR}–1.${MAX_MINOR}"
fi

log "✓ Istio ${ver} is within the claimed support matrix"
echo "  See: ${ROOT}/docs/SUPPORT_MATRIX.md"
