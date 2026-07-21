# Support matrix — virtualization-framework

## Istio

| Claim | Range |
| --- | --- |
| **Supported** (major.minor) | **1.20 – 1.23** inclusive |
| **Verified** in this monorepo (kind e2e) | **1.22.x** (`istio/pilot:1.22.3` observed) |

### APIs used

| API | Version | Usage |
| --- | --- | --- |
| VirtualService | `networking.istio.io/v1beta1` | header match → virtual backend |
| ServiceEntry | `networking.istio.io/v1beta1` | third-party host resolution |
| DestinationRule | `networking.istio.io/v1beta1` | TLS DISABLE + h2 upgrade (teaching) |
| EnvoyFilter | `networking.istio.io/v1alpha3` | Lua capture/inject (+ optional Microcks rewrite) |

### Assumptions

- Sidecar injection available  
- Lua HTTP filter enabled (Istio default)  
- DNS capture recommended for ServiceEntry hosts (see reference-app `docs/MESH.md`)  

### Check a cluster

```bash
cd apps/virtualization-framework
make istio-check
# or warn-only during install:
./scripts/check-istio-version.sh --warn
```

Unit-tested parser: `internal/istiosupport`.

### Outside the matrix

Versions **&lt; 1.20** or **&gt; 1.23** are **unsupported** until re-verified. They may still work; open a matrix update with e2e evidence.

---

## Kubernetes

| Claim | Notes |
| --- | --- |
| Tested | kind + Kubernetes **1.29+** (local e2e used 1.36) |
| CRD | `apiextensions.k8s.io/v1` |
| Admission | `admissionregistration.k8s.io/v1` (mutating + validating) |
| NetworkPolicy | optional enforcement depends on CNI (kindnet supports basic NP) |

---

## Virtual backends

| Backend | `spec.virtualBackend` | Status |
| --- | --- | --- |
| Teaching mock | `teaching-mock` (default) | **Supported** — `reference-app` microcks-mock |
| Real Microcks | `microcks` | **Supported path** — point `microcksService` at Microcks gRPC; operator emits rewrite EnvoyFilter |

See [MICROCKS.md](./MICROCKS.md).
