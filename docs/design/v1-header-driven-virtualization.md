# Technical Design Document

| Field | Value |
| --- | --- |
| **Title** | reference-app on Service Mesh — Header-Driven Virtualization with Istio/gRPC |
| **Version** | 1.0 |
| **Status** | Draft |
| **Plain name** | **reference-app with service mesh** |
| **Monorepo** | Same app as v3: `apps/reference-app`, mesh path `apps/reference-app/kube/` |
| **Relates To** | v3 reference-app no mesh; v2 virtualization-framework |
| **Classification** | Internal Engineering |

---

## Document series placement

| Design | Plain English | Implementation |
| --- | --- | --- |
| **v3** | **reference-app** without service mesh | `apps/reference-app` — `make demo`, Compose, `SIMULATION_MODE=local` |
| **v1 (this doc)** | **Same reference-app on a service mesh** | Istio — `SIMULATION_MODE=mesh`, Envoy + VirtualService |
| **v2** | **virtualization-framework** (operator) | `apps/virtualization-framework` |
| **Final** | Framework + meshed reference-app | `examples/reference-app-with-framework` |

**v1 is not a second application.** It is how **reference-app** from v3 runs when
Istio owns header propagation and third-party routing. Application binaries stay
identical; the data plane supplies virtualization.

See also: monorepo map [`docs/MONOREPO.md`](../MONOREPO.md) (option B naming).

---

## Table of Contents

1. [Abstract](#1-abstract)
2. [Background](#2-background)
3. [Goals and Non-Goals](#3-goals-and-non-goals)
4. [Terminology](#4-terminology)
5. [System Context](#5-system-context)
6. [Design Overview](#6-design-overview)
7. [Detailed Design](#7-detailed-design)
   - 7.1 [Header Schema](#71-header-schema)
   - 7.2 [Header Propagation](#72-header-propagation)
   - 7.3 [Traffic Routing — Layer 3](#73-traffic-routing--layer-3)
   - 7.4 [Virtual Service Backend — Microcks](#74-virtual-service-backend--microcks)
   - 7.5 [Traffic Shadowing — Layer 4](#75-traffic-shadowing--layer-4)
8. [Deployment Architecture](#8-deployment-architecture)
9. [Operational Considerations](#9-operational-considerations)
10. [Security Considerations](#10-security-considerations)
11. [Failure Modes and Mitigations](#11-failure-modes-and-mitigations)
12. [Decision Record](#12-decision-record)
13. [Open Questions](#13-open-questions)
14. [Appendix](#14-appendix)

---

## 1. Abstract

This document describes **header-driven service virtualization for
reference-app when it runs on an Istio service mesh**. It is the mesh-mode design
for the same gRPC services introduced in v3 (payment-gateway → fraud-checker →
external risk). Test callers exercise real internal services while the mesh
substitutes controlled virtual responses for outbound third-party gRPC calls—without
changing service business logic or shipping environment-specific binaries.

A single HTTP/2 metadata header — `test-data-simulation-action-name` — carries the
scenario name. Envoy sidecar filters propagate this header across every hop in
the internal call chain. When any mesh service makes an outbound gRPC call to a
third party, the Istio data plane routes that call to a Microcks (or stand-in)
virtual gRPC server, which serves the response defined for the named scenario.

In contrast to v3 local mode (where the app may dial a mock endpoint when the
header is present), **mesh mode always dials the real third-party host name**;
Istio performs the redirect. That is how the **same** binary works for local
reference-app, meshed reference-app, and production.

Production traffic is never affected. No application code changes for
virtualization routing. No service binary is environment-specific.

**Teaching implementation:** hand-written resources under `apps/reference-app/kube/`.  
**Product automation:** **virtualization-framework** (v2) generates equivalent
resources from a `SimulationManifest` (see `examples/reference-app-with-framework`).

---

## 2. Background

### 2.1 Problem Statement

Testing a distributed gRPC system that depends on external third-party services presents three compounding problems.

**Test isolation.** Third-party services are not under the engineering team's control. They may be unavailable, rate-limited, non-deterministic, or require payment credentials for each call. Running integration tests against live third-party endpoints is fragile and expensive.

**Scenario coverage.** Production third-party calls return the response that real data produces. Testing specific scenarios — declined authorization, network timeout, invalid card — requires either manipulating production data or maintaining separate test accounts. Neither scales.

**Environment fidelity.** Current approaches to service virtualization (in-process mocks, bundled stub responses, environment-variable feature flags) require the test binary to differ from the production binary, or require application code to know whether it is running in a test context. Both approaches violate the principle that the artifact deployed to production is the same artifact tested before it.

### 2.2 Current State

The existing approach relies on build-time switches that bundle stub response fixtures into the application binary for non-production environments. This produces the following consequences:

- The binary that runs in a development environment is not the binary that runs in production. Defects that depend on the difference are not caught before deployment.
- Adding a new test scenario requires a code change, a build, and a deployment.
- Switching between scenarios at runtime requires restarting the service.
- The stub responses are maintained by the consumer team and drift from the actual third-party API over time.
- There is no mechanism to validate that stub responses match what the real third party returns.

### 2.3 Driving Requirements

The replacement design must satisfy the following requirements, in priority order:

| Priority | Requirement |
| --- | --- |
| P0 | A single binary is deployed to all environments. No environment-specific build flags. |
| P0 | Application service code is not modified to support virtualization. |
| P0 | Production traffic is never routed to a virtual service under any circumstance. |
| P1 | A test caller can select a named scenario via a single request header. |
| P1 | Scenario changes take effect without restarting any service. |
| P1 | Virtual responses are validated against real third-party responses before use in tests. |
| P2 | The design extends to new third-party services without changes to existing services. |

---

## 3. Goals and Non-Goals

### Goals

- Define a single header-based protocol for activating service virtualization per request.
- Design an Envoy-native mechanism to propagate the header across all internal service hops without application code changes.
- Route outbound gRPC calls to third parties to a Microcks virtual server when the header is present.
- Define a Layer 4 traffic shadowing mechanism to validate scenario fidelity against real third-party responses.
- Ensure the production namespace is structurally incapable of routing to virtual services.

### Non-Goals

- **Virtualizing internal service-to-service calls.** Services A, B, and C remain real in all environments. Only third-party egress is virtualized.
- **In-process mocking or unit test doubles.** Out of scope. This design covers Layer 3 (mesh routing) and Layer 4 (shadow validation) only.
- **Contract testing (Pact or equivalent).** Out of scope. Scenario fidelity is validated via traffic shadowing, not consumer-driven contract tests.
- **Multi-tenant scenario isolation.** All requests with the same scenario name receive the same response. Per-request state machines are not in scope.
- **HTTP/1.1 services.** This design targets gRPC (HTTP/2) exclusively. HTTP/1.1 services require a separate design.

---

## 4. Terminology

| Term | Definition |
| --- | --- |
| **Virtual service** | A controlled substitute for a real downstream service that returns programmed responses. Not to be confused with Istio `VirtualService` resource. |
| **Scenario** | A named, pre-defined set of responses for each virtualizable third-party service, corresponding to a specific test condition (e.g. `payment-declined`, `fraud-flagged`). |
| **Mesh service** | A service running inside the Istio service mesh — Services A, B, C in this design. These are real services and are never virtualized. |
| **Third party** | An external gRPC service outside the Istio mesh boundary. These are the only services that are virtualized. |
| **Header propagation** | The act of forwarding the simulation header from each inbound request to all outbound requests made by the same service. |
| **Microcks** | An open-source API mocking and contract testing server. Used here as the virtual gRPC backend. |
| **EnvoyFilter** | An Istio resource that injects custom Lua or WASM logic into Envoy's filter chain at specific processing points. |
| **Layer 3** | Mesh-level traffic routing: routing test traffic to virtual services via Istio `VirtualService`. |
| **Layer 4** | Production validation via Istio traffic mirroring: copying real production traffic to virtual services and comparing responses. |
| **Shadow traffic** | A copy of real traffic sent to a secondary destination for observation purposes. The response from the shadow destination is discarded; the caller receives only the real response. |

---

## 5. System Context

### 5.1 Topology

```
                    Istio Service Mesh
┌─────────────────────────────────────────────────────────┐
│                                                         │
│   Service A ──────► Service B ──────► Service C         │
│   [Envoy sidecar]   [Envoy sidecar]   [Envoy sidecar]   │
│        │                 │                 │            │
└────────┼─────────────────┼─────────────────┼────────────┘
         ▼                 ▼                 ▼
   Third Party X     Third Party Y     Third Party Z
   (gRPC external)   (gRPC external)   (gRPC external)
```

### 5.2 Virtualization overlay (test traffic only)

When a request carrying `test-data-simulation-action-name` enters the mesh, the topology becomes:

```
                    Istio Service Mesh
┌─────────────────────────────────────────────────────────┐
│                                                         │
│   Service A ──────► Service B ──────► Service C         │
│    (real)            (real)            (real)           │
│        │                 │                 │            │
│   VirtualService    VirtualService    VirtualService    │
│    intercepts        intercepts        intercepts       │
│        └─────────────────┼─────────────────┘            │
│                          ▼                              │
│                      Microcks                           │
│                   (virtual gRPC)                        │
└─────────────────────────────────────────────────────────┘
```

Services A, B, C run their real application code. Only their outbound calls to external third parties are redirected.

---

## 6. Design Overview

### 6.1 Request lifecycle

**Step 1 — Test caller sends request**

- gRPC call to Service A
- metadata: `test-data-simulation-action-name: payment-declined`

**Step 2 — Envoy inbound filter at Service A**

- Lua filter reads the header
- Stores `"payment-declined"` in Envoy connection filter state

**Step 3 — Service A processes and calls Service B**

- Envoy outbound filter at Service A
- Lua filter reads from filter state
- Injects `test-data-simulation-action-name: payment-declined` into the outbound gRPC metadata

**Step 4 — Envoy inbound filter at Service B**

- Receives the header, stores in filter state (same as Step 2)

**Step 5 — Service B calls Third Party Y**

- Envoy outbound filter at Service B injects the header
- Istio VirtualService for Third Party Y matches on header presence
- Routes to Microcks instead of real Third Party Y

**Step 6 — Microcks receives the call**

- Envoy inbound filter at Microcks rewrites `test-data-simulation-action-name` → `x-microcks-operation`
- Microcks matches on:
  - gRPC service + method (from `:path`)
  - scenario name (from `x-microcks-operation = "payment-declined"`)
- Returns the programmed response for that scenario

**Step 7 — Response propagates back**

- Service B receives virtual response, processes it normally
- Service A receives Service B's response
- Test caller receives the final response

### 6.2 Key design decisions summary

| Decision | Choice | Rationale |
| --- | --- | --- |
| Header format | Single header, value = scenario name | Simplest possible interface for the test caller. All routing and scenario selection derived from one value. |
| Propagation mechanism | Envoy Lua filter (not application code) | Zero application code changes. Applied declaratively via EnvoyFilter resources. |
| Propagation scope | Filter state at CONNECTION scope | Per-request scope is not shared across HTTP/2 stream boundaries in Envoy's filter model. CONNECTION scope is the correct mechanism for this use case. |
| Virtual backend | Microcks | Native gRPC support via proto files, scenario sets, Admin API for runtime updates, health check implementation. |
| Scenario selection at Microcks | Envoy Lua rewrite on Microcks inbound | Decouples the test header name from Microcks internals. The application never needs to know about Microcks' native header. |
| Traffic shadowing | Istio mirror on non-virtual path | Validates scenario fidelity against real third-party responses before scenarios are promoted to active use. |

---

## 7. Detailed Design

### 7.1 Header Schema

**Header name:** `test-data-simulation-action-name`

**Header value:** A string identifier for the test scenario. The value is a logical name agreed upon between the test team and the scenario authors. It must map to a named example in Microcks for every virtualizable third-party service.

**Examples:**

| Value | Intended scenario |
| --- | --- |
| `payment-declined` | All third-party calls return a declined authorization response |
| `fraud-flagged` | Fraud evaluation service returns high-risk score; other services return nominal responses |
| `network-timeout` | All third-party calls respond after a configured delay, triggering deadline exceeded |
| `valid-transaction` | All third-party calls return nominal success responses |

**Header presence semantics:**

- Header present with any non-empty value → all outbound third-party gRPC calls virtualized using the named scenario.
- Header absent → all outbound third-party gRPC calls route to real third parties.
- Header present with an unrecognised scenario name → Microcks serves the configured default example for each service. If no default is configured, Microcks returns `NOT_FOUND`.

**Header immutability:**

The header value must not be modified by any mesh service. EnvoyFilter resources use `replace` semantics to prevent duplication, not `add`. If a header with the same name is already present on an outbound call (set by the application), the filter overwrites it with the value captured from the inbound context.

### 7.2 Header Propagation

#### 7.2.1 The propagation problem

The `test-data-simulation-action-name` header arrives on the inbound gRPC call to Service A. Istio `VirtualService` routing decisions for Third Party Y are evaluated at the moment Service B makes its outbound call. The header must therefore be present on that outbound call.

Envoy's filter pipeline processes inbound and outbound requests as separate filter chains. Dynamic metadata set during inbound processing is not natively available to outbound filter chains because they represent distinct request processing contexts.

The solution uses Envoy's connection-scoped filter state, which persists for the lifetime of the underlying TCP connection and is accessible to both inbound and outbound filter chains operating on that connection.

#### 7.2.2 Inbound capture filter

Applied to every mesh service that participates in the call chain.

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: test-simulation-inbound-capture
  namespace: payments-namespace
spec:
  workloadSelector:
    labels:
      test-simulation-propagation: "enabled"
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
              local HEADER   = "test-data-simulation-action-name"
              local STATE_KEY = "test.simulation.action.name"

              function envoy_on_request(request_handle)
                local scenario = request_handle:headers():get(HEADER)
                if scenario ~= nil and scenario ~= "" then
                  request_handle:streamInfo():filterState():set(
                    STATE_KEY,
                    scenario,
                    "MUTABLE",
                    "CONNECTION"
                  )
                end
              end
```

#### 7.2.3 Outbound inject filter

Applied to the same mesh services as the capture filter.

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: test-simulation-outbound-inject
  namespace: payments-namespace
spec:
  workloadSelector:
    labels:
      test-simulation-propagation: "enabled"
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
              local HEADER   = "test-data-simulation-action-name"
              local STATE_KEY = "test.simulation.action.name"

              function envoy_on_request(request_handle)
                local scenario = request_handle:streamInfo():filterState():get(
                  STATE_KEY)
                if scenario ~= nil then
                  request_handle:headers():replace(HEADER, scenario)
                end
              end
```

#### 7.2.4 Opt-in label

The two filters are scoped to pods carrying the label `test-simulation-propagation: "enabled"`. This label is applied to Services A, B, and C in their Deployment manifests:

```yaml
spec:
  template:
    metadata:
      labels:
        app: service-a
        test-simulation-propagation: "enabled"
```

#### 7.2.5 CONNECTION scope trade-off

Filter state at CONNECTION scope persists for the lifetime of the TCP/TLS connection. In an HTTP/2 connection, multiple gRPC streams are multiplexed on the same connection. If two concurrent requests arrive on the same connection — one carrying `test-data-simulation-action-name` and one without it — the connection-scoped filter state reflects whichever request was processed most recently by the capture filter.

**Implication:** virtual traffic and non-virtual traffic must not share a connection pool. In a dedicated test environment this is structurally guaranteed. In a shared environment, test callers must use a dedicated connection pool isolated from other callers.

### 7.3 Traffic Routing — Layer 3

#### 7.3.1 VirtualService structure

A `VirtualService` is added for each third-party service that can be virtualized. The structure is identical for every third party; only the hostname changes.

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: <third-party-name>-simulation
  namespace: payments-namespace
spec:
  hosts:
    - <third-party-hostname>
  http:
    # Rule 1: header present → route to Microcks
    - match:
        - headers:
            test-data-simulation-action-name:
              regex: ".+"
      route:
        - destination:
            host: microcks-svc.simulation-namespace.svc.cluster.local
            port:
              number: 9090

    # Rule 2: header absent → route to real third party (default)
    - route:
        - destination:
            host: <third-party-hostname>
            port:
              number: 443
```

#### 7.3.2 DestinationRule for Microcks

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: DestinationRule
metadata:
  name: microcks-grpc-destination
  namespace: payments-namespace
spec:
  host: microcks-svc.simulation-namespace.svc.cluster.local
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
      maxEjectionPercent: 100
```

### 7.4 Virtual Service Backend — Microcks

#### 7.4.1 Scenario header rewrite

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: microcks-scenario-rewrite
  namespace: simulation-namespace
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

#### 7.4.2 Microcks deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: microcks
  namespace: simulation-namespace
spec:
  replicas: 2
  selector:
    matchLabels:
      app: microcks
  template:
    metadata:
      labels:
        app: microcks
    spec:
      containers:
        - name: microcks
          image: quay.io/microcks/microcks-uber:1.9.x
          ports:
            - name: http
              containerPort: 8080
            - name: grpc
              containerPort: 9090
          volumeMounts:
            - name: proto-artifacts
              mountPath: /deployments/repository
          readinessProbe:
            grpc:
              port: 9090
            initialDelaySeconds: 15
            periodSeconds: 10
          livenessProbe:
            httpGet:
              path: /api/health
              port: 8080
            initialDelaySeconds: 30
            periodSeconds: 15
      volumes:
        - name: proto-artifacts
          configMap:
            name: microcks-proto-artifacts
---
apiVersion: v1
kind: Service
metadata:
  name: microcks-svc
  namespace: simulation-namespace
spec:
  selector:
    app: microcks
  ports:
    - name: http
      port: 8080
      targetPort: 8080
    - name: grpc
      port: 9090
      targetPort: 9090
```

### 7.5 Traffic Shadowing — Layer 4

#### 7.5.1 Purpose

Layer 4 validates that the scenario responses defined in Microcks match what the real third-party service would return for equivalent requests. This validation runs before a scenario is promoted to active use in any shared test environment.

#### 7.5.2 Shadow configuration

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: <third-party-name>-simulation
  namespace: payments-namespace
spec:
  hosts:
    - <third-party-hostname>
  http:
    # Rule 1: header present → Microcks (unchanged)
    - match:
        - headers:
            test-data-simulation-action-name:
              regex: ".+"
      route:
        - destination:
            host: microcks-svc.simulation-namespace.svc.cluster.local
            port:
              number: 9090

    # Rule 2: header absent → real third party with shadow copy to Microcks
    - route:
        - destination:
            host: <third-party-hostname>
            port:
              number: 443
          weight: 100
      mirror:
        host: microcks-svc.simulation-namespace.svc.cluster.local
        port:
          number: 9090
      mirrorPercentage:
        value: 10.0
```

> **Note:** The `mirror` and `mirrorPercentage` stanzas are temporary. They are applied during the validation window and removed once the divergence threshold is satisfied. They must never be present in a production environment overlay.

#### 7.5.3 Promotion gate

| Gate | Threshold | Window |
| --- | --- | --- |
| Minimum sample size | ≥ 500 shadowed requests | — |
| Divergence rate | < 0.5% | 1 hour rolling |
| Sustained pass | < 0.5% continuously | 24 hours |

---

## 8. Deployment Architecture

### 8.1 Namespace layout

```
cluster
├── payments-namespace
│   ├── service-a
│   ├── service-b
│   ├── service-c
│   ├── envoyfilter-inbound-capture     (non-prod only)
│   ├── envoyfilter-outbound-inject     (non-prod only)
│   ├── virtual-service-third-party-x   (non-prod only)
│   ├── virtual-service-third-party-y   (non-prod only)
│   ├── virtual-service-third-party-z   (non-prod only)
│   └── microcks-destination-rule       (non-prod only)
│
└── simulation-namespace
    ├── microcks
    └── envoyfilter-scenario-rewrite    (non-prod only)
```

### 8.2 Environment overlay matrix

| Resource | dev | qa | uat | prod |
| --- | --- | --- | --- | --- |
| envoyfilter-inbound-capture | ✓ | ✓ | ✓ | ✗ |
| envoyfilter-outbound-inject | ✓ | ✓ | ✓ | ✗ |
| virtual-service-\<third-party\> | ✓ | ✓ | ✓ | ✗ |
| microcks Deployment | ✓ | ✓ | ✓ | ✗ |
| microcks-destination-rule | ✓ | ✓ | ✓ | ✗ |
| envoyfilter-scenario-rewrite | ✓ | ✓ | ✓ | ✗ |
| Shadow mirror stanzas | ✗ | ✗ | ✓ (temp) | ✗ |

---

## 9. Operational Considerations

### 9.1 Monitoring

| Signal | Source | Alert condition |
| --- | --- | --- |
| Microcks pod health | Kubernetes readiness probe (gRPC health check) | Pod not ready for > 60s |
| Microcks 5xx rate | Istio access logs | > 1% over 5 min |
| Outbound routing anomaly | Istio access logs | Request with simulation header routed to non-Microcks destination |
| Shadow divergence rate | Comparison job metric | > 0.5% over 1h rolling window |
| Header present in prod | Custom metric from inbound Lua filter | Any non-zero count |

---

## 10. Security Considerations

### 10.1 Header stripping at production ingress

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

### 10.2 Production structural guarantees

- No `VirtualService` resources redirecting to Microcks exist in production overlays.
- No `EnvoyFilter` resources for header capture or injection exist in production overlays.
- CI policy rejects any GitOps PR adding Microcks-targeting resources to a production overlay.

### 10.3 Connection pool isolation

Test clients must use a dedicated connection pool isolated from production callers to prevent CONNECTION-scope filter state contamination.

---

## 11. Failure Modes and Mitigations

| Failure | Observed behaviour | Mitigation |
| --- | --- | --- |
| Microcks pod unavailable | Test calls receive `503 UNAVAILABLE` | `outlierDetection` in DestinationRule; `replicas: 2` in Deployment. |
| Named scenario not in Microcks | Microcks returns gRPC `NOT_FOUND` | Configure a default example per operation. |
| Header absent on third-party call | Call routes to real third party | Default routing rule handles correctly. Document which services require the propagation label. |
| Header leaks to production | Production calls virtualized | Ingress strip filter and absence of Microcks VirtualService in prod overlays provide two independent layers of protection. |
| CONNECTION-scope contamination | Wrong scenario applied to concurrent request | Isolate test traffic to dedicated connection pool. |
| Microcks scenario diverges from real | Tests pass; production calls fail | Shadow validation must pass before scenario is promoted. |

---

## 12. Decision Record

### DR-001: Single header vs multiple headers

**Decision:** Use a single header (`test-data-simulation-action-name`) whose value is the scenario name.

**Rejected alternatives:** Per-service headers and separate service-selection + scenario headers. Both require the test caller to know the internal service graph.

### DR-002: Envoy Lua filter vs application-level propagation

**Decision:** Envoy Lua `EnvoyFilter` resources. Zero application code changes.

**Rejected alternatives:** Application gRPC interceptors (requires code change) and W3C Baggage (requires OTel instrumentation universally present).

### DR-003: Microcks as the virtual gRPC backend

**Decision:** Microcks. Native gRPC support, proto as first-class artifact, Admin API for runtime updates.

**Rejected alternatives:** WireMock (less mature gRPC support); custom stub servers per service (N deployments to maintain).

---

## 13. Open Questions

| # | Question | Owner | Target |
| --- | --- | --- | --- |
| OQ-1 | Can Microcks serve stateful bidirectional streaming RPCs, or is a custom server needed? | Platform Engineering | Before first streaming RPC onboarded |
| OQ-2 | Scenario naming convention: verb-noun or structured identifier? | Test Engineering | Before first scenario authored |
| OQ-3 | Should shadow percentage be configurable per third-party service? | Platform Engineering | Before Layer 4 rollout |
| OQ-4 | Who owns scenario definitions — consumer team or platform team? | Engineering Leadership | Before shared environment deployment |

---

## 14. Appendix

### 14.1 Complete request flow

```
Test Caller
  gRPC call, metadata: test-data-simulation-action-name: payment-declined
  │
  ▼
Service A pod
  [Envoy inbound]  captures header → filter state
  Application executes
  [Envoy outbound] injects header on call to Service B
  │
  ▼
Service B pod
  [Envoy inbound]  captures header → filter state
  Application calls Third Party Y
  [Envoy outbound] injects header
  │
  ▼
Istio VirtualService for Third Party Y
  match: header present → Microcks
  │
  ▼
Microcks pod
  [Envoy inbound] rewrites: test-data-simulation-action-name → x-microcks-operation
  gRPC path: /com.thirdparty.y.FraudService/Evaluate
  Serves "payment-declined" example
  │
  ▼
Response propagates back to Test Caller
```

### 14.2 Manifest inventory

| File | Kind | Namespace | Environment |
| --- | --- | --- | --- |
| `envoyfilter-inbound-capture.yml` | EnvoyFilter | payments-namespace | dev, qa, uat |
| `envoyfilter-outbound-inject.yml` | EnvoyFilter | payments-namespace | dev, qa, uat |
| `microcks-destination-rule.yml` | DestinationRule | payments-namespace | dev, qa, uat |
| `virtual-service-<name>.yml` | VirtualService | payments-namespace | dev, qa, uat |
| `microcks-deployment.yml` | Deployment | simulation-namespace | dev, qa, uat |
| `microcks-service.yml` | Service | simulation-namespace | dev, qa, uat |
| `microcks-proto-artifacts.yml` | ConfigMap | simulation-namespace | dev, qa, uat |
| `envoyfilter-scenario-rewrite.yml` | EnvoyFilter | simulation-namespace | dev, qa, uat |
| `strip-simulation-header.yml` | EnvoyFilter | istio-ingress | prod only |

---

*Document reconstructed from screenshots (`IMG_3713.jpeg`–`IMG_3734.jpeg`) of `v1-header-driven-virtualization.md`.*
