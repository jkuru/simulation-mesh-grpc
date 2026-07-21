# RBAC matrix — virtualization-framework

Platform engineering practice: **least privilege by persona**, not one cluster-admin
binding for everyone.

Installed ClusterRoles (labels `simulation.io/rbac-persona`):

| Persona | ClusterRole | Bound to | Purpose |
| --- | --- | --- | --- |
| **Operator** | `virtualization-framework-manager` | SA `virtualization-framework` in `simulation-system` | Reconcile CRs → Istio objects, status, Events |
| **App team** | `simulation-app-team-editor` | *your* RoleBinding per app namespace | Author `SimulationManifest` only |
| **Platform admin** | `simulation-platform-admin` | Platform engineers (ClusterRoleBinding) | Install/upgrade operator, CRD, webhooks |

Sample RoleBinding: [`config/samples/app-team-rolebinding-example.yaml`](../config/samples/app-team-rolebinding-example.yaml)

---

## Permission matrix

| Capability | App team | Operator SA | Platform admin |
| --- | --- | --- | --- |
| Create/update/delete `SimulationManifest` (own ns) | **Yes** | No (watch/update finalizers only) | Yes (cluster) |
| Read `SimulationManifest` status | **Yes** | Yes | Yes |
| Create/update Istio VS/SE/DR/EnvoyFilter | **No** (read optional) | **Yes** (managed objects) | Yes (break-glass) |
| Install CRD / operator / webhook | No | No | **Yes** |
| Write Events on CRs | No | **Yes** | Yes |
| Scrape operator metrics | No (cluster scrape SA) | Exposes `:8080` | Yes |

---

## Why this split

1. **Golden path** for app teams is *only* the CR — not raw EnvoyFilter YAML.  
2. The operator is the **only** writer of mesh routing for simulation.  
3. Platform admins own install and break-glass; they are not the day-to-day CR authors.

---

## Bind app teams (example)

```bash
# ClusterRole is installed with the framework
kubectl apply -f config/samples/app-team-rolebinding-example.yaml
# edit subjects: Group or ServiceAccount for the team
```

```yaml
kind: RoleBinding
metadata:
  name: simulation-app-team-editor
  namespace: poc
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: simulation-app-team-editor
subjects:
  - kind: Group
    name: poc-developers
```

---

## Verify

```bash
kubectl get clusterrole -l app.kubernetes.io/name=virtualization-framework
kubectl describe clusterrole simulation-app-team-editor
kubectl auth can-i create simulationmanifests -n poc --as=system:serviceaccount:poc:deploy
```
