# Platform engineering with this monorepo

**Goal of this repository as a training ground:** practice the skills of a
**platform engineer**, not only write another microservices demo.

---

## Platform competencies this repo exercises

| Competency | Where it shows up |
| --- | --- |
| Golden path / self-service DX | `examples/reference-app-with-framework`, [GOLDEN_PATH.md](./GOLDEN_PATH.md) |
| Control plane (CRD + reconcile + status) | `apps/virtualization-framework` |
| **Admission webhooks** | Mutating defaults + validating policy on `SimulationManifest` |
| **Metrics + Events** | Prometheus `:8080`, Kubernetes Events on CRs |
| **RBAC matrix** | App team / operator / platform admin — [RBAC.md](../apps/virtualization-framework/docs/RBAC.md) |
| **Support matrix** | Istio 1.20–1.23 — [SUPPORT_MATRIX.md](../apps/virtualization-framework/docs/SUPPORT_MATRIX.md) |
| **Virtual backend adapter** | teaching-mock vs microcks — [MICROCKS.md](../apps/virtualization-framework/docs/MICROCKS.md) |
| **Pod / network hardening** | NetworkPolicy + restricted-compatible securityContext |
| **Shared contract** | `packages/virtualization-contract` |
| Data plane literacy (mesh, headers, DNS) | `apps/reference-app` mesh mode + Istio |
| Guardrails | `ENVIRONMENT=prod` → Forbidden; admission deny |
| Contracts | Header name, label, CR schema + goldens |
| Operability | Install script, status phases, e2e scripts |
| Empathy for app teams | `make demo` — feel the problem before abstracting it |

---

## Recommended learning order (platform track)

1. **Empathy** — `make demo` (local virtualization)  
2. **Data plane** — `docs/MESH.md` + `make mesh-e2e` (optional but valuable)  
3. **Control plane** — read generator + reconciler; `make framework-coverage`  
4. **Product surface** — [GOLDEN_PATH.md](./GOLDEN_PATH.md) + `make platform-accept`  
5. **Acceptance** — `make platform-e2e` when you have kind + Istio  
6. **Hardening** — admission, metrics/Events, RBAC, NetworkPolicy  
7. **Growth** — support matrix, Microcks adapter, mutating defaults  

Do not start by inventing new services. Harden the **platform surface**.

---

## What “done” means for the platform (not the app)

| Signal | Command / artifact |
| --- | --- |
| Library quality | `make framework-coverage` (100% api+internal) |
| Config stability | Generator goldens (`make framework-golden`) |
| Consumer safety | Example has no hand-written VS/EF |
| Install works | `make framework-install` (TLS webhooks + NP + PSS labels) |
| End-to-end DX | `make platform-e2e` / `example-e2e` |
| Admission | Mutating defaults + invalid CR rejected |
| Operability | Metrics + Events |
| Multi-tenant safety | App-team ClusterRole ≠ operator ClusterRole |
| Istio claim | `make -C apps/virtualization-framework istio-check` |
| Contract | `packages/virtualization-contract` used by app + framework |

App unit tests (`make coverage`) matter for the sample app; they are not the
platform’s primary acceptance signal.

---

## Explicit non-goals (for platform maturity)

- Multi-cluster / multi-mesh federation  
- GUI dashboard  
- Replacing Istio  
- Vendoring a full Microcks stack (DB/UI) in this monorepo  
- Encouraging teams to fork teaching EnvoyFilter YAML  

---

## Further growth (optional, beyond this monorepo)

1. CI pipeline running `platform-accept` on every PR  
2. Cert-manager for webhook cert rotation  
3. Multi-tenant quotas / namespace inventory  
4. Official Microcks Helm values tuned for this header contract  

---

## Related

- [GOLDEN_PATH.md](./GOLDEN_PATH.md) — what teams copy  
- [MONOREPO.md](./MONOREPO.md) — layout  
- Framework [OPERATOR.md](../apps/virtualization-framework/docs/OPERATOR.md)  
- Framework [RBAC.md](../apps/virtualization-framework/docs/RBAC.md)  
- Framework [SUPPORT_MATRIX.md](../apps/virtualization-framework/docs/SUPPORT_MATRIX.md)  
- Framework [MICROCKS.md](../apps/virtualization-framework/docs/MICROCKS.md)  
