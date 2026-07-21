#!/usr/bin/env bash
# Enforce 100% statement coverage on testable packages (api + internal).
# Excludes cmd/operator (composition root / process main).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

THRESHOLD="${COVERAGE_THRESHOLD:-100}"
OUT="${TMPDIR:-/tmp}/vf-coverage.out"

PKGS=(
  ./api/...
  ./internal/...
)

go test "${PKGS[@]}" -covermode=atomic -coverprofile="$OUT"
total="$(go tool cover -func="$OUT" | tail -1 | awk '{print $3}' | tr -d '%')"

echo ""
echo "virtualization-framework coverage (api+internal): ${total}% (threshold ${THRESHOLD}%)"
go tool cover -func="$OUT" | awk '$NF != "100.0%" && $NF != "statements" {print}'

awk -v t="$total" -v m="$THRESHOLD" 'BEGIN {
  if (t+0 < m+0) {
    printf("coverage %.2f%% is below threshold %s%%\n", t+0, m) > "/dev/stderr"
    exit 1
  }
}'

echo "✓ coverage OK"
