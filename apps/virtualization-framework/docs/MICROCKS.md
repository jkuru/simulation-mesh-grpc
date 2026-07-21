# Virtual backend adapter — teaching mock vs real Microcks

The platform does **not** embed Microcks. It routes simulated third-party traffic
to a **virtual backend** address (`spec.microcksService` / `MICROCKS_SERVICE`).

## Contract

| Concept | Value |
| --- | --- |
| Simulation header | `test-data-simulation-action-name` (`virtualization-contract.SimulationHeader`) |
| Microcks operation header | `x-microcks-operation` |
| Default Service | `microcks-svc.simulation-system.svc.cluster.local:9090` |

App code (fraud-checker → risk) dials the **third-party host**. When the
simulation header is present, Istio VirtualService sends that call to the
virtual backend instead of the real host.

```
fraud-checker  --dial external-risk-api.com-->  mesh
                     │ header present?
                     ├─ yes → microcksService (virtual)
                     └─ no  → real external-risk
```

## Strategies (`spec.virtualBackend`)

| Value | Meaning | Operator behavior |
| --- | --- | --- |
| `teaching-mock` (default) | In-repo `microcks-mock` gRPC RiskService | VS + SE + DR + capture/inject EnvoyFilters |
| `microcks` | Real Microcks (or any backend that reads `x-microcks-operation`) | Same **plus** EnvoyFilter rewrite on `app=microcks` in `simulation-system` |

Mutating webhook fills empty `microcksService` and `virtualBackend`.

## Teaching path (default golden path)

```bash
# example deploys microcks-mock as Deployment/Service "microcks" / "microcks-svc"
cd examples/reference-app-with-framework
make apply-app apply-manifest
```

`SimulationManifest` scenarios document expected responses; **microcks-mock**
implements them in Go (not by importing Microcks).

## Real Microcks path

1. Deploy Microcks (or a compatible mock) in `simulation-system` with label `app=microcks`.  
2. Expose gRPC (or the protocol your services use) on a Service.  
3. Import protos / examples for `EvaluateRisk` (or your RPCs) into Microcks.  
4. Apply a manifest:

```yaml
apiVersion: simulation.io/v1alpha1
kind: SimulationManifest
metadata:
  name: with-real-microcks
  namespace: poc
spec:
  virtualBackend: microcks
  microcksService: microcks-svc.simulation-system.svc.cluster.local:9090
  thirdParties:
    - name: external-risk
      host: external-risk-api.com
      port: 9003
      backendHost: external-risk.poc.svc.cluster.local
      backendPort: 9003
  scenarios:
    - name: fraud-declined
      responses:
        external-risk:
          - operation: EvaluateRisk
            body: '{"risk_score":92,"decision":"DECLINE"}'
```

`scenarios` remain the **platform catalog** (docs + future import helpers).
Real Microcks must be loaded separately with matching example names.

### Optional sample (skeleton)

See `config/samples/microcks-real-backend.yaml` — documents Service shape only;
full Microcks install (DB, UI) is out of scope for this monorepo (see PLATFORM
non-goals). Prefer the official Microcks operator/Helm charts in real clusters.

## Adapter interface (mental model)

```text
type VirtualBackend interface {
  // Address returned as host:port for VirtualService destinations
  HostPort() string
  // Whether operator should emit x-microcks-operation rewrite EnvoyFilter
  NeedsOperationRewrite() bool
}
```

| Implementation | HostPort | Rewrite |
| --- | --- | --- |
| teaching-mock | microcks-svc…:9090 | optional (mock accepts simulation header too) |
| microcks | operator-configured Service | **yes** |

Go constants: `packages/virtualization-contract` (`BackendTeachingMock`, `BackendMicrocks`).
