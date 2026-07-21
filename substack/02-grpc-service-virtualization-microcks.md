# Service Virtualization for gRPC: Microcks, Outbound Routing, and the Simulation Header

**Subtitle:** Part 2 — How a third-party gRPC call becomes “real” or “virtual” without rewriting your business services. For intermediate developers.

**Part 1** covered sidecars, Istio, CRDs, and data vs control plane.  
**This part** zooms into one job: **service virtualization**—especially for **gRPC outbound calls**—and where **Microcks** (or a Microcks-shaped backend) fits.

Community teaching, not a résumé. Optional lab: [simulation-mesh-grpc](https://github.com/jkuru/simulation-mesh-grpc) (toy NFT marketplace domain—not banking/cards).

---

## 1. What “service virtualization” means here

**Service virtualization** = replace a **dependency’s behavior** in some environments with a **controllable stand-in**, while keeping:

- the **same client code path** as much as possible  
- the **same contract** (protobuf / API shape)  
- a **deliberate switch** (here: a request header = scenario)

It is **not**:

| Not this | Why |
| --- | --- |
| Unit-test mocks inside one process only | Those never exercise network, mesh, or deploy topology |
| “Fake the whole world” | We keep **internal** services real |
| Recording production secrets | Scenarios are synthetic fixtures |

### The rule of the lab (and a good platform rule)

```text
Internal services  → always real
Third-party egress → may be virtualized when a simulation header is present
```

Example chain:

```text
test-client
   → checkout-gateway      (real)
   → fraud-checker         (real)
   → RiskService           (third party: REAL or VIRTUAL)
```

Same `nft_token`, same price:

- **No** simulation header → real third-party path → e.g. APPROVED  
- Header `test-data-simulation-action-name: fraud-declined` → virtual path → DECLINED  

That single “aha” is the product of virtualization done right.

---

## 2. Why gRPC makes this interesting

### 2.1 Contracts are strong

gRPC + protobuf give you:

- a **service** (`RiskService`)  
- **RPCs** (`EvaluateRisk`)  
- **messages** (`RiskRequest` / `RiskResponse`)

Virtualization should honor that contract. The stand-in must speak the same RPC, not a random HTTP stub with a different JSON shape—unless you consciously accept contract drift.

### 2.2 Metadata is first-class

gRPC carries **metadata** (headers). Our switch is one metadata key:

```text
test-data-simulation-action-name: <scenario-name>
```

Examples of scenario names: `fraud-approved`, `fraud-declined`.

That value is **not** business logic inside checkout. It’s a **routing / fixture selector**.

### 2.3 Outbound calls are the virtualization point

In the demo, **checkout-gateway** and **fraud-checker** stay real.  
Only the **outbound** call from fraud-checker to **RiskService** is eligible for substitution.

That’s deliberate:

- You still test multi-service interaction.  
- You only fake what is expensive, flaky, or owned by someone else (third parties).

---

## 3. Why Microcks?

### 3.1 The job Microcks is good at

[**Microcks**](https://microcks.io) is an open-source tool for **API mocking / service virtualization** and contract-style artifacts. In mesh platforms people reach for it (or something like it) because:

| Need | How Microcks-shaped tools help |
| --- | --- |
| **Named scenarios** | “fraud-declined” returns a known body |
| **Contract alignment** | Import OpenAPI/AsyncAPI/protobuf-oriented mocks |
| **Shared fixture store** | QA and dev don’t each invent incompatible stubs |
| **HTTP and more** | Broader than one hard-coded fake in a single repo |

In short: Microcks is a **virtual backend product**, not a service mesh.  
The mesh (or the app) **routes** to it; Microcks **responds** with scenario data.

### 3.2 What the open lab actually runs

The public lab ships **`microcks-mock`**: a small gRPC server that **implements `RiskService`** and selects responses by scenario. It is a **teaching stand-in** for “something Microcks-like.”

Why not always embed full Microcks?

- Full Microcks is a real install (often with its own deps).  
- The **routing lessons** (header → VirtualService → backend) are the same.  
- The lab stays `make demo`-able in minutes.

Think of it as:

```text
Microcks (product)     = industrial virtual service catalog
microcks-mock (in lab) = minimal RiskService that understands scenarios
```

When `virtualBackend: microcks` is used in the operator path, the platform can also emit an EnvoyFilter that rewrites:

```text
test-data-simulation-action-name  →  x-microcks-operation
```

…because many Microcks setups select examples using **operation/dispatch headers** like `x-microcks-operation`. The teaching mock accepts both styles.

### 3.3 Why not only mock inside the fraud-checker binary?

You *can* branch in code:

```text
if header { return canned Decline } else { call real }
```

That works for unit tests. It fails as a **platform story**:

- Every language reimplements the branch.  
- Mesh-based environments can’t share one virtualization policy.  
- You never practice “app always dials the logical third-party host.”

So the lab teaches **two modes** (next section): app-local dial vs mesh routing.

---

## 4. Two modes: who decides the outbound target?

This is the heart of “how is the outbound call routed?”

### Mode A — Local / no mesh (`SIMULATION_MODE=local`)

**Who routes?** The **application** (fraud-checker).

```text
fraud-checker process
        │
        │  read simulation scenario from context (header captured by interceptor)
        │
        ├─ scenario present? ──yes──► gRPC dial microcks-mock:9090
        │                              EvaluateRisk(...)
        │
        └─ no scenario ─────────────► gRPC dial external-risk:9003
                                       EvaluateRisk(...)
```

Pseudocode of the resolver idea:

```text
if mode == local AND scenario != "" AND microcks client configured:
    use microcks client
else:
    use external-risk client
```

**Use when:** laptops, CI without Istio, `make demo`.

**Property:** virtualization works **without** a mesh.  
**Cost:** app knows about two endpoints and mode flags.

---

### Mode B — Mesh (`SIMULATION_MODE=mesh`)

**Who routes?** The **data plane** (Istio / Envoy), not the app’s if/else.

```text
fraud-checker process
        │
        │  ALWAYS dials logical third-party host:
        │     external-risk-api.com:9003
        │
        ▼
   sidecar (Envoy)
        │
        │  VirtualService on host external-risk-api.com
        │
        ├─ request has simulation header ──► microcks-svc.simulation-system:9090
        │
        └─ no header ─────────────────────► real external-risk (via ServiceEntry)
```

**Use when:** Kubernetes + Istio; platform golden path.

**Property:** **one dial target in app code** for third parties; environments differ by mesh config / CR.  
**Cost:** needs mesh objects (hand-written for learning, or **generated by an operator**).

---

## 5. Anatomy of an outbound virtualized call (mesh path)

Walk one RPC end-to-end.

### Step 1 — Client sets the simulation header

`test-client` attaches metadata when proving the simulated path:

```text
test-data-simulation-action-name: fraud-declined
```

(Empty header for the real path.)

### Step 2 — Header must hop internal services

checkout-gateway and fraud-checker should **propagate** metadata (gRPC interceptors and/or mesh filters).  
If the header dies at hop 1, the VirtualService never sees it on the outbound third-party call.

### Step 3 — fraud-checker builds a normal gRPC call

In mesh mode the app does **not** choose Microcks. It calls:

```text
RiskService.EvaluateRisk
  host: external-risk-api.com
  port: 9003
  body: { nft_token, amount_cents }
  metadata: (includes simulation header if present)
```

From the app’s point of view: “I’m calling the third party.”

### Step 4 — Sidecar intercepts outbound

Envoy in the fraud-checker pod sees the outbound connection to `external-risk-api.com`.

### Step 5 — ServiceEntry: “this host exists in the mesh model”

Without a **ServiceEntry** (or equivalent), Envoy may not treat `external-risk-api.com` as a known mesh destination with the ports/protocols you expect.

ServiceEntry answers: *What is this host? Which ports? Resolution?*  
In the lab, the “real” path often maps that logical host to the in-cluster `external-risk` service (teaching stand-in for a vendor).

### Step 6 — VirtualService: match header → pick destination

Conceptual VirtualService:

```yaml
# teaching shape — operator generates the real thing in the golden path
hosts:
  - external-risk-api.com
http:
  - name: simulate
    match:
      - headers:
          test-data-simulation-action-name:
            regex: ".+"    # any non-empty scenario
    route:
      - destination:
          host: microcks-svc.simulation-system.svc.cluster.local
          port: { number: 9090 }
  - name: real
    route:
      - destination:
          host: external-risk-api.com   # real path via ServiceEntry
          port: { number: 9003 }
```

**Order matters:** specific match (header) first; default real route second.

### Step 7 — DestinationRule: how to speak to upstream

gRPC often needs clear TLS and HTTP/2 settings in teaching clusters (e.g. plaintext inside the mesh). **DestinationRule** shapes that per host.

### Step 8 — Virtual backend responds with scenario data

**microcks-mock** (or real Microcks) implements `EvaluateRisk` and returns the fixture for `fraud-declined` (high risk score, DECLINE, factors, …).

### Step 9 — Response flows back the data plane

Envoy → fraud-checker → checkout-gateway → test-client.  
Client prints DECLINED. Same NFT and price as the APPROVED run—only the header differed.

---

## 6. Local path vs mesh path (side-by-side)

| | **Local mode** | **Mesh mode** |
| --- | --- | --- |
| App dial target | Switches: microcks **or** external | **Always** logical third-party host |
| Router | App code (`LocalRiskResolver`) | Envoy + VirtualService |
| Needs Istio? | No | Yes |
| Best for | Laptop demo, unit-adjacent e2e | Shared cluster, platform golden path |
| Risk of drift | Two dial paths in app | Mesh config must match app host names |

**Platform preference:** mesh mode for multi-team environments—so app binaries don’t embed every environment’s fake endpoints.

---

## 7. Scenarios: what Microcks (or the mock) stores

A scenario is a **named fixture**, not a free-form prompt.

Examples:

| Scenario name | Typical RiskResponse idea |
| --- | --- |
| `fraud-approved` | low score, APPROVE |
| `fraud-declined` | high score, DECLINE + factors |

In the operator/CR world, a `SimulationManifest` documents third parties + scenarios. The **mesh** uses the header value primarily as a **routing switch** (any non-empty value → virtual backend). The **virtual backend** uses the value (or rewritten operation header) to pick the **body**.

Two layers:

```text
Header value
   ├─► Mesh: "send to virtual backend?" (match on presence/value)
   └─► Microcks/mock: "which fixture to return?"
```

---

## 8. `x-microcks-operation` rewrite (real Microcks interop)

Real Microcks often dispatches mocks using headers such as:

```text
x-microcks-operation: fraud-declined
```

Your app and platform standard may use:

```text
test-data-simulation-action-name: fraud-declined
```

So the mesh can run a small **EnvoyFilter** on the virtual backend workload:

```text
on inbound to microcks:
  copy/replace simulation header → x-microcks-operation
```

Teaching `microcks-mock` accepts **either** header so local and mesh demos stay simple.

**Takeaway:** routing (where the call goes) and **dispatch** (which fixture runs) can use **related but different** headers. Platforms should document both.

---

## 9. Who creates the routing rules?

### Learning path (manual)

You apply Istio YAML yourself (VirtualService, ServiceEntry, …). Excellent for understanding.

### Golden path (platform)

App team applies only something like:

```text
kind: SimulationManifest
spec:
  thirdParties: [ external-risk host/port/... ]
  scenarios: [ fraud-declined, ... ]
  virtualBackend: teaching-mock | microcks
```

An **operator** generates VS/SE/DR/EnvoyFilters.  
That’s service virtualization as a **product**, not a YAML hobby.

---

## 10. Failure modes (intermediate debugging)

| Symptom | Likely layer |
| --- | --- |
| Always hits real third party even with header | Header not propagated; VS match wrong; app still in local mode dialing only external |
| Always hits mock | VS default wrong; header always attached in client |
| `external-risk-api.com` resolve/connect errors | ServiceEntry / DNS capture / DestinationRule |
| Mock returns wrong body | Scenario name mismatch; rewrite header missing for real Microcks |
| Works in `make demo`, fails on cluster | Mode mesh vs local; missing injection; wrong Service DNS |

Debug order:

1. **Does the outbound request carry the header?** (app logs / proxy access logs)  
2. **Which upstream did Envoy choose?** (data plane)  
3. **Do VS/SE/DR exist and match the host the app dials?** (config objects)  
4. **Is the virtual backend healthy and implementing the same gRPC service?**  

---

## 11. Design principles worth stealing

1. **Virtualize at the third-party boundary**, not the whole graph.  
2. **One switch** (header/scenario) visible in traces.  
3. **Same protobuf contract** on real and virtual backends.  
4. Prefer **mesh routing** for shared environments; keep **local dial switch** for laptop speed.  
5. Treat Microcks as the **fixture engine**; treat Istio as the **router**.  
6. Generate mesh config from intent so teams don’t fork EnvoyFilter snippets.

---

## 12. Try it

```bash
# Local virtualization (app routes to microcks-mock when header set)
make demo

# Offline platform checks
make platform-accept

# Full mesh golden path (operator generates routes)
CLUSTER=servicemesh KEEP_CLUSTER=1 make platform-e2e
```

Repo: [github.com/jkuru/simulation-mesh-grpc](https://github.com/jkuru/simulation-mesh-grpc)

Read next in-repo:

- `docs/GOLDEN_PATH.md`  
- `docs/SYSTEM_CONTEXT.md`  
- `apps/virtualization-framework/docs/MICROCKS.md`  

---

## Closing

Service virtualization for gRPC is not “random mocking.” It’s a deliberate split:

> **Apps call contracts.  
> Meshes route by policy (and headers).  
> Microcks-like systems serve scenario fixtures.  
> Platforms turn that into a golden path.**

If Part 1 was “what are sidecars and CRDs?”, Part 2 is “**where does this outbound RPC go, and why?**”

When that picture is solid, “we’ll virtualize the vendor in QA” stops being a hand-wave and becomes an architecture you can draw on a whiteboard.

---

### Suggested Substack tags

`grpc` · `service-virtualization` · `microcks` · `istio` · `kubernetes` · `platform-engineering` · `testing` · `open-source`

### Series

- **Part 1:** [Sidecars, Istio, CRDs, data/control plane](./01-mesh-data-control-k8s.md)  
- **Part 2 (this post):** gRPC virtualization, Microcks, outbound routing  
- **Part 3:** [The framework vs industry patterns](./03-virtualization-framework-vs-industry.md)  

### Footer

Educational community writing. Demo domain is a toy NFT marketplace; not a production financial system.
