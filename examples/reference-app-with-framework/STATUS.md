# STATUS — examples/reference-app-with-framework

| Field | Value |
| --- | --- |
| **Role** | Final: framework drives reference-app on mesh |
| **State** | **Implemented** — **platform acceptance e2e** |

## Checklist

- [x] Kustomize: deploy reference-app services in mesh mode only  
- [x] No VS / SE / EnvoyFilter in `kustomize/`  
- [x] `simulation-manifest.yaml` for external-risk  
- [x] Makefile + `scripts/e2e.sh`  
- [x] Proof Job  
- [x] Labels `simulation.io/propagation=enabled` on A/B  

## How to verify

```bash
CLUSTER=servicemesh KEEP_CLUSTER=1 make e2e
# or stepwise on a live cluster with Istio + images loaded:
make install-framework apply-app apply-manifest test-job
```
