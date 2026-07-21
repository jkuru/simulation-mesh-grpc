# AGENTS.md — apps/virtualization-framework

Platform **virtualization-framework** (design v2). Monorepo map: `docs/MONOREPO.md`.

## Role

Kubernetes operator: `SimulationManifest` → Istio resources (VS, SE, DR, EnvoyFilter),
with admission, metrics, Events, and persona RBAC.

## Rules

- Business logic for generation lives in `internal/generator` (pure, unit-tested).
- Spec validation + defaults live in `internal/admission` (mutating + validating).
- Shared product constants: `github.com/servicemesh/virtualization-contract`.
- Reconcile lives in `internal/controller` (fake-client unit-tested; Events + metrics).
- Product metrics live in `internal/metrics` (register on controller-runtime registry in main).
- Istio matrix: `internal/istiosupport` + `make istio-check`.
- `cmd/operator` is composition root only (excluded from coverage gate).
- Production safety: `ENVIRONMENT=prod` → admission deny + status `Forbidden`.
- Install via `make install` (TLS + mutating/validating webhooks + NetworkPolicy).
- Do not embed reference-app business services here.
- Prefer unstructured Istio objects over hard Istio client-go deps unless needed.
- App teams get `simulation-app-team-editor`; never bind them to `virtualization-framework-manager`.

## Coverage

```bash
make coverage   # 100% on ./api/... + ./internal/...
```

## After changes

```bash
make test coverage build
# optional cluster:
make image && kind load docker-image virtualization-framework:latest --name <cluster>
make install sample
```

See [docs/OPERATOR.md](docs/OPERATOR.md) · [docs/RBAC.md](docs/RBAC.md).
