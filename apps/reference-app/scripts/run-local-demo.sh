#!/usr/bin/env bash
# Start all POC services as local processes, run the test client, then tear down.
#
# System context (local mode):
#   test-client → payment-gateway:9001 → fraud-checker:9002
#        no header  → external-risk:9003
#        header set → microcks-mock:9090
#
# Invoked by: make demo
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$ROOT/bin"
LOGDIR="${TMPDIR:-/tmp}/reference-app-logs"
mkdir -p "$LOGDIR"

PIDS=()
cleanup() {
  echo
  echo "==> shutting down services"
  for pid in "${PIDS[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done
  wait 2>/dev/null || true
}
trap cleanup EXIT

start() {
  local name="$1"; shift
  echo "==> starting $name"
  "$@" >"$LOGDIR/$name.log" 2>&1 &
  PIDS+=($!)
}

# Wait until a TCP port accepts connections.
wait_port() {
  local port="$1" tries=40
  while (( tries > 0 )); do
    if nc -z localhost "$port" 2>/dev/null || \
       (echo >/dev/tcp/localhost/"$port") 2>/dev/null; then
      return 0
    fi
    sleep 0.15
    tries=$((tries - 1))
  done
  echo "timeout waiting for port $port" >&2
  echo "--- log ---" >&2
  cat "$LOGDIR"/*.log 2>/dev/null || true
  exit 1
}

start external-risk \
  env GRPC_PORT=9003 \
  "$BIN/external-risk"

start microcks-mock \
  env GRPC_PORT=9090 \
  "$BIN/microcks-mock"

start fraud-checker \
  env GRPC_PORT=9002 \
      SIMULATION_MODE=local \
      EXTERNAL_RISK_ENDPOINT=localhost:9003 \
      MICROCKS_ENDPOINT=localhost:9090 \
  "$BIN/fraud-checker"

wait_port 9003
wait_port 9090
wait_port 9002

start payment-gateway \
  env GRPC_PORT=9001 \
      FRAUD_CHECKER_ENDPOINT=localhost:9002 \
  "$BIN/payment-gateway"

wait_port 9001

echo "==> running test-client"
echo
env PAYMENT_GATEWAY_ENDPOINT=localhost:9001 "$BIN/test-client"
echo
echo "==> service logs: $LOGDIR"
