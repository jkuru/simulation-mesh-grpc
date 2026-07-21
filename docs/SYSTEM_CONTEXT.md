# System context — servicemesh

Runtime map: problem, topology, modes.  
**Repository names:** [MONOREPO.md](./MONOREPO.md) (option B).

---

## 1. Problem

Third-party gRPC dependencies make integration tests fragile. Environment-specific
stubs break **one binary everywhere**. Header-driven virtualization: one header
selects a scenario; only third-party egress is substituted.

---

## 2. Topology (reference-app services)

```
test-client → checkout-gateway → fraud-checker → external RiskService
(NFT marketplace demo domain — not banking/cards)
                                    │
                    no header  → real external-risk
                    header set → virtual backend (scenario)
```

Same card + amount can APPROVE or DECLINE based only on  
`test-data-simulation-action-name`.

---

## 3. Modes (same `apps/reference-app` binaries)

| Mode | Design | Routing | How |
| --- | --- | --- | --- |
| **No mesh** | **v3** | App dials mock when header set (`SIMULATION_MODE=local`) | `make demo` / Compose |
| **Manual mesh** | **v1** | Istio VS + EnvoyFilters; `SIMULATION_MODE=mesh` | `apps/reference-app/kube` |
| **Framework mesh** | **v2 + final** | Operator generates mesh objects from a CR | `examples/reference-app-with-framework` |

---

## 4. Header and labels

| Name | Value |
| --- | --- |
| Simulation header | `test-data-simulation-action-name` |
| Scenarios | `fraud-approved`, `fraud-declined` |
| Rewrite header | `x-microcks-operation` |
| Propagation label | `simulation.io/propagation=enabled` |

---

## 5. Monorepo map

```
apps/reference-app                      sample application (v3 + v1)
apps/virtualization-framework           platform product (v2)
examples/reference-app-with-framework   final how-to
packages/virtualization-contract        optional shared constants
docs/design/v1|v2|v3
```

---

## 6. Status

| Area | Status |
| --- | --- |
| reference-app no mesh (v3) | **Complete** (`make demo`) |
| reference-app on mesh (v1) | **Complete** (`make mesh-e2e`, `docs/MESH.md`) |
| virtualization-framework (v2) | **MVP** (`make framework-test`, install via kustomize) |
| reference-app-with-framework | **Done** (`make example-e2e`) |

---

## 7. Glossary

| Term | Meaning |
| --- | --- |
| **reference-app** | Sample app in `apps/reference-app` |
| **virtualization-framework** | Operator product in `apps/virtualization-framework` |
| **SimulationManifest** | CR describing third parties + scenarios |
| **No mesh / on mesh** | Modes of reference-app, not separate apps |

---

## 8. Proof

```text
Same card. Same amount. Different outcome — only the header changes.
```

Today: `make demo` (reference-app, no mesh).
