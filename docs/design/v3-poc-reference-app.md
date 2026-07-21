# Technical Design Document

| Field | Value |
| --- | --- |
| **Title** | reference-app — Without Service Mesh (local / Compose) |
| **Version** | 1.0 |
| **Status** | Draft (implementation: complete for local path) |
| **Plain name** | **reference-app** (no mesh) |
| **Monorepo** | `apps/reference-app` |
| **Relates To** | v1 reference-app on mesh; v2 virtualization-framework |
| **Classification** | Internal Engineering |

---

## Document series placement

| Design | Plain English | Implementation |
| --- | --- | --- |
| **v3 (this doc)** | **reference-app** without service mesh | `apps/reference-app` — `make demo`, Docker Compose |
| **v1** | **Same reference-app** with service mesh | `apps/reference-app/kube` + Istio (see v1 design) |
| **v2** | **virtualization-framework** | `apps/virtualization-framework` |
| **Final** | Framework + meshed reference-app | `examples/reference-app-with-framework` |

**Primary focus of v3:** teach the application topology and header propagation
**without** requiring Kubernetes or Istio. Local mode uses
`SIMULATION_MODE=local` so fraud-checker dials a Microcks stand-in when the
simulation header is present.

Mesh deployment of this same app is specified in **v1** (not a second codebase).
Kubernetes teaching manifests under `apps/reference-app/kube/` implement that mesh path;
this document still describes services and protos shared by both modes.

See [`docs/MONOREPO.md`](../MONOREPO.md) (option B naming).

---

## Table of Contents

1. [Purpose](#1-purpose)
2. [What reference-app Demonstrates](#2-what-the-poc-demonstrates)
3. [Service Topology](#3-service-topology)
4. [Service Descriptions](#4-service-descriptions)
5. [gRPC API Definitions](#5-grpc-api-definitions)
6. [The Propagation Interceptor](#6-the-propagation-interceptor)
7. [Scenarios in Microcks](#7-scenarios-in-microcks)
8. [Project Structure](#8-project-structure)
9. [Tech Stack](#9-tech-stack)
10. [Environment Configuration](#10-environment-configuration)
11. [Build and Run](#11-build-and-run)
12. [Kubernetes Manifests Reference](#12-kubernetes-manifests-reference)
13. [SimulationManifest Reference](#13-simulationmanifest-reference)
14. [Key Files Explained](#14-key-files-explained)
15. [Expected Behaviour](#15-expected-behaviour)
16. [Scope Boundaries](#16-scope-boundaries)
17. [Developer Checklist](#17-developer-checklist)

---

## 1. Purpose

This document describes **reference-app** used throughout the document
series—especially the **no-mesh** path that any developer can run on a laptop. It
gives a complete, runnable example of header-driven virtualization before
introducing Istio (v1) or **virtualization-framework** (v2).

reference-app is not a production system. It is not a test suite. It is a teaching
tool. Every file is written to be read and understood, not just executed.

**Monorepo path:** `apps/reference-app`  
**Go module:** `github.com/servicemesh/reference-app`

**The single concept reference-app proves:**

> A test caller sends one header. Real internal services run without modification.
> Outbound third-party gRPC calls receive virtual responses for that scenario.
> The same service binary is used without mesh (v3) and with mesh (v1).

---

## 2. What reference-app Demonstrates

| Capability | Where it is visible |
| --- | --- |
| Single header activates virtualization | `cmd/test-client/main.go` — sends `test-data-simulation-action-name: fraud-declined` |
| Header propagates through A → B chain | `internal/sim/propagation.go` — server + client interceptors |
| Third-party call virtualized, not A or B | `kube/overlays/dev/virtual-service-external-risk.yaml` — VirtualService on third-party host only |
| Real service code is unchanged | `cmd/checkout-gateway/main.go` and `cmd/fraud-checker/main.go` contain zero simulation logic |
| Scenario switching without restart | Test client sends `fraud-approved` vs `fraud-declined` — Microcks returns different responses |
| Local mode vs mesh mode | Docker Compose uses app-level interceptor; Kubernetes uses EnvoyFilter |
| What the framework generates | `kube/overlays/dev/` — all Istio resources shown explicitly |

---

## 3. Service Topology

### 3.1 Normal flow (no simulation header)

```
Test Client
  │
  ▼ gRPC: ProcessCheckout(transactionId, nftToken, amount)
checkout-gateway  (Service A – port 9001)
  │
  ▼ gRPC: CheckFraud(transactionId, nftToken, amount)
fraud-checker  (Service B – port 9002)
  │
  ▼ gRPC: EvaluateRisk(nftToken, amount)
external-risk-api.com  (Third Party – port 443)
  │
  ▼ RiskResponse { risk_score: 5, decision: "APPROVE" }
fraud-checker → checkout-gateway → Test Client
APPROVED
```

### 3.2 Simulation flow (header present)

```
Test Client
  │
  ▼ gRPC: ProcessCheckout(...)
    metadata: test-data-simulation-action-name: fraud-declined
checkout-gateway  (Service A – real, unmodified)
  │  [Envoy or interceptor propagates header]
  ▼
fraud-checker  (Service B – real, unmodified)
  │  [Envoy or interceptor propagates header]
  ▼ gRPC: EvaluateRisk(...)  ← header present on this call
Istio VirtualService matches header
  │
  ▼
Microcks  (virtual backend)
  serves "fraud-declined" scenario:
  RiskResponse { risk_score: 92, decision: "DECLINE" }
  │
  ▼
fraud-checker → checkout-gateway → Test Client
DECLINED  (same card, same amount, different outcome — scenario controlled)
```

Services A and B do not change. The routing decision is made entirely in the mesh layer.

---

## 4. Service Descriptions

### 4.1 checkout-gateway (Service A)

**Responsibility:** Entry point for payment authorization. Orchestrates the fraud check and makes the final payment decision.

**gRPC server:** exposes `CheckoutGateway.ProcessCheckout`

**Logic:**

1. Receive `CheckoutRequest`
2. Call `fraud-checker.CheckFraud` with the transaction data
3. If recommendation is `DECLINE` → return `CheckoutResponse{ status: "DECLINED" }`
4. If recommendation is `APPROVE` → return `CheckoutResponse{ status: "APPROVED", auth_code: <generated> }`

**Simulation awareness:** none. The service logs the value of `test-data-simulation-action-name` if present in incoming metadata, but only for observability. It does not inspect or act on it.

**Interceptors registered:**

- `sim.ServerInterceptor()` — captures simulation header from inbound metadata into context
- `sim.ClientInterceptor()` — injects simulation header from context into outbound calls

### 4.2 fraud-checker (Service B)

**Responsibility:** Fraud evaluation. Delegates risk scoring to the external risk API and maps the score to a recommendation.

**gRPC server:** exposes `FraudChecker.CheckFraud`

**Logic:**

1. Receive `FraudCheckRequest`
2. Call `RiskService.EvaluateRisk` at `EXTERNAL_RISK_ENDPOINT`
3. Map score to recommendation:
   - score ≥ 70 → `DECLINE`
   - score < 70 → `APPROVE`
4. Return `FraudCheckResponse`

This is the service whose outbound call gets virtualized. It always calls `EXTERNAL_RISK_ENDPOINT`. In local mode, the interceptor may redirect to Microcks based on the simulation header. In mesh mode, Istio handles the redirect — the service code is identical.

**Interceptors registered:**

- `sim.ServerInterceptor()` — captures simulation header
- `sim.ClientInterceptor()` — propagates simulation header on the call to `RiskService`

**Environment variable:** `SIMULATION_MODE`

- `local` — interceptor routes to `MICROCKS_ENDPOINT` when simulation header present
- `mesh` — always routes to `EXTERNAL_RISK_ENDPOINT` (Istio handles virtualization)

### 4.3 external-risk (Local third-party stand-in)

**Responsibility:** Simulates the real external risk API for local development and baseline comparison.

**gRPC server:** exposes `RiskService.EvaluateRisk`

**Logic:**

- Card token prefix `nft_low_risk_*` → risk score 10, decision `APPROVE`
- Card token prefix `nft_high_risk_*` → risk score 85, decision `DECLINE`
- All other tokens → risk score 20, decision `APPROVE`

This service exists only in docker-compose. In Kubernetes it is replaced by a real external service. The `ServiceEntry` in the dev overlay declares the external host. The `VirtualService` redirects calls to Microcks when the simulation header is present.

### 4.4 test-client

**Responsibility:** Demonstrates virtualization by sending the same payment request twice with different headers and logging both outcomes.

**Behaviour:**

1. Send `ProcessCheckout` with card `nft_low_risk_4242`, amount `$50.00` — no simulation header
2. Log the response
3. Send `ProcessCheckout` with the same card and amount — with header `fraud-declined`
4. Log the response
5. Print comparison summary

**Expected output:**

```
SIMULATION FRAMEWORK POC — Integration Test

[1] Real path (no simulation header)
    card:   nft_low_risk_4242
    amount: $50.00
    ────────────────────────────────
    result: APPROVED
    auth:   ORDER-3821-K
    fraud:  risk_score=10, recommendation=APPROVE

[2] Simulated path (fraud-declined scenario)
    card:   nft_low_risk_4242  ← same card
    amount: $50.00             ← same amount
    header: test-data-simulation-action-name: fraud-declined
    ────────────────────────────────
    result: DECLINED
    reason: HIGH_RISK_SCORE
    fraud:  risk_score=92, recommendation=DECLINE

✓ Virtualization confirmed
  Same card. Same amount. Different outcome.
  Scenario controlled entirely by header.
```

---

## 5. gRPC API Definitions

### 5.1 checkout-gateway

```protobuf
syntax = "proto3";
package checkout.v1;
option go_package = "github.com/your-org/reference-app/gen/checkout/v1;checkoutv1";

service CheckoutGateway {
  rpc ProcessCheckout (CheckoutRequest) returns (CheckoutResponse);
}

message CheckoutRequest {
  string transaction_id = 1;
  string nft_token     = 2;
  int64  amount_cents   = 3;
  string currency       = 4;
}

message CheckoutResponse {
  string transaction_id = 1;
  string status         = 2; // APPROVED | DECLINED
  string auth_code      = 3;
  string decline_reason  = 4;
}
```

### 5.2 fraud-checker

```protobuf
syntax = "proto3";
package fraud.v1;
option go_package = "github.com/your-org/reference-app/gen/fraud/v1;fraudv1";

service FraudChecker {
  rpc CheckFraud (FraudCheckRequest) returns (FraudCheckResponse);
}

message FraudCheckRequest {
  string transaction_id = 1;
  string nft_token     = 2;
  int64  amount_cents   = 3;
}

message FraudCheckResponse {
  string transaction_id = 1;
  string recommendation = 2; // APPROVE | DECLINE
  int32  risk_score     = 3;
  string reason         = 4;
}
```

### 5.3 external-risk-api (third party — the one virtualized)

```protobuf
syntax = "proto3";
package risk.v1;
option go_package = "github.com/your-org/reference-app/gen/risk/v1;riskv1";

service RiskService {
  rpc EvaluateRisk (RiskRequest) returns (RiskResponse);
}

message RiskRequest {
  string nft_token   = 1;
  int64  amount_cents = 2;
}

message RiskResponse {
  int32           risk_score   = 1; // 0–100
  string          decision     = 2; // APPROVE | DECLINE
  repeated string risk_factors = 3;
}
```

---

## 6. The Propagation Interceptor

**File:** `internal/sim/propagation.go`

This is the only simulation-aware code in the application. It is used in local/Docker Compose mode. In Kubernetes with Istio, the EnvoyFilter resources generated by the framework perform this function automatically and this interceptor becomes a no-op.

### 6.1 Constant

```
Header name: test-data-simulation-action-name
Context key: unexported struct type (avoids key collisions)
```

### 6.2 ServerInterceptor — reads header, stores in context

On every inbound gRPC call to this server:

1. Read incoming gRPC metadata
2. Look for key `"test-data-simulation-action-name"`
3. If found and non-empty:
   - Store value in request context under the context key
4. Call the next handler with the updated context

### 6.3 ClientInterceptor — reads context, injects into outbound call

On every outbound gRPC call made by this client:

1. Read context for simulation context key
2. If found and non-empty:
   - Append `"test-data-simulation-action-name: <value>"` to outgoing gRPC metadata
3. Make the call

### 6.4 Local routing logic (fraud-checker only)

When `SIMULATION_MODE=local`, the fraud-checker client uses a custom dialer that inspects the simulation header before the call is made:

```
If simulation header present in outgoing metadata
AND SIMULATION_MODE == "local"
AND MICROCKS_ENDPOINT is configured:
    Dial MICROCKS_ENDPOINT instead of EXTERNAL_RISK_ENDPOINT
Else:
    Dial EXTERNAL_RISK_ENDPOINT as normal
```

This logic lives in `cmd/fraud-checker/main.go`, not in the shared interceptor, because it is specific to local mode. The interceptor itself is environment-neutral.

### 6.5 Registration

Every service that participates in the chain registers both interceptors:

```go
// Server — registers ServerInterceptor
grpc.NewServer(
  grpc.UnaryInterceptor(sim.ServerInterceptor()),
)

// Client — registers ClientInterceptor
grpc.NewClient(
  targetEndpoint,
  grpc.WithUnaryInterceptor(sim.ClientInterceptor()),
)
```

---

## 7. Scenarios in Microcks

Two scenarios are pre-loaded. Both target `risk.v1.RiskService/EvaluateRisk`.

### 7.1 fraud-approved

```yaml
name: fraud-approved
service: RiskService
version: v1
operation: EvaluateRisk
response:
  risk_score: 5
  decision: APPROVE
  risk_factors: []
```

### 7.2 fraud-declined

```yaml
name: fraud-declined
service: RiskService
version: v1
operation: EvaluateRisk
response:
  risk_score: 92
  decision: DECLINE
  risk_factors:
    - VELOCITY_BREACH
    - HIGH_AMOUNT
```

Microcks selects the scenario based on the `x-microcks-operation` header, which is rewritten from `test-data-simulation-action-name` by the EnvoyFilter on the Microcks sidecar (in mesh mode) or by the local routing logic (in local mode).

---

## 8. Project Structure

```
apps/reference-app/
├── README.md                          ← start here
├── Makefile                           ← all build, run, deploy commands
├── docker-compose.yml                 ← local full-stack
├── buf.gen.yaml                       ← proto generation config
├── go.mod                             ← Go module definition
│
├── proto/
│   ├── checkout/v1/checkout.proto
│   ├── fraud/v1/fraud.proto
│   └── risk/v1/risk.proto
│
├── gen/                               ← generated, not committed
│   ├── payment/v1/
│   │   ├── payment.pb.go
│   │   └── payment_grpc.pb.go
│   ├── fraud/v1/
│   │   ├── fraud.pb.go
│   │   └── fraud_grpc.pb.go
│   └── risk/v1/
│       ├── risk.pb.go
│       └── risk_grpc.pb.go
│
├── internal/
│   └── sim/
│       └── propagation.go             ← server + client interceptors
│
├── cmd/
│   ├── checkout-gateway/
│   │   └── main.go                    ← Service A
│   ├── fraud-checker/
│   │   └── main.go                    ← Service B
│   ├── external-risk/
│   │   └── main.go                    ← local third-party stand-in
│   └── test-client/
│       └── main.go                    ← sends test requests, prints diff
│
├── build/
│   ├── Dockerfile.checkout-gateway
│   ├── Dockerfile.fraud-checker
│   ├── Dockerfile.external-risk
│   └── Dockerfile.test-client
│
├── simulation/
│   ├── simulation-manifest.yaml       ← SimulationManifest CRD instance
│   └── microcks-scenarios/
│       ├── fraud-approved.yaml
│       └── fraud-declined.yaml
│
└── kube/
    └── kustomize/
        ├── base/
        │   ├── kustomization.yaml
        │   ├── checkout-gateway.yaml   ← Deployment + Service
        │   └── fraud-checker.yaml     ← Deployment + Service
        └── overlays/
            ├── dev/
            │   ├── kustomization.yaml
            │   ├── configmap.yaml
            │   ├── envoyfilter-inbound-capture.yaml
            │   ├── envoyfilter-outbound-inject.yaml
            │   ├── envoyfilter-microcks-rewrite.yaml
            │   ├── service-entry-external-risk.yaml
            │   ├── virtual-service-external-risk.yaml
            │   ├── destination-rule-microcks.yaml
            │   └── microcks.yaml
            └── prod/
                ├── kustomization.yaml
                └── envoyfilter-strip-header.yaml
```

---

## 9. Tech Stack

| Layer | Technology | Version | Reason |
| --- | --- | --- | --- |
| Language | Go | 1.22+ | Native gRPC, standard for Kubernetes tooling |
| gRPC | `google.golang.org/grpc` | v1.64+ | Standard Go gRPC library |
| Protobuf | `google.golang.org/protobuf` | v1.34+ | Generated message types |
| Proto toolchain | buf | v1.32+ | Simpler than raw protoc, schema linting |
| Containerisation | Docker | 24+ | Multi-stage builds, distroless runtime images |
| Local orchestration | Docker Compose | v2 | Single-command full-stack startup |
| Kubernetes packaging | Kustomize | v5+ | Overlay-based, matches TDD design |
| Virtual gRPC backend | Microcks | 1.9+ | Proto-native, scenario sets, Admin API |
| Service mesh | Istio | 1.21+ | EnvoyFilter + VirtualService |

---

## 10. Environment Configuration

### 10.1 checkout-gateway

| Variable | Default | Description |
| --- | --- | --- |
| `GRPC_PORT` | `9001` | Port the gRPC server listens on |
| `FRAUD_CHECKER_ENDPOINT` | `fraud-checker:9002` | Address of the fraud-checker service |
| `LOG_LEVEL` | `info` | Logging verbosity |

### 10.2 fraud-checker

| Variable | Default | Description |
| --- | --- | --- |
| `GRPC_PORT` | `9002` | Port the gRPC server listens on |
| `EXTERNAL_RISK_ENDPOINT` | `external-risk:9003` | Address of the external risk service |
| `MICROCKS_ENDPOINT` | `microcks:9090` | Address of Microcks gRPC server |
| `SIMULATION_MODE` | `mesh` | `local` routes to Microcks when header present; `mesh` always routes to `EXTERNAL_RISK_ENDPOINT` |
| `LOG_LEVEL` | `info` | Logging verbosity |

### 10.3 test-client

| Variable | Default | Description |
| --- | --- | --- |
| `CHECKOUT_GATEWAY_ENDPOINT` | `checkout-gateway:9001` | Address of the payment gateway |
| `SCENARIO` | *(empty)* | If set, overrides the scenario sent in the simulation header |

### 10.4 docker-compose vs Kubernetes ConfigMap

In `docker-compose.yml`, these are set inline under each service's `environment` block.

In Kubernetes, they are set in `kube/kustomize/overlays/dev/configmap.yaml` and mounted into the pod via `envFrom`. The `SIMULATION_MODE` is set to `mesh` in the Kubernetes ConfigMap — the app never routes to Microcks directly; Istio handles it.

---

## 11. Build and Run

### 11.1 Prerequisites

| Tool | Notes |
| --- | --- |
| Go 1.22+ | |
| buf CLI | https://buf.build/docs/installation |
| Docker 24+ | |
| Docker Compose v2 | |
| kubectl | (for Kubernetes mode) |
| Istio 1.21+ | (for Kubernetes mode, pre-installed on cluster) |

### 11.2 Local mode (Docker Compose)

```bash
# 1. Install buf (one-time)
brew install bufbuild/buf/buf   # macOS
# or see https://buf.build/docs/installation

# 2. Generate Go code from proto files
make generate

# 3. Build all service images
make build

# 4. Start the full stack
docker-compose up --build

# 5. In a new terminal — run the test client
docker-compose run --rm test-client

# 6. Tear down
docker-compose down
```

**What you will see in logs:**

```
checkout-gateway  | received ProcessCheckout txn=txn-001 card=nft_low_risk_4242 amount=5000
checkout-gateway  | calling fraud-checker
fraud-checker    | received CheckFraud txn=txn-001
fraud-checker    | simulation header absent — calling real external risk API
external-risk    | EvaluateRisk card=nft_low_risk_4242 → score=10 decision=APPROVE
fraud-checker    | risk_score=10 recommendation=APPROVE
checkout-gateway  | payment APPROVED auth=ORDER-3821-K

checkout-gateway  | received ProcessCheckout txn=txn-002 card=nft_low_risk_4242 amount=5000
checkout-gateway  | simulation header: fraud-declined
checkout-gateway  | calling fraud-checker
fraud-checker    | received CheckFraud txn=txn-002
fraud-checker    | simulation header: fraud-declined — routing to Microcks
fraud-checker    | risk_score=92 recommendation=DECLINE
checkout-gateway  | payment DECLINED reason=HIGH_RISK_SCORE
```

### 11.3 Kubernetes mode (Istio)

```bash
# 1. Generate and push images
make generate
make build push IMAGE_REGISTRY=your-registry.io TAG=latest

# 2. Create namespace
kubectl create namespace poc

# 3. Deploy base services + dev overlay
kubectl apply -k kube/kustomize/overlays/dev

# 4. Apply the SimulationManifest
kubectl apply -f simulation/simulation-manifest.yaml -n poc

# 5. Label services for header propagation
kubectl label deployment checkout-gateway simulation.io/propagation=enabled -n poc
kubectl label deployment fraud-checker simulation.io/propagation=enabled -n poc

# 6. Watch SimulationManifest reach Ready
kubectl get simm -n poc -w

# 7. Run test client
kubectl run test-client \
  --image=your-registry.io/test-client:latest \
  --env="CHECKOUT_GATEWAY_ENDPOINT=checkout-gateway-svc.poc.svc.cluster.local:9001" \
  --restart=Never -n poc

# 8. View results
kubectl logs test-client -n poc
```

### 11.4 Makefile targets

| Target | Description |
| --- | --- |
| `make generate` | Run buf to generate Go code from proto files |
| `make build` | Build all Docker images locally |
| `make push` | Push images to registry (set `IMAGE_REGISTRY`) |
| `make up` | Start docker-compose stack |
| `make down` | Stop docker-compose stack |
| `make test` | Run test-client against local stack |
| `make deploy-dev` | Apply Kubernetes dev overlay |
| `make clean` | Remove generated code and built images |

---

## 12. Kubernetes Manifests Reference

### 12.1 base/checkout-gateway.yaml

Standard Kubernetes Deployment and Service. Notable fields:

```yaml
spec:
  template:
    metadata:
      labels:
        app: checkout-gateway
        # simulation.io/propagation=enabled added separately by operator or kubectl label
    spec:
      containers:
        - name: checkout-gateway
          envFrom:
            - configMapRef:
                name: poc-config
```

No sidecar annotations. No simulation-specific configuration. The Deployment is identical to what you would deploy in production.

### 12.2 overlays/dev/envoyfilter-inbound-capture.yaml

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: poc-inbound-capture
  namespace: poc
spec:
  workloadSelector:
    labels:
      simulation.io/propagation: "enabled"
  configPatches:
    - applyTo: HTTP_FILTER
      match:
        context: SIDECAR_INBOUND
        listener:
          filterChain:
            filter:
              name: envoy.filters.network.http_connection_manager
      patch:
        operation: INSERT_BEFORE
        value:
          name: envoy.filters.http.lua
          typed_config:
            "@type": type.googleapis.com/envoy.extensions.filters.http.lua.v3.LuaPerRoute
            inline_code: |
              local HEADER  = "test-data-simulation-action-name"
              local STATE_KEY = "test.simulation.action.name"

              function envoy_on_request(request_handle)
                local scenario = request_handle:headers():get(HEADER)
                if scenario ~= nil and scenario ~= "" then
                  request_handle:streamInfo():filterState():set(
                    STATE_KEY, scenario, "MUTABLE", "CONNECTION")
                end
              end
```

### 12.3 overlays/dev/envoyfilter-outbound-inject.yaml

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: poc-outbound-inject
  namespace: poc
spec:
  workloadSelector:
    labels:
      simulation.io/propagation: "enabled"
  configPatches:
    - applyTo: HTTP_FILTER
      match:
        context: SIDECAR_OUTBOUND
        listener:
          filterChain:
            filter:
              name: envoy.filters.network.http_connection_manager
      patch:
        operation: INSERT_BEFORE
        value:
          name: envoy.filters.http.lua
          typed_config:
            inline_code: |
              local HEADER  = "test-data-simulation-action-name"
              local STATE_KEY = "test.simulation.action.name"

              function envoy_on_request(request_handle)
                local scenario = request_handle:streamInfo():filterState():get(
                  STATE_KEY)
                if scenario ~= nil then
                  request_handle:headers():replace(HEADER, scenario)
                end
              end
```

### 12.4 overlays/dev/service-entry-external-risk.yaml

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: ServiceEntry
metadata:
  name: external-risk-api
  namespace: poc
spec:
  hosts:
    - external-risk-api.com
  ports:
    - number: 443
      name: grpc
      protocol: GRPC
  resolution: DNS
  location: MESH_EXTERNAL
```

### 12.5 overlays/dev/virtual-service-external-risk.yaml

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: external-risk-simulation
  namespace: poc
spec:
  hosts:
    - external-risk-api.com
  http:
    # Header present → Microcks
    - match:
        - headers:
            test-data-simulation-action-name:
              regex: ".+"
      route:
        - destination:
            host: microcks-svc.simulation-system.svc.cluster.local
            port:
              number: 9090
    # No header → real third party
    - route:
        - destination:
            host: external-risk-api.com
            port:
              number: 443
```

### 12.6 overlays/dev/destination-rule-microcks.yaml

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: DestinationRule
metadata:
  name: microcks-grpc
  namespace: poc
spec:
  host: microcks-svc.simulation-system.svc.cluster.local
  trafficPolicy:
    connectionPool:
      http:
        h2UpgradePolicy: UPGRADE
    loadBalancer:
      simple: ROUND_ROBIN
    tls:
      mode: ISTIO_MUTUAL
    outlierDetection:
      consecutive5xxErrors: 3
      interval: 30s
      baseEjectionTime: 60s
```

### 12.7 overlays/dev/envoyfilter-microcks-rewrite.yaml

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: microcks-scenario-rewrite
  namespace: simulation-system
spec:
  workloadSelector:
    labels:
      app: microcks
  configPatches:
    - applyTo: HTTP_FILTER
      match:
        context: SIDECAR_INBOUND
        listener:
          filterChain:
            filter:
              name: envoy.filters.network.http_connection_manager
      patch:
        operation: INSERT_BEFORE
        value:
          name: envoy.filters.http.lua
          typed_config:
            inline_code: |
              function envoy_on_request(request_handle)
                local scenario = request_handle:headers():get(
                  "test-data-simulation-action-name")
                if scenario ~= nil then
                  request_handle:headers():replace(
                    "x-microcks-operation", scenario)
                end
              end
```

### 12.8 overlays/prod/envoyfilter-strip-header.yaml

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: strip-simulation-header
  namespace: istio-ingress
spec:
  configPatches:
    - applyTo: HTTP_FILTER
      match:
        context: GATEWAY
      patch:
        operation: INSERT_BEFORE
        value:
          name: envoy.filters.http.lua
          typed_config:
            inline_code: |
              function envoy_on_request(request_handle)
                request_handle:headers():remove(
                  "test-data-simulation-action-name")
              end
```

---

## 13. SimulationManifest Reference

**File:** `simulation/simulation-manifest.yaml`

```yaml
apiVersion: simulation.io/v1alpha1
kind: SimulationManifest
metadata:
  name: poc-simulation
  namespace: poc
spec:
  thirdParties:
    - name: external-risk
      host: external-risk-api.com
      port: 443
      proto: |
        syntax = "proto3";
        package risk.v1;
        service RiskService {
          rpc EvaluateRisk (RiskRequest) returns (RiskResponse);
        }
        message RiskRequest {
          string nft_token   = 1;
          int64  amount_cents = 2;
        }
        message RiskResponse {
          int32           risk_score   = 1;
          string          decision     = 2;
          repeated string risk_factors = 3;
        }

  scenarios:
    - name: fraud-approved
      responses:
        external-risk:
          - operation: EvaluateRisk
            body: '{ "risk_score": 5, "decision": "APPROVE", "risk_factors": [] }'

    - name: fraud-declined
      responses:
        external-risk:
          - operation: EvaluateRisk
            body: '{ "risk_score": 92, "decision": "DECLINE", "risk_factors": ["VELOCITY_BREACH", "HIGH_AMOUNT"] }'
```

When applied to the cluster (with the v2.0 virtualization-framework operator installed), this single resource causes the operator to generate all the EnvoyFilter, VirtualService, DestinationRule, and Microcks configuration described in Section 12. Developers do not author those Istio resources manually — they apply this one file.

---

## 14. Key Files Explained

| File | Why it matters |
| --- | --- |
| `internal/sim/propagation.go` | The only simulation-aware application code. Two interceptors, ~50 lines. Understand this first. |
| `cmd/fraud-checker/main.go` | Shows how interceptors are registered and how `SIMULATION_MODE` controls local routing. |
| `cmd/checkout-gateway/main.go` | Shows a service that is completely unaware of simulation — it just registers the interceptor without any conditional logic. |
| `cmd/test-client/main.go` | Shows the complete test caller pattern: same request, two headers, two outcomes. |
| `kube/overlays/dev/virtual-service-external-risk.yaml` | The single VirtualService routing rule. Two rules: header present → Microcks; no header → real. |
| `kube/overlays/dev/envoyfilter-inbound-capture.yaml` | How Envoy captures the header into filter state. Replaces the need for the server interceptor in Kubernetes. |
| `kube/overlays/dev/envoyfilter-outbound-inject.yaml` | How Envoy propagates the header on every outbound call. Replaces the need for the client interceptor in Kubernetes. |
| `simulation/simulation-manifest.yaml` | What developers submit to the framework. Everything above is generated from this. |
| `docker-compose.yml` | Full local stack. Read this to understand how the services wire together before looking at the Go code. |

---

## 15. Expected Behaviour

### 15.1 Local mode (Docker Compose)

| Condition | Expected result |
| --- | --- |
| No simulation header | Calls route to `external-risk` container. Real risk score returned. Payment APPROVED for low-risk card. |
| Header = `fraud-approved` | Calls route to Microcks. Returns risk score 5. Payment APPROVED. |
| Header = `fraud-declined` | Calls route to Microcks. Returns risk score 92. Payment DECLINED. |
| Header = `unknown-scenario` | Microcks returns `NOT_FOUND` gRPC status. fraud-checker logs the error. Payment DECLINED (fail safe). |
| Microcks container down | Calls fall back to `external-risk` because the interceptor catches the connection error and retries the real endpoint. |

### 15.2 Kubernetes mode (Istio)

| Condition | Expected result |
| --- | --- |
| No simulation header | VirtualService default rule → real `external-risk-api.com`. |
| Header present | EnvoyFilter propagates header → VirtualService matches → Microcks. |
| Header present in prod | Ingress strip filter removes header before it enters the mesh. Real path is always used in prod. |
| SimulationManifest applied to prod | Operator returns `Forbidden` status. No resources generated. |

---

## 16. Scope Boundaries

The POC does not include:

| Excluded | Reason |
| --- | --- |
| Production manifests | Prod overlay contains only the ingress strip filter. Developers should not look at prod to understand the pattern. |
| Streaming RPCs | Unary RPCs only. Streaming adds complexity that obscures the core propagation pattern. Covered in v1.0 TDD Section 7.5. |
| The virtualization-framework operator | The POC applies Istio resources directly from `kube/overlays/dev/`. This makes the generated resources visible. In a real integration, apply only `simulation/simulation-manifest.yaml` and the operator generates the rest. |
| mTLS / authentication | Plain gRPC for clarity. Production uses Istio mTLS as documented in v1.0 TDD Section 10. |
| Multiple third parties | One third party (external-risk-api) is enough to demonstrate the pattern. Extending to multiple is additive: one additional ServiceEntry + VirtualService per third party, one additional entry in the SimulationManifest. |

---

## 17. Developer Checklist

Use this to verify reference-app is working correctly after each mode of deployment.

### Local (Docker Compose)

- [ ] `make generate` completes without errors
- [ ] `make build` completes without errors
- [ ] `docker-compose up` starts all five containers (checkout-gateway, fraud-checker, external-risk, microcks, test-client)
- [ ] `docker-compose run test-client` prints two results: APPROVED then DECLINED
- [ ] Log for second request shows routing to Microcks in fraud-checker logs
- [ ] Changing `SCENARIO=fraud-approved` in test-client env produces APPROVED for both requests

### Kubernetes (Istio)

- [ ] `kubectl apply -k kube/kustomize/overlays/dev` creates all resources without error
- [ ] `kubectl get simm poc-simulation -n poc` shows `STATUS: Ready`
- [ ] Payment gateway and fraud-checker pods show Istio sidecar injected (2/2 containers running)
- [ ] Test client logs show APPROVED for request 1 (no header)
- [ ] Test client logs show DECLINED for request 2 (header = `fraud-declined`)
- [ ] `kubectl logs -l app=fraud-checker -n poc` shows no direct routing to Microcks — Istio handled it
- [ ] Removing `simulation.io/propagation=enabled` label from fraud-checker causes request 2 to also return APPROVED (header not propagated to third-party call — confirms EnvoyFilter is doing the work)
- [ ] Applying prod overlay strips the header: `kubectl apply -k kube/kustomize/overlays/prod` → test client with simulation header still receives APPROVED (header stripped at ingress)

---

*Document reconstructed from screenshots (`IMG_3683.jpeg`–`IMG_3712.jpeg`) of `v3-poc-reference-app.md`.*
