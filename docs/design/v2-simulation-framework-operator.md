# Technical Design Document

| Field | Value |
| --- | --- |
| **Title** | virtualization-framework — Operator for Header-Driven Virtualization |
| **Version** | 2.0 |
| **Status** | Draft |
| **Plain name** | **virtualization-framework** (platform product) |
| **Monorepo** | `apps/virtualization-framework` |
| **Relates To** | v1 reference-app on mesh; v3 reference-app no mesh; final `examples/reference-app-with-framework` |
| **Classification** | Internal Engineering |

---

## Document series placement

| Design | Plain English | Implementation |
| --- | --- | --- |
| **v3** | **reference-app** without mesh | `apps/reference-app` local / Compose |
| **v1** | **reference-app** with mesh | `apps/reference-app/kube` + Istio (v1 design) |
| **v2 (this doc)** | **virtualization-framework** automates v1 | `apps/virtualization-framework` |
| **Final** | Framework + meshed reference-app | `examples/reference-app-with-framework` |

This document is **not** another application. It is the **installable platform
product**: teams install **virtualization-framework** once; service teams apply
one CR and labels. The operator emits the Istio objects defined in v1 for apps
such as **reference-app**.

See [`docs/MONOREPO.md`](../MONOREPO.md) (option B naming).

---

## Table of Contents

1. [Abstract](#1-abstract)
2. [Background](#2-background)
3. [Goals and Non-Goals](#3-goals-and-non-goals)
4. [Developer Experience — The Three Steps](#4-developer-experience--the-three-steps)
5. [System Architecture](#5-system-architecture)
6. [Custom Resource Definition](#6-custom-resource-definition)
7. [Framework Docker Image](#7-framework-docker-image)
8. [Operator Design](#8-operator-design)
9. [Overlay Deployment Architecture](#9-overlay-deployment-architecture)
10. [Generated Resource Catalogue](#10-generated-resource-catalogue)
11. [Operational Considerations](#11-operational-considerations)
12. [Security Considerations](#12-security-considerations)
13. [Failure Modes and Mitigations](#13-failure-modes-and-mitigations)
14. [Decision Record](#14-decision-record)
15. [Open Questions](#15-open-questions)
16. [Appendix](#16-appendix)

---

## 1. Abstract

This document describes **virtualization-framework** — a Kubernetes operator
packaged as a Docker image that implements the header-driven service
virtualization design defined in **v1** (reference-app on service mesh). The
framework abstracts mesh complexity behind a three-step developer experience:
install the framework once, declare a `SimulationManifest` custom resource, and
label the participating services.

The operator watches for `SimulationManifest` resources and automatically
generates all Istio `VirtualService`, `EnvoyFilter`, `DestinationRule`, and
`ServiceEntry` resources described in v1. It manages the lifecycle of a Microcks
virtual gRPC server, loads proto schemas and scenario examples, and enforces
production safety constraints automatically.

Developers interact with one YAML file. The operator handles the rest.

**Acceptance for this monorepo:** virtualization-framework is complete when
`examples/reference-app-with-framework` can virtualize **reference-app** on a
cluster **without** hand-applying `apps/reference-app/kube` teaching Istio YAML.

---

## 2. Background

The v1.0 design document defined a complete technical architecture for header-driven service virtualization. It requires the creation and maintenance of the following resources per environment:

- 2 `EnvoyFilter` resources per mesh service (inbound capture, outbound inject)
- 1 `EnvoyFilter` at the Microcks sidecar (scenario header rewrite)
- 1 `VirtualService` per third-party service
- 1 `DestinationRule` for Microcks
- 1 Deployment + Service for Microcks
- ConfigMap resources for proto files and scenario examples
- 1 `EnvoyFilter` at the production ingress gateway

For a system with five third-party services and four mesh services participating in the call chain, this is approximately 25 Kubernetes resources to author, version, and maintain. Changes to any third-party service or scenario require coordinated updates across multiple resources. This is not an acceptable operational model for the teams consuming the framework.

The virtualization-framework reduces this to three steps and one YAML file per team.

---

## 3. Goals and Non-Goals

### Goals

- Package the entire v1.0 implementation as a single installable Kubernetes operator distributed as a Docker image.
- Define a `SimulationManifest` CRD that is the only resource a developer needs to author.
- Provide Kustomize overlays for environment-specific deployment that work without modification for standard environments.
- Automatically generate, apply, and reconcile all Istio and Microcks resources from the `SimulationManifest`.
- Enforce production safety: the framework structurally prevents virtual routing from being activated in a production cluster.
- Provide a status API on the `SimulationManifest` resource so developers can see readiness without reading operator logs.

### Non-Goals

- Replacing the v1.0 design. This document describes the packaging and abstraction layer. The underlying implementation is unchanged.
- Support for HTTP/1.1 services. Out of scope in this version.
- Multi-cluster federation. The framework operates within a single cluster.
- A GUI or web dashboard. Operational visibility is provided through standard Kubernetes status fields and metrics.

---

## 4. Developer Experience — The Three Steps

This section defines the target developer experience. All subsequent sections describe how the framework delivers it.

### Step 1 — Install the framework

Run once per cluster and environment. Performed by the platform team, not service teams.

```bash
helm install virtualization-framework \
  oci://internal-registry/simulation/virtualization-framework:1.0.0 \
  --namespace simulation-system \
  --create-namespace \
  --values values-dev.yaml
```

After installation the cluster has:

- The `SimulationManifest` CRD registered
- The operator running in `simulation-system` namespace
- A Microcks instance running in `simulation-system` namespace
- RBAC configured for the operator to manage Istio and simulation resources
- Production ingress strip filter applied (if `environment: prod` is set in values)

### Step 2 — Declare a SimulationManifest

The developer creates one YAML file describing their third-party services and the scenarios they need.

```yaml
apiVersion: simulation.io/v1alpha1
kind: SimulationManifest
metadata:
  name: payment-simulation
  namespace: payments-namespace
spec:
  thirdParties:
    - name: fraud-service
      host: api.fraud-service.com
      port: 443
      proto: |
        syntax = "proto3";
        package fraud.v1;
        service FraudEvaluator {
          rpc Evaluate (EvaluateRequest) returns (EvaluateResponse);
        }
        message EvaluateRequest  { string card_token = 1; int64 amount = 2; }
        message EvaluateResponse { int32 risk_score = 1; string decision = 2; }

    - name: card-network
      host: api.card-network.com
      port: 443
      proto: |
        syntax = "proto3";
        package card.v1;
        service CardAuthorization {
          rpc Authorize (AuthRequest) returns (AuthResponse);
        }
        message AuthRequest  { string pan_token = 1; int64 amount = 2; }
        message AuthResponse { string auth_code = 1; string status = 2; }

  scenarios:
    - name: payment-declined
      responses:
        fraud-service:
          - operation: Evaluate
            body: '{ "risk_score": 95, "decision": "DECLINE" }'
        card-network:
          - operation: Authorize
            body: '{ "auth_code": "", "status": "DECLINED" }'

    - name: valid-transaction
      responses:
        fraud-service:
          - operation: Evaluate
            body: '{ "risk_score": 5, "decision": "APPROVE" }'
        card-network:
          - operation: Authorize
            body: '{ "auth_code": "A12345", "status": "APPROVED" }'
```

Apply it:

```bash
kubectl apply -f payment-simulation.yaml
```

The operator detects the new resource and generates all required Istio and Microcks configuration within seconds.

### Step 3 — Label participating services

Add one label to each service that must propagate the simulation header through the call chain.

```bash
kubectl label deployment service-a \
  simulation.io/propagation=enabled \
  -n payments-namespace

kubectl label deployment service-b \
  simulation.io/propagation=enabled \
  -n payments-namespace

kubectl label deployment service-c \
  simulation.io/propagation=enabled \
  -n payments-namespace
```

Wait for the `SimulationManifest` to reach `Ready` status:

```bash
kubectl get simulationmanifest payment-simulation -n payments-namespace
```

```
NAME                 STATUS   THIRD-PARTIES   SCENARIOS   AGE
payment-simulation   Ready    2               2           43s
```

The system is operational. Test callers can now send:

```
gRPC metadata: test-data-simulation-action-name: payment-declined
```

---

## 5. System Architecture

### 5.1 Component overview

```
simulation-system namespace
┌─────────────────────────────────────────────────────────────┐
│                   Simulation Operator                       │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ Reconciler   │  │ Generator    │  │ Microcks Client  │   │
│  │ (watches CRD)│  │ (Istio YAML) │  │ (Admin API)      │   │
│  └──────┬───────┘  └──────┬───────┘  └────────┬─────────┘   │
│         │                 │                   │             │
│         ▼                 ▼                   ▼             │
│  Kubernetes API     Istio API              Microcks         │
│  (CRD, ConfigMap)   (VS, EF, DR)        (gRPC backend)      │
└─────────────────────────────────────────────────────────────┘

payments-namespace
┌─────────────────────────────────────────────────────────────┐
│  SimulationManifest → (watched by operator)                 │
│                                                             │
│  Service A [label: simulation.io/propagation=enabled]       │
│  Service B [label: simulation.io/propagation=enabled]       │
│  Service C [label: simulation.io/propagation=enabled]       │
│                                                             │
│  ── generated by operator ──                                │
│  EnvoyFilter:   inbound-capture                             │
│  EnvoyFilter:   outbound-inject                             │
│  VirtualService: fraud-service-simulation                   │
│  VirtualService: card-network-simulation                    │
│  DestinationRule: microcks                                  │
└─────────────────────────────────────────────────────────────┘
```

### 5.2 Data flow

```
Developer                    Kubernetes API              Operator
    │                              │                         │
    │  kubectl apply simulation.yaml                         │
    │─────────────────────────────►│                         │
    │                              │  SimulationManifest     │
    │                              │  created event          │
    │                              │────────────────────────►│
    │                              │                         │ reconcile()
    │                              │                         │  - validate spec
    │                              │                         │  - generate EnvoyFilters
    │                              │                         │  - generate VirtualServices
    │                              │                         │  - generate DestinationRule
    │                              │                         │  - load protos → Microcks
    │                              │                         │  - load scenarios → Microcks
    │                              │                         │  - update status → Ready
    │  kubectl get simulationmanifest                        │
    │─────────────────────────────►│◄────────────────────────│
    │  STATUS: Ready               │                         │
```

---

## 6. Custom Resource Definition

### 6.1 Full specification

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: simulationmanifests.simulation.io
spec:
  group: simulation.io
  names:
    kind: SimulationManifest
    listKind: SimulationManifestList
    plural: simulationmanifests
    singular: simulationmanifest
    shortNames:
      - simm
  scope: Namespaced
  versions:
    - name: v1alpha1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              required: [thirdParties, scenarios]
              properties:
                thirdParties:
                  type: array
                  minItems: 1
                  items:
                    type: object
                    required: [name, host, port]
                    properties:
                      name:
                        type: string
                      host:
                        type: string
                      port:
                        type: integer
                      proto:
                        type: string
                        description: Inline protobuf definition
                      protoConfigMap:
                        type: string
                        description: Optional ConfigMap name holding proto files
                scenarios:
                  type: array
                  minItems: 1
                  items:
                    type: object
                    required: [name, responses]
                    properties:
                      name:
                        type: string
                      responses:
                        type: object
                        additionalProperties:
                          type: array
                          items:
                            type: object
                            required: [operation, body]
                            properties:
                              operation:
                                type: string
                              body:
                                type: string
                              delayMs:
                                type: integer
                                default: 0
                              grpcStatus:
                                type: string
                                default: OK
            status:
              type: object
              properties:
                phase:
                  type: string
                  enum: [Pending, Configuring, Ready, Degraded, Error, Forbidden]
                message:
                  type: string
                thirdPartyStatuses:
                  type: array
                  items:
                    type: object
                    properties:
                      name:
                        type: string
                      virtualServiceReady:
                        type: boolean
                      microcksLoaded:
                        type: boolean
                scenarioCount:
                  type: integer
                thirdPartyCount:
                  type: integer
                lastReconciled:
                  type: string
                  format: date-time
      additionalPrinterColumns:
        - name: Status
          type: string
          jsonPath: .status.phase
        - name: Third-Parties
          type: integer
          jsonPath: .status.thirdPartyCount
        - name: Scenarios
          type: integer
          jsonPath: .status.scenarioCount
        - name: Age
          type: date
          jsonPath: .metadata.creationTimestamp
      subresources:
        status: {}
```

---

## 7. Framework Docker Image

### 7.1 Image contents

```
virtualization-framework:1.0.0
├── /operator
│   └── virtualization-framework          # compiled Go binary
├── /templates                       # embedded Istio resource templates
│   ├── envoyfilter-inbound.yaml.tmpl
│   ├── envoyfilter-outbound.yaml.tmpl
│   ├── envoyfilter-microcks-rewrite.yaml.tmpl
│   ├── envoyfilter-ingress-strip.yaml.tmpl
│   ├── virtual-service.yaml.tmpl
│   ├── destination-rule.yaml.tmpl
│   └── service-entry.yaml.tmpl
├── /manifests                       # static resources applied at startup
│   ├── crds/
│   │   └── simulationmanifests.yaml
│   └── rbac/
│       ├── cluster-role.yaml
│       ├── cluster-role-binding.yaml
│       └── service-account.yaml
└── /healthz
```

### 7.2 Dockerfile

```dockerfile
FROM golang:1.22-alpine AS builder

WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download

COPY cmd/      cmd/
COPY internal/ internal/
COPY templates/ templates/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" \
    -o /workspace/virtualization-framework \
    ./cmd/operator

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /workspace/virtualization-framework /operator/virtualization-framework
COPY templates/ /templates/
COPY manifests/ /manifests/
USER nonroot:nonroot
ENTRYPOINT ["/operator/virtualization-framework"]
```

### 7.3 Operator binary structure

```
cmd/
  operator/
    main.go

internal/
  controller/
    simulationmanifest/
      reconciler.go
      finalizer.go
      status.go

  generator/
    envoyfilter.go
    virtualservice.go
    destinationrule.go
    serviceentry.go

  microcks/
    client.go
    proto_loader.go
    scenario_loader.go
    health.go

  validator/
    proto.go
    scenario.go
    safety.go

  config/
    config.go
```

### 7.4 Environment configuration

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: virtualization-framework-config
  namespace: simulation-system
data:
  environment: "dev"
  microcks-url: "http://microcks-svc:8080"
  trigger-header: "test-data-simulation-action-name"
  microcks-operation-header: "x-microcks-operation"
  ingress-gateway-namespace: "istio-ingress"
  shadow-mirror-percentage: "10.0"
```

When `environment: prod` the operator enters restricted mode:

- Applies the ingress strip `EnvoyFilter` on startup
- Rejects any `SimulationManifest` with a `Forbidden` status
- Does not start Microcks
- Emits an alert metric for any `SimulationManifest` found in any namespace

---

## 8. Operator Design

### 8.1 Reconciliation loop

```
SimulationManifest created/updated
            │
            ▼
       reconcile(req)
  1. Fetch SimulationManifest
     If not found → return

  2. Safety check
     If environment == prod → set status=Forbidden, return

  3. Validate spec
     Parse inline protos
     Validate scenario JSON against proto fields
     If invalid → set status=Error, return

  4. Set status=Configuring

  5. Ensure Microcks is ready
     If not → requeue after 10s

  6. Load protos into Microcks

  7. Load scenarios into Microcks

  8. Generate and apply Istio resources
     a. EnvoyFilter inbound-capture
     b. EnvoyFilter outbound-inject
     c. EnvoyFilter microcks-rewrite
     d. Per thirdParty: ServiceEntry (if not pre-existing)
     e. Per thirdParty: VirtualService
     f. DestinationRule for Microcks

  9. Verify all generated resources exist
     If any missing → requeue after 5s

 10. Set status=Ready
```

### 8.2 Deletion (finalizer)

On deletion:

1. Delete `VirtualService` resources owned by this manifest
2. Remove Microcks examples for each scenario
3. Remove Microcks proto for each thirdParty
4. If no other `SimulationManifest` in namespace:
   - Delete `EnvoyFilter` inbound-capture
   - Delete `EnvoyFilter` outbound-inject
   - Delete `DestinationRule` for Microcks
5. Remove finalizer

### 8.3 Ownership labels

Every generated resource carries:

```yaml
labels:
  simulation.io/managed-by: virtualization-framework
  simulation.io/manifest-name: <name>
  simulation.io/manifest-namespace: <namespace>
annotations:
  simulation.io/manifest-uid: <uid>
```

Resources not carrying these labels (e.g. pre-existing `ServiceEntry`) are never modified.

### 8.4 Idempotency

The operator uses server-side apply. Identical input produces identical output. Partial failures are safe to retry.

### 8.5 Label watch

A secondary controller watches `Deployment` resources. When a Deployment gains or loses `simulation.io/propagation=enabled`, the operator requeues the `SimulationManifest` in that namespace to regenerate `EnvoyFilter` `workloadSelector`.

---

## 9. Overlay Deployment Architecture

### 9.1 Repository structure

```
simulation-framework/
├── helm/
│   └── simulation-framework/
│       ├── Chart.yaml
│       ├── values.yaml
│       ├── values-dev.yaml
│       ├── values-qa.yaml
│       ├── values-uat.yaml
│       ├── values-prod.yaml
│       └── templates/
│           ├── namespace.yaml
│           ├── operator-deployment.yaml
│           ├── operator-configmap.yaml
│           ├── operator-service-account.yaml
│           ├── operator-cluster-role.yaml
│           ├── operator-cluster-role-binding.yaml
│           ├── microcks-deployment.yaml
│           ├── microcks-service.yaml
│           └── crds/
│               └── simulationmanifests.yaml
│
└── kustomize/
    ├── base/
    │   ├── kustomization.yaml
    │   ├── namespace.yaml
    │   ├── operator-deployment.yaml
    │   ├── operator-configmap.yaml
    │   ├── operator-rbac.yaml
    │   ├── microcks-deployment.yaml
    │   ├── microcks-service.yaml
    │   └── crds/
    │       └── simulationmanifests.yaml
    └── overlays/
        ├── dev/
        │   ├── kustomization.yaml
        │   └── configmap-patch.yaml
        ├── qa/
        │   ├── kustomization.yaml
        │   └── configmap-patch.yaml
        ├── uat/
        │   ├── kustomization.yaml
        │   └── configmap-patch.yaml
        └── prod/
            ├── kustomization.yaml
            ├── configmap-patch.yaml
            └── disable-microcks.yaml
```

### 9.2 Kustomize base

```yaml
# kustomize/base/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: simulation-system

resources:
  - namespace.yaml
  - crds/simulationmanifests.yaml
  - operator-deployment.yaml
  - operator-configmap.yaml
  - operator-rbac.yaml
  - microcks-deployment.yaml
  - microcks-service.yaml

images:
  - name: simulation-framework
    newName: internal-registry/simulation/simulation-framework
    newTag: "1.0.0"
  - name: microcks
    newName: quay.io/microcks/microcks-uber
    newTag: "1.9.0"
```

### 9.3 Dev overlay

```yaml
# kustomize/overlays/dev/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../../base
patches:
  - path: configmap-patch.yaml
    target:
      kind: ConfigMap
      name: virtualization-framework-config
```

```yaml
# kustomize/overlays/dev/configmap-patch.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: virtualization-framework-config
data:
  environment: "dev"
  shadow-mirror-percentage: "0.0"
  microcks-replicas: "1"
```

### 9.4 Production overlay

```yaml
# kustomize/overlays/prod/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../../base
patches:
  - path: configmap-patch.yaml
    target:
      kind: ConfigMap
      name: virtualization-framework-config
  - path: disable-microcks.yaml
    target:
      kind: Deployment
      name: microcks
```

```yaml
# kustomize/overlays/prod/configmap-patch.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: virtualization-framework-config
data:
  environment: "prod"
  ingress-gateway-namespace: "istio-ingress"
```

```yaml
# kustomize/overlays/prod/disable-microcks.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: microcks
spec:
  replicas: 0
```

### 9.5 Overlay matrix

| Resource | dev | qa | uat | prod |
| --- | --- | --- | --- | --- |
| CRD `SimulationManifest` | ✓ | ✓ | ✓ | ✓ |
| Operator Deployment | ✓ | ✓ | ✓ | ✓ (restricted) |
| Microcks Deployment | ✓ | ✓ | ✓ | ✗ (replicas: 0) |
| SimulationManifest reconciliation | ✓ | ✓ | ✓ | ✗ (rejected) |
| Ingress strip EnvoyFilter | ✗ | ✗ | ✗ | ✓ (auto-applied) |
| Shadow mirror stanzas | ✗ | ✗ | ✓ (configurable) | ✗ |

---

## 10. Generated Resource Catalogue

For a `SimulationManifest` with N third-party services the operator generates:

| Resource | Kind | Namespace | Quantity |
| --- | --- | --- | --- |
| `{manifest-name}-inbound-capture` | EnvoyFilter | consumer namespace | 1 |
| `{manifest-name}-outbound-inject` | EnvoyFilter | consumer namespace | 1 |
| `microcks-scenario-rewrite` | EnvoyFilter | simulation-system | 1 (shared) |
| `{third-party-name}-simulation` | VirtualService | consumer namespace | N |
| `{third-party-name}-entry` | ServiceEntry | consumer namespace | N (if not pre-existing) |
| `microcks-destination` | DestinationRule | consumer namespace | 1 |
| `strip-simulation-header` | EnvoyFilter | istio-ingress | 1 (prod only) |

**Developer authors 1 resource. Operator generates up to 2N + 4.**

---

## 11. Operational Considerations

### 11.1 Scenario updates

```bash
# Edit SimulationManifest, re-apply
kubectl apply -f payment-simulation.yaml
# Operator updates Microcks within seconds. No restarts.
```

### 11.2 Monitoring

| Metric | Type | Description |
| --- | --- | --- |
| `simulation_manifest_reconcile_total` | Counter | Total reconciliations by result |
| `simulation_manifest_reconcile_duration_seconds` | Histogram | Duration per reconciliation |
| `simulation_manifest_ready` | Gauge | 1 if Ready, 0 otherwise |
| `simulation_microcks_load_errors_total` | Counter | Failed proto/scenario uploads |
| `simulation_prod_manifest_rejected_total` | Counter | Manifests rejected in prod — alert on any non-zero |
| `simulation_shadow_divergence_rate` | Gauge | Divergence rate per third-party (Layer 4) |

### 11.3 Framework upgrades

```bash
helm upgrade virtualization-framework \
  oci://internal-registry/simulation/virtualization-framework:1.1.0 \
  --namespace simulation-system \
  --values values-dev.yaml
```

---

## 12. Security Considerations

### 12.1 Production protection — three independent layers

**Layer 1 — Operator restricted mode:** `environment: prod` causes the operator to reject all `SimulationManifest` resources with `Forbidden` status. No routing resources generated.

**Layer 2 — Microcks disabled:** `replicas: 0` in prod overlay. No virtual backend exists to route to.

**Layer 3 — Ingress strip filter:** Operator applies an `EnvoyFilter` at the prod ingress gateway removing `test-data-simulation-action-name` from every inbound request unconditionally.

No single configuration error can activate virtual routing in production.

### 12.2 RBAC

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: virtualization-framework
rules:
  - apiGroups: ["simulation.io"]
    resources: ["simulationmanifests", "simulationmanifests/status"]
    verbs: ["get", "list", "watch", "update", "patch"]
  - apiGroups: ["networking.istio.io"]
    resources: ["virtualservices", "envoyfilters", "destinationrules", "serviceentries"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["get", "list", "watch", "create", "update", "patch"]
```

### 12.3 Image security

- Base image: `gcr.io/distroless/static:nonroot`. No shell, no package manager.
- Process runs as `nonroot:nonroot` (UID 65532).
- Image signed and signature verified at deploy time.
- CI blocks on high/critical CVEs.

---

## 13. Failure Modes and Mitigations

| Failure | Observed behaviour | Mitigation |
| --- | --- | --- |
| Operator pod down | New manifests not reconciled. Existing resources continue to function. | `replicas: 2`, PodDisruptionBudget ensures one replica always available. |
| Microcks down | Test calls receive `503` | `outlierDetection` ejects unhealthy Microcks. Operator sets manifest status to `Degraded`. |
| Proto syntax error in manifest | Status `Error: proto parse failed`. No resources generated. | Proto validated before any resources are created. Fix and re-apply. |
| Scenario references unknown third party | Status `Error: scenario references undefined third party` | Cross-reference validation runs before Microcks calls. |
| SimulationManifest applied to prod | Status `Forbidden`. No resources generated. Alert fires. | Production protection layer 1. |
| No labelled pods in namespace | Warning condition on manifest status | Operator checks for pods with propagation label and surfaces missing label as a status condition. |

---

## 14. Decision Record

### DR-001: Kubernetes Operator vs Helm chart templates

**Decision:** Implement as a Kubernetes Operator.

**Rationale:** Helm generates at install time. Day-2 operations (new scenario, new third party, labelling a service) require `helm upgrade`. An operator reconciles continuously, reacting to changes in seconds without CLI interaction.

### DR-002: Inline proto vs external reference

**Decision:** Support both inline proto and ConfigMap reference.

**Rationale:** Inline proto is lowest friction for getting started. ConfigMap reference avoids duplication for large or shared protos. Both produce identical operator behaviour.

### DR-003: One Microcks per environment vs one per manifest

**Decision:** One shared Microcks per environment.

**Rationale:** Multiple instances multiply routing complexity linearly. A single instance is sufficient for test environment load and simpler to operate and observe.

---

## 15. Open Questions

| # | Question | Owner | Target |
| --- | --- | --- | --- |
| OQ-1 | Should the framework ship a `kubectl` plugin with pre-apply validation? | Platform Engineering | Framework v1.1 |
| OQ-2 | For GitOps teams, should the `SimulationManifest` live in the application repo or the platform repo? | Engineering Leadership | Before GA |
| OQ-3 | Should the operator support a dry-run mode showing which resources would be generated? | Platform Engineering | Framework v1.1 |
| OQ-4 | Should shadow mirror percentage be configurable per manifest or per third-party? | Platform Engineering | Before Layer 4 rollout |

---

## 16. Appendix

### 16.1 Complete developer workflow

**Platform team (once per cluster):**

```bash
kubectl apply -k virtualization-framework/kustomize/overlays/dev
# or
helm install virtualization-framework \
  oci://internal-registry/simulation/virtualization-framework:1.0.0 \
  --namespace simulation-system --create-namespace \
  --values values-dev.yaml
```

**Service team:**

```bash
# Step 1: Done by platform team above.

# Step 2:
kubectl apply -f my-simulation.yaml
kubectl get simm my-simulation -n my-namespace
# Wait for STATUS: Ready

# Step 3:
kubectl label deployment service-a \
  simulation.io/propagation=enabled -n my-namespace
kubectl label deployment service-b \
  simulation.io/propagation=enabled -n my-namespace
```

**Test caller:**

```
grpc_call(
  target="service-a.my-namespace.svc.cluster.local:443",
  method="/com.example.MyService/MyMethod",
  metadata={"test-data-simulation-action-name": "my-scenario"}
)
```

### 16.2 Status conditions reference

| Phase | Meaning | Action |
| --- | --- | --- |
| `Pending` | Received, reconciliation not started | Wait |
| `Configuring` | Reconciliation in progress | Wait |
| `Ready` | All resources generated and loaded | Operational |
| `Degraded` | Partial — some resources created, Microcks not fully loaded | Check `message` |
| `Error` | Reconciliation failed, no resources generated | Fix error in `message`, re-apply |
| `Forbidden` | Found in production environment | Remove from prod. Never apply `SimulationManifest` to prod. |

### 16.3 Relationship to v1.0

This document does not replace v1.0. The v1.0 document remains authoritative for the Istio resource design, header propagation mechanics, Layer 4 shadow validation, and security model. This document describes the framework packaging and automation layer. Any change to the underlying design requires v1.0 to be revised first, followed by this document.
