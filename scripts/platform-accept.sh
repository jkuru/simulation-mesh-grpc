#!/usr/bin/env bash
# Platform acceptance suite (offline) — no cluster required.
#
# Verifies the platform product surface for platform-engineering quality:
#   - reference-app library coverage 100%
#   - framework library coverage 100%
#   - generator golden files
#   - example kustomize has no hand-written Istio routing resources
#
# Cluster golden path (optional): make platform-e2e
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

log() { printf '==> %s\n' "$*"; }

log "platform-accept (offline)"

log "1/4 reference-app unit coverage (100%)"
make -C apps/reference-app coverage

log "2/4 virtualization-framework unit coverage (100%)"
make -C apps/virtualization-framework coverage

log "3/4 generator golden tests"
make -C apps/virtualization-framework golden

log "4/4 example: no hand-written VS/SE/EnvoyFilter"
make -C examples/reference-app-with-framework assert-no-istio-in-kustomize

echo ""
echo "✓ platform-accept PASSED (offline)"
echo "  Cluster golden path: CLUSTER=<kind-name> KEEP_CLUSTER=1 make platform-e2e"
