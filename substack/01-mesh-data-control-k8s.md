# From Mobile to Platform: Istio, Sidecars, CRDs, and Kubernetes—A Deep Map for Intermediate Developers

**Subtitle:** You already ship APIs or clients. Here’s how service mesh data planes, control planes, and custom resources actually fit together—without assuming you’re a mesh specialist.

---

You don’t need another “Hello Kubernetes” for absolute beginners. You also don’t need a 400-page Istio manual.

This is for **intermediate developers**: you can ship services or mobile/API clients, you’ve seen YAML or Docker, and words like “sidecar” or “CRD” still feel fuzzy when someone says them in a design review.

I’m writing this to **give back a clear map**—the one I wish existed when I moved from building clients and APIs toward platform-style systems. Not a résumé. Not a product pitch. A mental model you can reuse.

Optional companion lab (toy **NFT marketplace** demo domain—not banking/cards):  
[github.com/jkuru/simulation-mesh-grpc](https://github.com/jkuru/simulation-mesh-grpc)

---

## 0. The problem in one picture

Without shared infrastructure, every service re-solves:

```text
App code = business logic
         + retries
         + timeouts
         + TLS
         + routing hacks for “test env”
         + half-broken metrics
```

A **service mesh** tries to pull the network cross-cutting concerns **out of app code** and into a **consistent data plane**, configured by a **control plane**, running on **Kubernetes**.

Three terms intermediate devs must own:

| Term | Plain meaning |
| --- | --- |
| **Sidecar** | A helper process **in the same pod** as your app; usually a proxy that sees your traffic |
| **Istio** | A popular service mesh: Envoy sidecars + a control plane + Kubernetes custom resources for policy |
| **CRD** | A way to **extend the Kubernetes API** with new object types your controllers understand |

Everything below deepens those three, then connects them.

---

## 1. Sidecars — deep dive

### 1.1 What a sidecar is (and is not)

In Kubernetes, a **Pod** is the atomic runnable unit: one or more containers that share:

- network namespace (same IP from inside the pod model)  
- storage volumes (when configured)  
- lifecycle coupling (pod starts/stops as a unit)

A **sidecar** is simply **another container in that pod**, with a supporting role.

```text
┌────────────────────────── Pod ──────────────────────────┐
│  IP: 10.x.x.x                                           │
│                                                         │
│   ┌─────────────────┐         ┌──────────────────────┐  │
│   │ container: app  │◄───────►│ container: sidecar   │  │
│   │ checkout-gw     │  local  │ envoy proxy          │  │
│   │ :9001           │  network│ (iptables/redirect   │  │
│   └─────────────────┘         │  or CNI integration) │  │
│                               └──────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

**Is not:**

- A second microservice you call over the cluster network by name (that would be another Deployment)  
- Automatically “free” (CPU/RAM, complexity, upgrade burden)  
- Magic that fixes bad app protocols  

**Is:**

- A colocated process that can intercept or mediate traffic  
- The usual home of the mesh **data plane** proxy (**Envoy** in Istio)

### 1.2 Why put the proxy in the pod?

Three practical reasons:

1. **Identity per workload** — policy can be “this workload may talk to that workload.”  
2. **No app rewrite** — app still dials `fraud-checker:9002`; the proxy applies mTLS, retries, routing.  
3. **Uniformity** — every language team gets the same network behavior.

### 1.3 How traffic gets into the sidecar (mental model)

Exact mechanism depends on mesh version and config (iptables redirect, eBPF, CNI plugins, “ambient” modes without per-pod sidecars, etc.). For classical Istio sidecar mode, think:

> Packets leaving or entering the app container are **redirected** to Envoy.  
> Envoy applies filters/routes, then forwards.

You rarely configure iptables by hand. You need to know **why** “localhost” and “outbound” behavior can surprise you in a meshed pod.

### 1.4 What Envoy actually does on a request (simplified)

For an outbound gRPC/HTTP call:

1. App opens connection to destination host:port.  
2. Sidecar receives the connection (via redirect).  
3. Envoy matches **listeners / routes / clusters** (config pushed by the control plane).  
4. Policies apply: timeouts, retries, TLS origination, header matches, fault injection, …  
5. Envoy connects to the real upstream (another pod’s sidecar or Service).

Inbound is symmetric: traffic hits Envoy first, then your app port.

### 1.5 Sidecar and header propagation (why demos care)

Suppose simulation needs a header to survive:

```text
test-client → checkout-gateway → fraud-checker → external-risk
```

If only the first hop has the header, and apps don’t forward metadata, the last hop never sees it.

Two complementary approaches:

| Approach | Where it lives |
| --- | --- |
| **App interceptors** | gRPC middleware copies header in/out (works without mesh too) |
| **Mesh filters** | Envoy/Lua (or other filters) capture/inject headers for labeled pods |

A solid platform often supports **both**: app-level for correctness in local mode; mesh-level for mesh-native behavior.

### 1.6 Sidecar costs intermediate devs should respect

- **Resources:** extra container per pod  
- **Cold start / readiness:** pod isn’t “ready” until proxy is ready (configure probes thoughtfully)  
- **Debugging:** is the bug in app, sidecar config, or control plane push?  
- **Version skew:** app teams don’t pick Envoy version; platform does  

**Sidecar takeaway:** the sidecar is the **data-plane worker** sitting beside your process. Master “request goes through proxy” before memorizing CRD field names.

---

## 2. Data plane vs control plane (mesh edition)

Intermediate developers often conflate “Istio” with “the YAML I applied.” Split it:

| Plane | Job | Examples |
| --- | --- | --- |
| **Data plane** | Forward and enforce on **live requests** | Envoy sidecars, outbound clusters, L7 routes |
| **Control plane** | **Compute and distribute config** to proxies; not your checkout QPS path | istiod, xDS config push, pilot-like components |

```text
                 ┌──────────────────── control plane ────────────────────┐
                 │  istiod / config discovery                             │
                 │  “here are routes, certs, destinations for each proxy” │
                 └─────────────────────────┬──────────────────────────────┘
                                           │ xDS (config APIs)
                    ┌──────────────────────┼──────────────────────┐
                    ▼                      ▼                      ▼
              Envoy (pod A)          Envoy (pod B)          Envoy (pod C)
                    │                      │                      │
                    └──────────── live requests (data plane) ─────┘
```

**Debug question that saves hours:**

- Symptom is wrong route for *this request* → start with **data plane** (config arrived? match conditions? destination healthy?)  
- Symptom is “nobody has config” / mass rejects after install → start with **control plane** and Kubernetes API objects  

---

## 3. Istio — deep dive for intermediate developers

### 3.1 What Istio is

**Istio** is a service mesh implementation that typically provides:

1. **Data plane:** Envoy sidecars (classic model)  
2. **Control plane:** istiod (aggregation of older split components)  
3. **Kubernetes integration:** install hooks, injection, and **Custom Resources** for traffic and security policy  
4. **Optional:** ingress/egress gateways (Envoy at the edge of the mesh)

Istio is not Kubernetes. It **runs on** Kubernetes (most common) and **extends** it.

### 3.2 Sidecar injection

How does Envoy get into your pod?

**Automatic injection (common):**  
Namespace or pod labeled for injection. When the Pod is created, a **mutating admission webhook** adds the Envoy container + init/redirect setup to the pod spec.

**Manual injection:**  
`istioctl kube-inject` rewrites YAML before apply (less common day-to-day).

Intermediate insight: **injection is admission + templating**, not something your app binary does.

### 3.3 Core Istio traffic objects (the ones you’ll actually meet)

You don’t need every CRD on day one. Learn this set:

#### VirtualService

**Intent:** “Given traffic for these hosts, match conditions, send to these destinations (with weights, timeouts, retries, headers matches).”

Mental model: **L7 routing table** for mesh traffic.

Example idea (not full YAML):

> For host `external-risk-api.com`,  
> if header `test-data-simulation-action-name` is set → route to microcks service  
> else → route to real external-risk backend.

#### DestinationRule

**Intent:** “Once I know the destination service/host, how do I talk to it?”  
Subsets (versions), load balancing, TLS mode to upstream, connection pools.

Often paired with VirtualService: VS picks destination; DR defines **how** to contact that destination’s endpoints.

#### ServiceEntry

**Intent:** “Register a host that is **not** a normal in-cluster Service—or shape how external hosts appear to the mesh.”

Critical for third-party style hostnames (`external-risk-api.com`) so Envoy has a cluster definition and DNS/resolution behavior the mesh understands.

#### PeerAuthentication / AuthorizationPolicy

**Intent:** mTLS mode between workloads; who is allowed to call whom.  
Security plane of the mesh (still enforced on the data plane using identities from the control plane).

#### EnvoyFilter (power tool—handle carefully)

**Intent:** patch Envoy config directly (listeners, filters, Lua, etc.).

Use cases in teaching labs: header capture/inject via Lua.  
In production platforms: **high support cost**—prefer higher-level APIs when possible. Many platforms **generate** EnvoyFilters centrally rather than letting every team write them.

### 3.4 How a request uses these objects (sequence)

```text
1. fraud-checker app dials external-risk-api.com:9003
2. Sidecar intercepts outbound call
3. Envoy uses config derived from:
     ServiceEntry  → “this host exists in the mesh model”
     VirtualService → “which upstream for this match?”
     DestinationRule → “TLS/load-balancing to that upstream”
4. Bytes go to chosen endpoints (real pod or virtual backend)
```

**Platform angle:** app teams think in *intent* (“virtualize third party X for scenario Y”).  
Platform turns intent into VS/SE/DR/EF so teams don’t each become Envoy experts.

### 3.5 Istio control plane push (xDS)

Envoy does not poll your VirtualService YAML files from disk. Roughly:

1. You apply Kubernetes objects (VS, DR, …).  
2. istiod watches them (+ Services, Endpoints, …).  
3. istiod computes per-proxy configuration.  
4. Config is pushed to Envoy via **xDS** APIs (discovery services).  

So “I applied YAML” ≠ “every Envoy already has it” if control plane or injection is broken—but when healthy, it’s continuous reconciliation.

### 3.6 Istio takeaway for intermediate devs

| Concept | You should be able to say |
| --- | --- |
| Sidecar | Envoy beside app; data plane |
| istiod | Control plane programming Envoys |
| VirtualService | L7 match → route |
| DestinationRule | How to talk to a destination |
| ServiceEntry | Bring external/logical hosts into the mesh model |
| EnvoyFilter | Low-level Envoy patches; powerful, sharp |

---

## 4. CRDs — deep dive for intermediate developers

### 4.1 Kubernetes API is a sea of types

Out of the box you get built-in types: `Pod`, `Deployment`, `Service`, `ConfigMap`, …

The API server stores objects and runs **admission**, **validation**, **RBAC**, and **watch** semantics the same way for those types.

### 4.2 What a CRD is

A **Custom Resource Definition** registers a **new type** with the API server, for example:

```text
apiVersion: simulation.io/v1alpha1
kind: SimulationManifest
```

After the CRD exists:

```bash
kubectl get simulationmanifests
# or short name, if defined: kubectl get simm
```

A **Custom Resource (CR)** is one instance of that type (like one Deployment instance).

**CRD = schema + API registration.**  
**CR = data (your object).**

### 4.3 Why platforms love CRDs

Because they turn platform products into **Kubernetes-native APIs**:

| Without CRD | With CRD |
| --- | --- |
| Wiki + ticket + hand YAML | `kubectl apply -f my-intent.yaml` |
| Every team copies EnvoyFilter snippets | Team applies one intent object |
| No shared schema | OpenAPI schema, validation, versioning (`v1alpha1`) |
| Hard to RBAC | Grant `create` on `simulationmanifests` only |

This is the same instinct as designing an external HTTP API for product teams—except the client is `kubectl`/GitOps and the server is the cluster.

### 4.4 CRD is not a controller

This is the #1 intermediate confusion.

| Piece | Responsibility |
| --- | --- |
| **CRD** | API exists; objects can be stored |
| **Controller / operator** | Code that **reacts** to those objects and does work |

If you apply a CRD and a CR but **no controller** is running, Kubernetes will happily store your object… and **nothing will happen**. Desired state with no reconciler.

```text
kubectl apply SimulationManifest
        │
        ▼
   etcd (stored)  ──watch──►  operator  ──create/update──►  VirtualService, ServiceEntry, …
        │                         │
        │                         └── writes status.phase=Ready
        ▼
   kubectl get simm   # shows Ready when operator is healthy
```

### 4.5 Spec vs status

Healthy custom resources split:

- **spec** — what the user wants (desired)  
- **status** — what the operator observed (actual/summary)

Example phases: `Pending`, `Ready`, `Error`, `Forbidden`.

App teams should learn to read **status** like they read HTTP error bodies.

### 4.6 Reconciliation loop (operator pattern)

A controller roughly:

```text
for each relevant object change (or period):
  read object
  compute desired child resources
  create/update/delete children
  update status
  return (maybe requeue)
```

Idempotency matters: reconcile may run many times.

### 4.7 Admission webhooks vs CRD OpenAPI validation

| Layer | When | Examples |
| --- | --- | --- |
| **CRD schema** (OpenAPI) | Structural: types, required fields, enums | `port` must be int; `thirdParties` minItems 1 |
| **Validating webhook** | Richer policy in code | “prod forbids this”, “response keys must match third party names” |
| **Mutating webhook** | Defaults / rewrites | fill `microcksService` if empty |

Intermediate rule: **schema catches shape; webhooks catch policy and defaults.**

### 4.8 API versions (`v1alpha1`, `v1beta1`, `v1`)

Custom resources version like any API:

- **alpha** — may break; for experiments  
- **beta** — more stable; still evolving  
- **v1** — stability expectations rise  

Platform owners version CRDs deliberately; app teams pin docs to a version.

### 4.9 CRD takeaway

| Phrase | Meaning |
| --- | --- |
| CRD | Extension of Kubernetes API |
| CR | Instance of that API |
| Operator | Controller implementing the type’s behavior |
| Golden path | Often “apply this CR” instead of “edit five mesh objects” |

---

## 5. Kubernetes pieces that make the above real

Quick precision for intermediate folks (not a full K8s course):

| Object | Role in this story |
| --- | --- |
| **Deployment** | Runs app (+ injected sidecar) |
| **Service** | Stable DNS to pods (`fraud-checker.poc.svc`) |
| **Namespace** | Scope for teams/env |
| **Label** | Selection for injection, EnvoyFilter workloadSelector, ops |
| **ServiceAccount + RBAC** | Who the operator is; who may create CRs |
| **MutatingWebhookConfiguration** | Injection + defaulting |
| **ValidatingWebhookConfiguration** | Policy |

**Desired state machine:** you declare; controllers converge. Mesh and operators are the same philosophy at higher layers.

---

## 6. One vertical slice (how it all connects)

Teaching scenario from the lab:

```text
Same nft_token + price
  A) no header     → data plane to real external-risk → APPROVED (demo)
  B) fraud-declined → data plane to virtual backend   → DECLINED (demo)
```

**Who does what?**

| Layer | Responsibility |
| --- | --- |
| **App** | Propagate header (interceptors); dial logical third-party host in mesh mode |
| **Sidecar (data plane)** | Match routes; send to real vs virtual upstream |
| **Istio objects** | VS/SE/DR/EF encode those rules |
| **Operator + CRD** | Generate those objects from `SimulationManifest` intent |
| **Admission** | Default and validate the CR before it lands |
| **istiod** | Push resulting config into Envoys |

Hand-written Istio YAML is excellent for **learning**.  
For a **supported platform**, generate it from intent so app teams don’t own EnvoyFilter archaeology.

---

## 7. How to practice (community give-back path)

1. **Explain sidecar to a rubber duck** — pod, two containers, traffic through proxy.  
2. **Apply a VirtualService** in a dev cluster and change a header match; watch behavior.  
3. **kubectl get crd** and read one CR’s spec/status.  
4. **Find the operator pod**; delete a generated child object and see reconcile recreate it (in a safe lab).  
5. **Draw** data plane vs control plane for your own service.  

If you use the open lab:

```bash
make demo                 # local aha: header changes outcome
make platform-accept      # tests + goldens offline
# later:
CLUSTER=servicemesh KEEP_CLUSTER=1 make platform-e2e
```

Repo: [github.com/jkuru/simulation-mesh-grpc](https://github.com/jkuru/simulation-mesh-grpc)

---

## 8. Glossary (pin this)

| Term | One-line definition |
| --- | --- |
| **Sidecar** | Helper container in the same pod; often Envoy |
| **Envoy** | Proxy implementing mesh data plane |
| **Data plane** | Path of live requests |
| **Control plane** | System that configures data plane components |
| **Istio** | Mesh providing Envoy + istiod + K8s traffic APIs |
| **VirtualService** | L7 routing rules inside the mesh |
| **DestinationRule** | Policies for talking to a destination |
| **ServiceEntry** | Registers external/logical hosts into the mesh |
| **EnvoyFilter** | Low-level Envoy config patch |
| **CRD** | Custom Kubernetes API type |
| **Custom Resource** | Object instance of a CRD |
| **Operator** | Controller reconciling CRs into real resources |
| **xDS** | Config discovery APIs Envoy uses to receive updates |
| **Golden path** | Supported short path for app teams |

---

## Closing

Sidecars, Istio, and CRDs are not three random buzzwords. They’re one stack of ideas:

> **CRDs express intent on Kubernetes → operators and Istio’s control plane turn intent into proxy config → sidecars enforce it on the data plane.**

If you internalize that sentence, design reviews get easier—whether you stay an app engineer or lean into platform work.

If this map helped, pass it to someone who’s been nodding along in mesh meetings without a picture in their head. That’s the whole reason to write it.

---

### Suggested Substack tags

`kubernetes` · `istio` · `service-mesh` · `envoy` · `platform-engineering` · `devops` · `software-engineering` · `open-source`

### Series

- **Part 1 (this post):** Sidecars, Istio, CRDs, data/control plane  
- **Part 2:** [Service virtualization for gRPC + Microcks + outbound routing](./02-grpc-service-virtualization-microcks.md)  
- **Part 3:** [The framework vs industry patterns](./03-virtualization-framework-vs-industry.md)

### Footer

Educational community writing. Companion lab is a learning project (toy NFT marketplace domain), not a production financial system.
