# virtualization-framework

**Platform product (design v2):** Kubernetes operator that turns a `SimulationManifest`
into Istio resources for header-driven third-party virtualization.

| | |
| --- | --- |
| Design | [v2](../../docs/design/v2-simulation-framework-operator.md) |
| Status | [STATUS.md](./STATUS.md) |
| Module | `github.com/servicemesh/virtualization-framework` |
| Runbook | [docs/OPERATOR.md](docs/OPERATOR.md) |
| RBAC | [docs/RBAC.md](docs/RBAC.md) |

## What it does

1. Install once (CRD + operator + **validating webhook** + RBAC matrix).  
2. Teams apply a `SimulationManifest` (admission-checked).  
3. Teams label workloads `simulation.io/propagation=enabled`.  

The operator generates:

- `EnvoyFilter` (inbound capture + outbound inject)  
- `ServiceEntry` + `VirtualService` + `DestinationRule` per third party  
- Status `Ready` / `Forbidden` (prod) / `Error`  
- Kubernetes **Events** + Prometheus **metrics**

Virtual backend host defaults to  
`microcks-svc.simulation-system.svc.cluster.local:9090`  
(deploy Microcks or `reference-app` microcks-mock separately).

## Quick start (local cluster)

```bash
# Build & load operator image (kind example)
make image
kind load docker-image virtualization-framework:latest --name servicemesh

# Install CRD + RBAC matrix + webhook TLS + Deployment
make install

# Apply sample CR (reference-app third party)
# Ensure namespace poc + microcks + external-risk already exist
make sample

kubectl get simm -n poc
kubectl describe simm -n poc   # Events
kubectl get vs,se,dr,envoyfilter -n poc -l app.kubernetes.io/managed-by=virtualization-framework
```

## Developer commands

```bash
make test          # unit tests
make coverage      # 100% gate on api/ + internal/
make build         # host binary → bin/operator
make image         # linux binary + docker image
make install       # scripts/install.sh (webhook certs + apply)
make sample        # sample SimulationManifest
make uninstall
```

Docs: [AGENTS.md](AGENTS.md) · [docs/OPERATOR.md](docs/OPERATOR.md) · [docs/RBAC.md](docs/RBAC.md)  
Platform contract: monorepo [`docs/GOLDEN_PATH.md`](../../docs/GOLDEN_PATH.md)  
Goldens: `make golden` (refresh: `make golden-update`)

## Configuration (operator env)

| Env | Default | Meaning |
| --- | --- | --- |
| `ENVIRONMENT` | `dev` | `prod` → admission deny + status Forbidden |
| `MICROCKS_SERVICE` | `microcks-svc.simulation-system.svc.cluster.local:9090` | VS destination |
| `SYSTEM_NAMESPACE` | `simulation-system` | framework namespace |

## Layout

```
virtualization-framework/
├── api/simulation/v1alpha1/     SimulationManifest types
├── cmd/operator/                main (webhook + metrics + controller)
├── internal/
│   ├── admission/               ValidateSpec + validating webhook
│   ├── config/
│   ├── controller/              reconciler (Events + metrics)
│   ├── events/                  Event reason constants
│   ├── generator/               Istio unstructured objects
│   └── metrics/                 Prometheus collectors
├── config/
│   ├── crd/
│   ├── manager/
│   ├── rbac/                    operator + app-team + platform-admin
│   ├── webhook/                 Service + ValidatingWebhookConfiguration
│   └── samples/
├── scripts/install.sh           TLS + install (prefer over raw kustomize)
└── Makefile
```

## Three-step DX

```bash
# 1) Platform
make install

# 2) Service team (ClusterRole simulation-app-team-editor + RoleBinding)
kubectl apply -f my-simulation.yaml

# 3) Service team
kubectl label deploy payment-gateway fraud-checker \
  simulation.io/propagation=enabled -n poc
```

## Related

- reference-app mesh: `apps/reference-app` (`make mesh-e2e`)  
- Final example: `examples/reference-app-with-framework`  
