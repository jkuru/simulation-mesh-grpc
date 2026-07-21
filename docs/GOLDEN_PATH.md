# Golden path (platform product surface)

This is the **supported way** to use the monorepo as a **platform**.

If you are learning **platform engineering**, treat this document as the product
contract. Everything else is for building empathy or understanding internals.

---

## What app teams are allowed to copy

| Do | Path |
| --- | --- |
| Install the framework once | `apps/virtualization-framework` (`make install`) |
| Deploy their services (mesh mode) | Same pattern as `examples/reference-app-with-framework/kustomize/` |
| Author **one** CR | `SimulationManifest` |
| Label workloads | `simulation.io/propagation=enabled` |
| Send test traffic with header | `test-data-simulation-action-name: <scenario>` |

| Do **not** copy (internals / teaching) |
| --- |
| Hand-written `VirtualService` / `ServiceEntry` / `EnvoyFilter` from `apps/reference-app/kube/overlays/dev` |
| Local `SIMULATION_MODE=local` dial logic as a production pattern |
| Operator implementation details (unless you own the platform) |

---

## Three steps (platform DX)

```bash
# 1) Platform team — once per cluster
cd apps/virtualization-framework
make image && kind load docker-image virtualization-framework:latest --name <cluster>
make install

# 2–3) App team — workloads + CR (see example)
cd examples/reference-app-with-framework
make apply-app
make apply-manifest
make test-job
```

Full automated acceptance (requires Docker, kind, Istio on cluster):

```bash
# from monorepo root
make platform-accept          # offline checks + coverage + goldens
CLUSTER=servicemesh KEEP_CLUSTER=1 make platform-e2e   # full cluster golden path
```

---

## Platform acceptance criteria

The platform is “green” when:

1. **Unit/golden (offline)**  
   - `make framework-coverage` → 100% on framework `api/` + `internal/`  
   - Generator **golden files** match (`make framework-golden`)  
   - Example kustomize contains **no** VS/SE/EnvoyFilter  

2. **Cluster golden path (e2e)**  
   - Framework installed  
   - `SimulationManifest` reaches `Ready`  
   - Istio objects labeled `app.kubernetes.io/managed-by=virtualization-framework`  
   - test-client: APPROVED (no header) + DECLINED (`fraud-declined`)  

That e2e is `examples/reference-app-with-framework` (`make example-e2e`).

---

## How other paths fit (for learning, not for consumers)

| Path | Audience | Command |
| --- | --- | --- |
| Local app only | Empathy / app contract | `make demo` |
| Hand-written mesh | Understand Istio objects | `apps/reference-app` → `make mesh-e2e` + `docs/MESH.md` |
| **Golden path** | **Platform + app teams** | **example + framework** |

```
                    ┌─────────────────────────────┐
                    │  GOLDEN PATH (supported)    │
                    │  framework + CR + labels    │
                    │  examples/reference-app-…   │
                    └─────────────────────────────┘
                              ▲
         learn internals      │
    ┌───────────┐      ┌──────┴──────┐
    │ make demo │      │ mesh-e2e    │
    │ (no mesh) │      │ (hand Istio)│
    └───────────┘      └─────────────┘
```

---

## Ownership (platform vs app team)

| Concern | Owner |
| --- | --- |
| Operator, CRD, install, upgrade story | Platform |
| SimulationManifest content (hosts, scenarios) | App team |
| Workload labels | App team |
| Istio control plane, cluster addons | Platform / SRE |
| reference-app code | Teaching / sample (not a real product service) |

---

## Related docs

| Doc | Role |
| --- | --- |
| [PLATFORM.md](./PLATFORM.md) | Skills map: becoming a platform engineer with this repo |
| [MONOREPO.md](./MONOREPO.md) | Full tree and phases |
| [OPERATOR.md](../apps/virtualization-framework/docs/OPERATOR.md) | Operator runbook |
| Example README | Consumer recipe |
