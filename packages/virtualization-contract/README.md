# virtualization-contract

Shared **constants** for the simulation / virtualization product surface.

| Constant | Value |
| --- | --- |
| `SimulationHeader` | `test-data-simulation-action-name` |
| `MicrocksOperationHeader` | `x-microcks-operation` |
| `PropagationLabelKey` / `Value` | `simulation.io/propagation` = `enabled` |
| `BackendTeachingMock` / `BackendMicrocks` | virtual backend strategies |
| `DefaultMicrocksHostPort` | teaching Microcks Service DNS |

## Consumers

| Project | Usage |
| --- | --- |
| `apps/reference-app` | `internal/sim` aliases `contract.SimulationHeader` |
| `apps/virtualization-framework` | operator config + generator defaults |

```bash
# from monorepo root (go.work)
cd packages/virtualization-contract && go test ./...
```

**Not** here: CRD types (stay in the operator), Istio YAML, business protos.
