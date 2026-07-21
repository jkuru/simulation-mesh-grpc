#!/usr/bin/env bash
# Install virtualization-framework with validating webhook TLS + CA bundle.
# Usage: ./scripts/install.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
NS="${SYSTEM_NAMESPACE:-simulation-system}"
SECRET_NAME="virtualization-framework-webhook-certs"
SERVICE_DNS="virtualization-framework-webhook.${NS}.svc"

need() { command -v "$1" >/dev/null 2>&1 || { echo "error: missing $1" >&2; exit 1; }; }
need kubectl
need openssl

log() { printf '==> %s\n' "$*"; }

CERT_DIR="$(mktemp -d)"
trap 'rm -rf "$CERT_DIR"' EXIT

log "generating webhook TLS cert for ${SERVICE_DNS}"
# SAN for Kubernetes service DNS names (openssl 1.1.1+ / LibreSSL on macOS).
cat >"${CERT_DIR}/openssl.cnf" <<EOF
[req]
distinguished_name = req_distinguished_name
x509_extensions = v3_req
prompt = no
[req_distinguished_name]
CN = ${SERVICE_DNS}
[v3_req]
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names
[alt_names]
DNS.1 = virtualization-framework-webhook
DNS.2 = virtualization-framework-webhook.${NS}
DNS.3 = virtualization-framework-webhook.${NS}.svc
DNS.4 = virtualization-framework-webhook.${NS}.svc.cluster.local
EOF

openssl req -x509 -newkey rsa:2048 -nodes \
  -keyout "${CERT_DIR}/tls.key" \
  -out "${CERT_DIR}/tls.crt" \
  -days 365 \
  -config "${CERT_DIR}/openssl.cnf" \
  -extensions v3_req \
  >/dev/null 2>&1

if base64 --help 2>&1 | grep -q -- '-w'; then
  CA_BUNDLE="$(base64 -w0 <"${CERT_DIR}/tls.crt")"
else
  CA_BUNDLE="$(base64 <"${CERT_DIR}/tls.crt" | tr -d '\n')"
fi

log "ensuring namespace ${NS}"
kubectl create namespace "${NS}" --dry-run=client -o yaml | kubectl apply -f -

log "creating/updating TLS secret ${SECRET_NAME}"
kubectl -n "${NS}" create secret tls "${SECRET_NAME}" \
  --cert="${CERT_DIR}/tls.crt" \
  --key="${CERT_DIR}/tls.key" \
  --dry-run=client -o yaml | kubectl apply -f -

log "applying kustomize (CRD, RBAC matrix, operator, webhook Service)"
kubectl apply -k "${ROOT}/config"

log "applying Mutating + Validating webhook configurations"
# shellcheck disable=SC2016
sed "s|\${CA_BUNDLE}|${CA_BUNDLE}|g" \
  "${ROOT}/config/webhook/mutatingwebhook.yaml" | kubectl apply -f -
# shellcheck disable=SC2016
sed "s|\${CA_BUNDLE}|${CA_BUNDLE}|g" \
  "${ROOT}/config/webhook/validatingwebhook.yaml" | kubectl apply -f -

log "waiting for operator rollout"
kubectl -n "${NS}" rollout status deploy/virtualization-framework --timeout=180s

if [[ -x "${ROOT}/scripts/check-istio-version.sh" ]] || [[ -f "${ROOT}/scripts/check-istio-version.sh" ]]; then
  log "checking Istio support matrix (warn-only)"
  bash "${ROOT}/scripts/check-istio-version.sh" --warn || true
fi

log "✓ virtualization-framework installed (webhooks + metrics + RBAC + NetworkPolicy)"
echo "  Metrics:   kubectl -n ${NS} port-forward deploy/virtualization-framework 8080:8080"
echo "  Events:    kubectl get events -A --field-selector reason=Ready"
echo "  RBAC:      see docs/RBAC.md"
echo "  Support:   see docs/SUPPORT_MATRIX.md / make istio-check"

