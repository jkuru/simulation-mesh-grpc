# reference-app on service mesh (design v1)

This is the **mesh mode** of the same application as local `make demo` (design v3).

| Mode | Design | Command |
| --- | --- | --- |
| No mesh | v3 | `make demo` |
| **On mesh** | **v1** | `make mesh-e2e` (or manual steps below) |

## What mesh mode proves

```
App always dials external-risk-api.com:9003  (SIMULATION_MODE=mesh)
                    │
        header absent  → VirtualService → external-risk stand-in → APPROVE
        header present → VirtualService → microcks-mock          → DECLINE
```

Header propagation still uses the Go interceptors (also present for local mode).
EnvoyFilters under `kube/` teach mesh-native capture/inject; VirtualService is
what **virtualizes** the third-party host.

## One-command e2e (kind + Istio)

**Requirements:** Docker, kind, kubectl. The script installs `istioctl` to
`~/.cache/reference-app/` if not on PATH.

```bash
cd apps/reference-app
make mesh-e2e
# keep cluster for debugging:
KEEP_CLUSTER=1 make mesh-e2e
```

This will:

1. Create kind cluster `reference-app-mesh` (unless it exists)
2. Install Istio demo profile
3. `make build-images` + `kind load`
4. `kubectl apply -k kube/kustomize/overlays/dev`
5. Run Job `reference-app-mesh-test` (test-client)
6. Assert “Virtualization confirmed”
7. Delete the kind cluster unless `KEEP_CLUSTER=1`

## Manual steps (existing cluster with Istio)

```bash
make build-images
# kind load docker-image reference-app/<svc>:latest --name <cluster>
make deploy-dev
kubectl apply -f kube/kustomize/overlays/dev/test-client-job.yaml
kubectl -n poc logs -f job/reference-app-mesh-test
```

## Key resources (`kube/kustomize/overlays/dev`)

| Resource | Role |
| --- | --- |
| Deployments payment-gateway, fraud-checker | Real mesh services |
| external-risk | Real third-party stand-in |
| microcks (simulation-system) | Virtual backend |
| ServiceEntry external-risk-api.com | Logical third-party host |
| VirtualService | Header → Microcks, else real |
| DestinationRule | Plaintext gRPC on teaching cluster |
| PeerAuthentication PERMISSIVE | Teaching mTLS policy |
| EnvoyFilters | Teaching header capture/inject |

## App config (mesh)

ConfigMap `poc-config`:

- `SIMULATION_MODE=mesh`
- `EXTERNAL_RISK_ENDPOINT=external-risk-api.com:9003`
- `FRAUD_CHECKER_ENDPOINT=fraud-checker.poc.svc.cluster.local:9002`

## Troubleshooting

| Symptom | Check |
| --- | --- |
| Job fails dial payment-gateway | `kubectl -n poc get pods`; wait for 2/2 sidecars |
| Always APPROVED with header | VS applied? Header on call? `kubectl -n poc get vs` |
| fraud-checker cannot dial `external-risk-api.com` | Istio DNS capture annotation on fraud-checker (see kustomize patch) |
| `exec format error` | Rebuild with `make build-images` (Linux binaries, not macOS) |
| ImagePullBackOff on kind | `kind load docker-image …` then `kubectl rollout restart` |

## Related

- Design: `docs/design/v1-header-driven-virtualization.md`
- Local mode: `make demo` / `README.md`
- Framework (later): `apps/virtualization-framework`
