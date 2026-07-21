# AGENTS.md — simulation-mesh-grpc monorepo

Rules for Grok (and compatible agents) in this repository.

**Public name:** `simulation-mesh-grpc` (GitHub). Local directory may differ.

## Platform golden path (priority)

For **platform engineering** work, the supported surface is:

1. `apps/virtualization-framework`  
2. `examples/reference-app-with-framework`  

Docs: `docs/GOLDEN_PATH.md`, `docs/PLATFORM.md`.

**Acceptance**

- Offline: `make platform-accept`  
- Cluster: `make platform-e2e`  

Do **not** teach app teams to copy `apps/reference-app/kube` Istio YAML.

## Easy names (always use these)

| Name | Path | Meaning |
| --- | --- | --- |
| **reference-app** | `apps/reference-app` | Sample NFT marketplace application (only demo app) |
| **virtualization-framework** | `apps/virtualization-framework` | Operator + CRD product |
| **reference-app-with-framework** | `examples/reference-app-with-framework` | Example: framework drives reference-app on mesh |
| **virtualization-contract** | `packages/virtualization-contract` | Optional shared constants |

### Design docs ↔ names

| Design | Means | Where |
| --- | --- | --- |
| **v3** | reference-app **without** mesh | local / Compose |
| **v1** | reference-app **with** mesh | `apps/reference-app/kube` + Istio |
| **v2** | virtualization-framework | operator product |
| **Final** | reference-app-with-framework | example recipe |

**One app codebase.** Do not invent a second payment service for mesh.

Read: [`docs/MONOREPO.md`](docs/MONOREPO.md), then the design doc for the layer you touch.

## Build sequence

1. reference-app local (v3) — **done** (`make demo`)  
2. reference-app on mesh (v1) — manifests ready; cluster e2e next  
3. virtualization-framework (v2)  
4. reference-app-with-framework (final)  

## Conventions

- One git root at `servicemesh/`.
- `apps/` = products (`reference-app`, `virtualization-framework`).
- `examples/` = recipes only (no cloned services).
- `packages/` = shared libs when needed.
- Each project: `STATUS.md` is checklist truth.
- Keep reference-app green: `make coverage && make demo`.

### Path ownership

| Path | Owns | Does not own |
| --- | --- | --- |
| `apps/reference-app` | Services, local demo, teaching mesh YAML | Operator reconcile |
| `apps/virtualization-framework` | CRD, controller, chart | Business services |
| `examples/reference-app-with-framework` | CR + kustomize + runbook | Hand-written VS/EF, Go services |
| `packages/virtualization-contract` | Shared constants (later) | Business logic |

### Go modules

| Path | Module |
| --- | --- |
| `apps/reference-app` | `github.com/servicemesh/reference-app` |
| `apps/virtualization-framework` | `github.com/servicemesh/virtualization-framework` (planned) |

## reference-app coding standards

- Interface-first ports under `internal/`; `cmd/` composition root only.
- `make coverage` **100%** on reference-app `./internal/...`; `make framework-coverage` **100%** on framework `api/`+`internal/` (exclude `cmd/` mains).
- Simulation header: `apps/reference-app/internal/sim` → `Header`.
- Local routing: `SIMULATION_MODE=local`; mesh mode always dials external host.

See `apps/reference-app/AGENTS.md`.

## Header contract

| Item | Value |
| --- | --- |
| Simulation header | `test-data-simulation-action-name` |
| Microcks rewrite | `x-microcks-operation` |
| Propagation label | `simulation.io/propagation=enabled` |

## Do not

- Virtualize internal service-to-service calls.
- Put production VirtualService routes to Microcks.
- Clone reference-app services into `examples/` or the framework.
- Apply teaching `apps/reference-app/kube` Istio **and** operator-generated resources for the same demo.
- Use deprecated names: `poc`, `sim-framework-poc`, `simulation-framework` (as app dir), `simulation-operator`, `simulation-mesh`, `payment-demo`.

## Commands

```bash
make demo          # reference-app, no mesh
make coverage
make status
```

## When finishing work

1. Green checks for touched package.  
2. Update that package’s `STATUS.md`.  
3. Keep `docs/MONOREPO.md`, root `README.md`, and this file aligned on **option B** names.
