# STATUS — apps/virtualization-framework

| Field | Value |
| --- | --- |
| **Product name** | virtualization-framework |
| **Design** | v2 |
| **State** | **Complete (platform growth track)** |

## Checklist

- [x] Go module + operator binary  
- [x] `SimulationManifest` CRD  
- [x] Generator + reconciler  
- [x] Status Ready / Error / Forbidden  
- [x] Kustomize install  
- [x] Helm chart skeleton  
- [x] Sample CR  
- [x] **Unit coverage 100%** on `api/` + `internal/` (`make coverage`)  
- [x] **Generator golden files** (`make golden`)  
- [x] Docs: README, STATUS, AGENTS, OPERATOR, RBAC, SUPPORT_MATRIX, MICROCKS  
- [x] Cluster smoke + final example e2e  
- [x] Platform suite: monorepo `make platform-accept` / `platform-e2e`  
- [x] **Validating admission webhook**  
- [x] **Mutating admission webhook** (defaults: `microcksService`, `virtualBackend`)  
- [x] **Prometheus metrics** + **Kubernetes Events**  
- [x] **RBAC matrix** (operator / app-team / platform-admin)  
- [x] **Istio support matrix** + `make istio-check`  
- [x] **Virtual backend adapter** (`teaching-mock` / `microcks` + rewrite EF)  
- [x] **NetworkPolicy** + **Pod Security** labels + restricted-compatible container securityContext  
- [x] **virtualization-contract** dependency for shared constants  

## Coverage note

Gate measures `./api/...` and `./internal/...` at **100%**.  
`cmd/operator` (main) is composition root, excluded like `reference-app` cmd packages.
