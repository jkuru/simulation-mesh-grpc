# Operator runbook — virtualization-framework

## Purpose

Install once per cluster. Service teams apply a `SimulationManifest` CR; the
operator generates Istio config so third-party gRPC hosts can be virtualized
when request metadata includes:

```text
test-data-simulation-action-name: <scenario>
```

Platform hardening (beyond MVP reconcile):

| Capability | Where |
| --- | --- |
| **Mutating admission** | Defaults `microcksService`, `virtualBackend` |
| **Validating admission** | Rejects bad/prod CRs at `kubectl apply` |
| **Metrics** | Prometheus on `:8080` (`/metrics`) |
| **Events** | `kubectl describe simm` / `kubectl get events` |
| **RBAC matrix** | App team vs operator vs platform admin — [RBAC.md](./RBAC.md) |
| **Support matrix** | Istio versions — [SUPPORT_MATRIX.md](./SUPPORT_MATRIX.md) |
| **Virtual backends** | teaching-mock vs microcks — [MICROCKS.md](./MICROCKS.md) |
| **NetworkPolicy / PSS** | operator NP + namespace baseline + restricted-compatible pod |

## Install

```bash
cd apps/virtualization-framework
make image
kind load docker-image virtualization-framework:latest --name <cluster>
make install   # scripts/install.sh: TLS secret + kustomize + ValidatingWebhookConfiguration
```

Creates:

| Resource | Namespace / scope |
| --- | --- |
| CRD `simulationmanifests.simulation.io` | cluster |
| Deployment `virtualization-framework` | `simulation-system` |
| ServiceAccount + operator ClusterRole/Binding | `simulation-system` / cluster |
| ClusterRoles `simulation-app-team-editor`, `simulation-platform-admin` | cluster |
| Service `virtualization-framework-webhook` | `simulation-system` |
| Secret `virtualization-framework-webhook-certs` | `simulation-system` |
| MutatingWebhookConfiguration `virtualization-framework-mutating` | cluster |
| ValidatingWebhookConfiguration `virtualization-framework-validating` | cluster |
| NetworkPolicy `virtualization-framework` | `simulation-system` |

## Configuration (env on Deployment)

| Variable | Default | Effect |
| --- | --- | --- |
| `ENVIRONMENT` | `dev` | `prod` / `production` → admission deny + status `Forbidden` |
| `MICROCKS_SERVICE` | `microcks-svc.simulation-system.svc.cluster.local:9090` | VirtualService destination |
| `SYSTEM_NAMESPACE` | `simulation-system` | Framework namespace |

Flags: `--webhook-enabled`, `--webhook-cert-dir`, `--metrics-bind-address`, `--health-probe-bind-address`.

## Admission webhooks

| Webhook | Path | Role |
| --- | --- | --- |
| Mutating | `/mutate-simulation-io-v1alpha1-simulationmanifest` | Fill `microcksService`, `virtualBackend=teaching-mock` |
| Validating | `/validate-simulation-io-v1alpha1-simulationmanifest` | Policy + schema semantics |

Operations: `CREATE`, `UPDATE` (`DELETE` always allowed on validating)

Validating denies when:

- Operator `ENVIRONMENT` is prod/production  
- Label/annotation `simulation.io/environment=prod|production`  
- Spec invalid (missing thirdParties/scenarios, bad hosts/ports, duplicate names, response keys not matching third party names, bad `microcksService`)

```bash
# Expect rejection
kubectl apply -f - <<'EOF'
apiVersion: simulation.io/v1alpha1
kind: SimulationManifest
metadata:
  name: bad
  namespace: poc
spec:
  thirdParties: []
  scenarios: []
EOF
```

## Metrics

| Metric | Meaning |
| --- | --- |
| `virtualization_framework_reconcile_total{result=}` | Reconcile outcomes (`success`, `error`, `forbidden`, `deleted`, `requeue`) |
| `virtualization_framework_reconcile_duration_seconds` | Latency histogram |
| `virtualization_framework_phase_transitions_total{phase=}` | Status phase writes |
| `virtualization_framework_generated_objects{namespace,name}` | Objects from last generate |
| `virtualization_framework_admission_denied_total{reason=}` | Reserved for admission instrumentation |

```bash
kubectl -n simulation-system port-forward deploy/virtualization-framework 8080:8080
curl -s localhost:8080/metrics | grep virtualization_framework
```

Pod annotations: `prometheus.io/scrape=true`, port `8080`.

## Events

| Reason | Type | When |
| --- | --- | --- |
| `FinalizerAdded` | Normal | First reconcile |
| `Ready` | Normal | Istio objects applied |
| `ReconcileError` | Warning | Generate/apply failure |
| `ValidationError` | Warning | Spec invalid (defense in depth) |
| `Forbidden` | Warning | Prod environment |
| `Deleting` | Normal | Finalizer cleanup |

```bash
kubectl describe simm -n poc reference-app-simulation
kubectl get events -n poc --field-selector involvedObject.kind=SimulationManifest
```

## Reconcile flow

```
SimulationManifest applied
        │
        ├─ ValidatingWebhook ── deny? → kubectl error (never stored)
        │
        ├─ ENVIRONMENT=prod? → status Forbidden + Event (no objects)
        │
        ├─ deleting? → delete labeled children, remove finalizer
        │
        ├─ no finalizer? → add finalizer, requeue
        │
        ├─ ValidateForAdmission (same rules as webhook)
        │
        └─ generator.Generate → apply SE/VS/DR/EnvoyFilters
                              → status Ready + Event + metrics
```

### Ownership

Generated objects carry:

```yaml
labels:
  app.kubernetes.io/managed-by: virtualization-framework
  simulation.io/manifest: <cr-name>
```

## What the generator creates (per CR)

| Kind | Count | Purpose |
| --- | --- | --- |
| EnvoyFilter | 2 | inbound capture + outbound inject (`simulation.io/propagation=enabled`) |
| ServiceEntry | 1 per third party | Logical third-party host |
| VirtualService | 1 per third party | header → Microcks, else real host |
| DestinationRule | 1 microcks + 1 per third party | plaintext h2 for teaching clusters |

## Sample CR

```bash
make sample
# config/samples/simulationmanifest_reference_app.yaml
kubectl get simm -n poc
kubectl get vs,se,dr,envoyfilter -n poc -l app.kubernetes.io/managed-by=virtualization-framework
```

## Failure modes

| Phase | Meaning |
| --- | --- |
| `Ready` | Resources applied |
| `Error` | Generate/apply/validation failed — see `status.message` + Events |
| `Forbidden` | Prod environment |
| `Pending` | Intermediate (before first success) |

## Prerequisites for virtualization to work

1. Istio installed  
2. Virtual backend (e.g. reference-app `microcks-mock`) listening at `MICROCKS_SERVICE`  
3. Workloads labeled `simulation.io/propagation=enabled`  
4. Apps dial third-party **host** from the CR (mesh mode); ServiceEntry/DNS capture as needed  

End-to-end consumer path: `examples/reference-app-with-framework`.

## Unit tests

```bash
make coverage   # 100% api + internal
```

`cmd/operator` main is process wiring and is not part of the coverage gate (same pattern as reference-app `cmd/*`).
