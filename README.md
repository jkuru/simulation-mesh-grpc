# servicemesh (monorepo)

Header-driven **service virtualization** for gRPC — a **platform-engineering**
training ground: sample app → mesh literacy → control plane → **golden path**.

One request header — `test-data-simulation-action-name` — selects a scenario.
Real internal services stay real. Only third-party egress is virtualized.

---

## Golden path (what you should use)

**App teams and platform acceptance** use this path only:

1. Install **virtualization-framework**  
2. Deploy workloads (mesh mode) + apply **SimulationManifest**  
3. Label `simulation.io/propagation=enabled`  
4. Send traffic with the simulation header  

```bash
# Offline platform checks (coverage + goldens + example safety)
make platform-accept

# Full cluster golden path (kind + Istio required)
CLUSTER=servicemesh KEEP_CLUSTER=1 make platform-e2e
```

Recipe: [`examples/reference-app-with-framework`](examples/reference-app-with-framework/)  
Contract: [`docs/GOLDEN_PATH.md`](docs/GOLDEN_PATH.md)  
Platform skills: [`docs/PLATFORM.md`](docs/PLATFORM.md)

**Do not** ship hand-written VirtualService / EnvoyFilter YAML from
`apps/reference-app/kube/` to product teams — that tree is for **learning mesh
internals**. The operator generates those objects from the CR.

---

## Projects

| Project | Path | Role |
| --- | --- | --- |
| **reference-app** | [`apps/reference-app`](apps/reference-app/) | Sample payment app (empathy + mesh literacy) |
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
| virtualization-framework MVP | Complete |
| reference-app-with-framework | Complete |

---

## License / classification

Internal engineering / learning. Not a production multi-tenant platform.
