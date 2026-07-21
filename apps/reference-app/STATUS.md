# STATUS — apps/reference-app

| Field | Value |
| --- | --- |
| **App name** | `reference-app` |
| **Go module** | `github.com/servicemesh/reference-app` |
| **v3 (no mesh)** | **Complete** |
| **v1 (on mesh)** | **Complete** (manifests + e2e proven on kind/Istio) |

## Modes

| Mode | Design | How | Status |
| --- | --- | --- | --- |
| No mesh | v3 | `make demo` / Compose | **Done** |
| Mesh (manual teaching) | v1 | `kube/` + Istio | **Done** |
| Mesh e2e | v1 | `make mesh-e2e` | **Done** (scripted) |
| Mesh (framework) | final | `examples/reference-app-with-framework` | Not this STATUS |

## Done (no mesh)

- [x] Services + interface-first packages  
- [x] Local demo + Compose  
- [x] Unit tests **100%** on `internal/` (`make coverage`)  

## Done (mesh / v1)

- [x] Hand-written Istio teaching resources in `kube/`  
- [x] ServiceEntry + VirtualService + DestinationRules + PeerAuthentication  
- [x] Startup dial retries for mesh readiness races  
- [x] `scripts/mesh-e2e.sh` (kind + Istio + images + proof Job)  
- [x] `make mesh-e2e` / `make mesh-test`  
- [x] `docs/MESH.md` runbook  
- [x] Istio DNS capture for ServiceEntry host  
- [x] Linux container packaging (`build-linux` + distroless)  
- [x] Live proof: APPROVED + DECLINED via VirtualService on kind cluster `servicemesh`  


## Not this project

- Operator → `apps/virtualization-framework`  
- Final recipe → `examples/reference-app-with-framework`  
