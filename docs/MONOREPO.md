# Monorepo design — servicemesh

| Field | Value |
| --- | --- |
| **Title** | Virtualization monorepo — names, phases, boundaries |
| **Status** | Active |
| **Naming style** | Option B — formal, self-describing |

Product mechanics live in `docs/design/`. This file is the **map of the repository**.

**Platform golden path (supported surface):** [GOLDEN_PATH.md](./GOLDEN_PATH.md)  
**Platform skills track:** [PLATFORM.md](./PLATFORM.md)  
**Acceptance:** `make platform-accept` (offline) · `make platform-e2e` (cluster)

---

## 0. Golden path vs internals

| Surface | Path | Audience |
| --- | --- | --- |
| **Golden path** | framework + `examples/reference-app-with-framework` | App teams + platform acceptance |
| Internals / teaching | `make demo`, hand `kube/` mesh | Learners of app contract + Istio |
| Theory | `docs/design/v1|v2|v3` | Design RFCs |

Consumers must **not** copy hand-written VS/EnvoyFilter from `apps/reference-app/kube/`.

---

## 1. Easy names (use these everywhere)

| Name | Path | What it is |
| --- | --- | --- |
| **reference-app** | `apps/reference-app` | Sample payment system (the only demo application) |
| **virtualization-framework** | `apps/virtualization-framework` | Installable platform product (operator + CRD) |
| **reference-app-with-framework** | `examples/reference-app-with-framework` | How-to: run the reference app using the framework |
| **virtualization-contract** | `packages/virtualization-contract` | Shared header/label/backend constants |

### How design docs map to names

| Design | Plain English | Path / mode |
| --- | --- | --- |
| **v3** | Reference app **without** service mesh | `apps/reference-app` — `make demo`, Compose |
| **v1** | Reference app **with** service mesh | **Same** app — `apps/reference-app/kube/` + Istio |
| **v2** | **Virtualization framework** | `apps/virtualization-framework` |
| **Final** | Framework applied to meshed reference app | `examples/reference-app-with-framework` |

```
v3  reference-app (local)     ✅
        │
        ▼
v1  reference-app on Istio
        │
        ▼
v2  virtualization-framework
        │
        ▼
final  reference-app-with-framework
```

**There is one application codebase.** Mesh is a deployment mode of `reference-app`, not a second app.

---

## 2. Build sequence

| Phase | Name | Path | State |
| --- | --- | --- | --- |
| 1 | Reference app, no mesh (v3) | `apps/reference-app` local | **Complete** |
| 2 | Reference app on mesh (v1) | `apps/reference-app/kube` + `make mesh-e2e` | **Complete** |
| 3 | Virtualization framework (v2) | `apps/virtualization-framework` | **Complete (hardened + growth)** |
| 4 | Reference app + framework | `examples/reference-app-with-framework` | **Implemented** |

---

## 3. Repository tree

```
servicemesh/
├── AGENTS.md
├── README.md
├── Makefile
├── go.work
│
├── docs/
│   ├── MONOREPO.md
│   ├── SYSTEM_CONTEXT.md
│   └── design/
│       ├── v1-header-driven-virtualization.md   # reference-app on mesh
│       ├── v2-simulation-framework-operator.md  # virtualization-framework
│       └── v3-poc-reference-app.md              # reference-app no mesh
│
├── apps/
│   ├── reference-app/                 # THE sample application
│   │   ├── cmd/ internal/ proto/
│   │   ├── docker-compose.yml         # v3 — no mesh
│   │   ├── kube/                      # v1 — mesh manifests
│   │   └── simulation/
│   │
│   └── virtualization-framework/      # THE platform product
│       ├── config/crd/
│       ├── charts/virtualization-framework/
│       └── STATUS.md
│
├── examples/
│   └── reference-app-with-framework/  # final how-to
│       ├── simulation-manifest.yaml
│       └── kustomize/
│
└── packages/
    └── virtualization-contract/       # optional shared types
```

---

## 4. Project boundaries

### 4.1 `apps/reference-app`

| Mode | Design | How |
| --- | --- | --- |
| No mesh | v3 | `make demo`, Compose, `SIMULATION_MODE=local` |
| Mesh (manual) | v1 | `kube/`, Istio, `SIMULATION_MODE=mesh` |

Owns: payment/fraud/risk demo services, local tests, teaching kube YAML.  
Does not own: operator controller.

### 4.2 `apps/virtualization-framework`

Owns: `SimulationManifest` CRD, operator, Helm chart, prod safety.  
Does not own: payment business logic, local `make demo`.

**Definition of done:** `examples/reference-app-with-framework` works without hand-applying `apps/reference-app/kube` teaching Istio resources.

### 4.3 `examples/reference-app-with-framework`

Owns: install steps, CR, labels, kustomize for reference-app images.  
Does not own: Go services, hand-written VirtualService/EnvoyFilter.

### 4.4 `packages/virtualization-contract`

Shared header/label/backend constants used by `reference-app` and
`virtualization-framework` so the product surface cannot drift.

---

## 5. Go modules

| Path | Module |
| --- | --- |
| `apps/reference-app` | `github.com/servicemesh/reference-app` |
| `apps/virtualization-framework` | `github.com/servicemesh/virtualization-framework` |
| `packages/virtualization-contract` | `github.com/servicemesh/virtualization-contract` |

---

## 6. Old names (do not use)

| Avoid | Use |
| --- | --- |
| `poc`, `sim-framework-poc`, `payment-demo` | `reference-app` |
| `simulation-framework`, `simulation-operator` | `virtualization-framework` |
| `poc-mesh-with-framework`, `poc-with-operator` | `reference-app-with-framework` |
| `simulation-mesh` (separate app) | mesh mode of `reference-app` + v1 design |

---

## 7. Success criteria

| # | Criterion |
| --- | --- |
| S1 (v3) | `make demo` on `reference-app` without Kubernetes — **met** |
| S2 (v1) | `make mesh-e2e` — teaching mesh path — **met** |
| S3 (v2) | Framework install + CR Ready + generator goldens — **met** |
| S4 (final / **platform**) | `make platform-e2e` / `example-e2e` — operator-only Istio — **met** |
| S5 (offline platform) | `make platform-accept` — coverage + goldens + example guard — **met** |

---

## 8. Related docs

| Doc | Role |
| --- | --- |
| [SYSTEM_CONTEXT.md](./SYSTEM_CONTEXT.md) | Topology, header, modes |
| [v3](./design/v3-poc-reference-app.md) | Reference app without mesh |
| [v1](./design/v1-header-driven-virtualization.md) | Reference app on mesh |
| [v2](./design/v2-simulation-framework-operator.md) | Virtualization framework |
