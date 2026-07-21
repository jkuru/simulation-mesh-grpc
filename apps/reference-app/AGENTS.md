# AGENTS.md — apps/reference-app

**reference-app** — sample payment application (option B monorepo naming).

| Mode | Design | Notes |
| --- | --- | --- |
| Local / Compose | **v3** | No service mesh; `SIMULATION_MODE=local` |
| Kubernetes + Istio | **v1** | Same binaries; `kube/` + `SIMULATION_MODE=mesh` |

Root: `docs/MONOREPO.md`, root `AGENTS.md`.

## Rules

- Interface-first ports under `internal/`; `cmd/` is wiring only.
- `make coverage` **100%** on `./internal/...` (excludes `cmd/`).
- Keep `make demo` green.
- Do not implement the operator here.
- Teaching mesh YAML: `kube/`. Framework consumers: `examples/reference-app-with-framework`.
