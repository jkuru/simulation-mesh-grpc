# simulation-mesh-grpc

Header-driven **service virtualization** for gRPC on Istio — a
**platform-engineering** training ground: sample app → mesh literacy →
control plane → **golden path**.

| | |
| --- | --- |
| **GitHub repo name** | `simulation-mesh-grpc` |
| **Local folder** | may still be `servicemesh` — rename optional |
| **Go module paths** | currently `github.com/servicemesh/...` (fine for learning; rename later if you want) |

One request header — `test-data-simulation-action-name` — selects a scenario.
Real internal services stay real. Only third-party egress is virtualized.

### Sample domain (toy NFT marketplace)

The demo app is a **fictional NFT marketplace** checkout flow used only to
teach mesh virtualization:

```text
test-client → checkout-gateway → fraud-checker → external-risk
                 (nft_token + price)
```

**Not a banking, card, or payments product.** No real PANs, card networks,
customer data, or production financial systems. Unrelated to any employer.
`fraud-checker` / `external-risk` are generic e‑commerce risk services in the
demo, not card-issuer infrastructure.

---

## Publish to GitHub

Git is already initialized on branch `main`. To put this on GitHub under
**`simulation-mesh-grpc`**:

```bash
# 1) From this directory (already has .git)
cd /path/to/this/repo

# 2) Create the empty GitHub repo (GitHub CLI), then push
gh repo create simulation-mesh-grpc --public --source=. --remote=origin --push

# Or manually:
#   - Create https://github.com/<you>/simulation-mesh-grpc (empty, no README)
#   git remote add origin git@github.com:<you>/simulation-mesh-grpc.git
#   git push -u origin main
```

Suggested GitHub description:

> Header-driven gRPC service virtualization on Istio — platform golden path (CRD, operator, webhooks).

Suggested topics: `istio` `grpc` `kubernetes` `operator` `platform-engineering` `service-mesh` `service-virtualization`

---

## How to start / where to start

### Mental model (30 seconds)

This monorepo is a **platform-engineering gym**, not four unrelated apps:

1. **Feel the problem** → local demo (no mesh)  
2. **Learn the mesh** → hand Istio (optional)  
3. **Use the product** → golden path (framework + CR)  
4. **Operate the platform** → webhooks, RBAC, metrics, support matrix  

| Surface | Meaning |
| --- | --- |
| **Supported product** | Framework + `SimulationManifest` |
| **Learning only** | Hand-written VS / EnvoyFilter under `apps/reference-app/kube/` |

### Pick your goal

| Your goal | Start here |
| --- | --- |
| **Learn as a platform engineer** (recommended) | Steps 1 → 3 below |
| **Just run the golden path** | Jump to Step 3 (needs Docker + kind + Istio) |
| **Understand the sample app only** | Step 1 only |
| **Interview / portfolio story** | Walk 1 → 3 with [`docs/PLATFORM.md`](docs/PLATFORM.md) open |

### Prerequisites

```bash
cd /path/to/simulation-mesh-grpc   # or local folder name

# Eventually you want:
# - Go 1.22+
# - Docker
# - kind, kubectl
# - Istio on a kind cluster (example name: servicemesh)
```

Skim first (5–10 min):

1. This README  
2. [`docs/PLATFORM.md`](docs/PLATFORM.md) — what “platform” means here  
3. [`docs/GOLDEN_PATH.md`](docs/GOLDEN_PATH.md) — what teams are allowed to copy  

---

### Step 1 — Empathy (local, no cluster) ~15 min

Feel header-driven virtualization before any operator:

```bash
make demo
```

What you should notice:

- Same card / amount  
- **Without** the simulation header → real path  
- **With** header `test-data-simulation-action-name: fraud-declined` → virtualized outcome  

More detail: [`apps/reference-app/README.md`](apps/reference-app/README.md)

Optional offline quality gate anytime:

```bash
make platform-accept
```

---

### Step 2 — Data plane literacy (optional) ~30–60 min

Only if you want to **see** Istio objects by hand:

```bash
make mesh-e2e
# or read: apps/reference-app/docs/MESH.md
```

Purpose: understand VirtualService / ServiceEntry / EnvoyFilter so the operator’s
output is not magic. **Do not** ship that YAML as the platform API.

---

### Step 3 — Golden path (the product) ~20–40 min

This is where you start as a **platform user / platform engineer on the product**.

```bash
# Full automated proof (reuses kind cluster "servicemesh", keeps it up)
CLUSTER=servicemesh KEEP_CLUSTER=1 make platform-e2e
```

Or step by step on an existing cluster with Istio:

```bash
# 1) Platform install (CRD, operator, webhooks, RBAC, NetworkPolicy)
cd apps/virtualization-framework
make image
kind load docker-image virtualization-framework:latest --name servicemesh
make install
make istio-check

# 2) App-team recipe
cd ../../examples/reference-app-with-framework
make apply-app
make apply-manifest
make test-job
```

Success looks like:

- `SimulationManifest` → **Ready**  
- Operator-created VS / SE / DR / EnvoyFilter  
- Proof Job: real path APPROVED → header path DECLINED → **Virtualization confirmed**  

Recipe: [`examples/reference-app-with-framework`](examples/reference-app-with-framework/)

---

### Step 4 — Platform engineer skills (after e2e works)

| Skill | Try this |
| --- | --- |
| **Admission** | Apply a bad CR (invalid host) → webhook deny |
| **Mutating defaults** | Apply CR without `microcksService` → defaults filled |
| **Events** | `kubectl describe simm -n poc` |
| **Metrics** | `kubectl -n simulation-system port-forward deploy/virtualization-framework 8080:8080` then `curl -s localhost:8080/metrics \| grep virtualization_framework` |
| **RBAC** | [`apps/virtualization-framework/docs/RBAC.md`](apps/virtualization-framework/docs/RBAC.md) |
| **Support matrix** | `make -C apps/virtualization-framework istio-check` |
| **Code** | `apps/virtualization-framework/internal/{admission,controller,generator}` |

---

### Where to open the IDE

```text
Product / golden path
  docs/GOLDEN_PATH.md
  docs/PLATFORM.md
  examples/reference-app-with-framework/

Control plane (operator)
  apps/virtualization-framework/

Sample app
  apps/reference-app/internal/sim/     # header propagation
  apps/reference-app/cmd/*/main.go     # composition roots only

Shared contract (header names, labels)
  packages/virtualization-contract/
```

---

### 45-minute path (if that is all you have today)

```bash
make demo
make platform-accept
CLUSTER=servicemesh KEEP_CLUSTER=1 make platform-e2e
```

Then explain out loud (from [`docs/GOLDEN_PATH.md`](docs/GOLDEN_PATH.md)):

> App teams apply a CR and a label; the platform generates mesh config;
> one header selects the scenario.

---

### What not to start with

- Writing more microservices  
- Hand-editing EnvoyFilters as “the product”  
- Full multi-cluster Microcks installs  

Those are distractions. The product surface is the **golden path**.

---

## Golden path (supported product surface)

**App teams and platform acceptance** use this path only:

1. Install **virtualization-framework**  
2. Deploy workloads (mesh mode) + apply **SimulationManifest**  
3. Label `simulation.io/propagation=enabled`  
4. Send traffic with the simulation header  

```bash
make platform-accept                                          # offline
CLUSTER=servicemesh KEEP_CLUSTER=1 make platform-e2e          # cluster
```

| Doc | Purpose |
| --- | --- |
| [`docs/GOLDEN_PATH.md`](docs/GOLDEN_PATH.md) | What teams may copy |
| [`docs/PLATFORM.md`](docs/PLATFORM.md) | Platform-engineer track |
| [`examples/reference-app-with-framework`](examples/reference-app-with-framework/) | End-to-end recipe |

**Do not** ship hand-written VirtualService / EnvoyFilter YAML from
`apps/reference-app/kube/` to product teams — that tree is for **learning mesh
internals**. The operator generates those objects from the CR.

---

## Projects

| Project | Path | Role |
| --- | --- | --- |
| **reference-app** | [`apps/reference-app`](apps/reference-app/) | Toy NFT marketplace app (empathy + mesh literacy) |
| **virtualization-framework** | [`apps/virtualization-framework`](apps/virtualization-framework/) | **Platform product** (operator + CRD + webhooks) |
| **reference-app-with-framework** | [`examples/reference-app-with-framework`](examples/reference-app-with-framework/) | **Golden path** consumer recipe |
| **virtualization-contract** | [`packages/virtualization-contract`](packages/virtualization-contract/) | Shared header/label/backend constants |

```
Empathy     make demo                 (local, no mesh)
Internals   make mesh-e2e             (hand Istio — learning only)
Platform    make platform-accept      (offline)
            make platform-e2e         (cluster golden path)  ← supported
```

---

## Layout

```
servicemesh/
├── apps/
│   ├── reference-app/
│   └── virtualization-framework/
├── examples/
│   └── reference-app-with-framework/    # GOLDEN PATH
├── packages/
│   └── virtualization-contract/
├── scripts/
│   └── platform-accept.sh
└── docs/
    ├── GOLDEN_PATH.md                   # what teams copy
    ├── PLATFORM.md                      # platform-engineer track
    ├── MONOREPO.md
    ├── SYSTEM_CONTEXT.md
    └── design/                          # v1, v2, v3 theory
```

---

## Commands (cheat sheet)

| Command | Purpose |
| --- | --- |
| `make platform-accept` | **Offline platform suite** (coverage + goldens + example guard) |
| `make platform-e2e` | **Cluster acceptance** = example e2e (framework + CR + proof) |
| `make demo` | Local app only (no K8s) |
| `make coverage` | reference-app internal/ 100% |
| `make framework-coverage` | framework api+internal 100% |
| `make framework-golden` | generator snapshot tests |
| `make mesh-e2e` | Teaching mesh with hand-written Istio |
| `make status` | Project STATUS headlines |
| `make -C apps/virtualization-framework install` | Install operator + webhooks + RBAC + NetworkPolicy |
| `make -C apps/virtualization-framework istio-check` | Cluster Istio vs support matrix |

---

## Design documents (theory)

| Doc | Topic |
| --- | --- |
| [v3](docs/design/v3-poc-reference-app.md) | reference-app without mesh |
| [v1](docs/design/v1-header-driven-virtualization.md) | reference-app on mesh |
| [v2](docs/design/v2-simulation-framework-operator.md) | virtualization-framework |

---

## Status

| Piece | Status |
| --- | --- |
| Golden path docs | **Current** |
| Generator goldens | **Current** |
| Platform accept (offline) | **Current** — `make platform-accept` |
| Platform e2e (cluster) | **Current** — `make platform-e2e` |
| reference-app local + mesh | Complete |
| virtualization-framework (hardened + growth) | Complete |
| reference-app-with-framework | Complete |
| virtualization-contract | Complete |

---

## License / classification

Internal engineering / learning. Not a production multi-tenant platform.
