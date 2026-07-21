# reference-app

**Toy NFT marketplace application** for the monorepo (option B naming).

> **Demo domain only.** Fictional NFT checkout → fraud → external risk.  
> **Not** banking/card systems. No real financial or card data.

| Mode | Design | How |
| --- | --- | --- |
| **Without service mesh** | **v3** | `make demo`, Docker Compose |
| **With service mesh** | **v1** | `kube/` on Istio, `SIMULATION_MODE=mesh` |

| | |
| --- | --- |
| Module | `github.com/servicemesh/reference-app` |
| Monorepo | [`docs/MONOREPO.md`](../../docs/MONOREPO.md) |
| Status | [`STATUS.md`](./STATUS.md) |

Related projects:

- **virtualization-framework** — `apps/virtualization-framework` (platform product)
- **reference-app-with-framework** — `examples/reference-app-with-framework` (final how-to)

## What it proves

```
Same NFT. Same price. Different outcome — only:
  test-data-simulation-action-name: fraud-declined
```

| Hop | Role | Port |
| --- | --- | --- |
| `test-client` | Two concurrent checkouts | — |
| `checkout-gateway` | Service A (marketplace checkout) | 9001 |
| `fraud-checker` | Service B | 9002 |
| `external-risk` | Third-party stand-in (virtualized) | 9003 |
| `microcks-mock` | Scenario backend | 9090 |

## Quick start (no mesh — v3)

```bash
make demo
make coverage
make up && make test && make down
```

## Mesh (v1)

Full automated path (kind + Istio + proof):

```bash
make mesh-e2e
# keep cluster: KEEP_CLUSTER=1 make mesh-e2e
```

Manual (cluster with Istio already):

```bash
make build-images
# kind load docker-image reference-app/<svc>:latest --name <cluster>
make deploy-dev
make mesh-test
```

Details: [docs/MESH.md](docs/MESH.md).  
App uses `SIMULATION_MODE=mesh`; Istio VirtualService virtualizes `external-risk-api.com`.

## Layout

```
reference-app/
├── cmd/                 composition roots
├── internal/            checkout, fraud, risk, demo, sim
├── proto/ gen/
├── docker-compose.yml   no mesh
├── kube/                mesh teaching manifests
└── simulation/          scenarios + sample manifest
```

## Design docs

- [v3 — without mesh](../../docs/design/v3-poc-reference-app.md)
- [v1 — with mesh](../../docs/design/v1-header-driven-virtualization.md)
- [v2 — virtualization-framework](../../docs/design/v2-simulation-framework-operator.md)
