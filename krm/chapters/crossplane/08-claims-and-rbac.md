# Chapter 08: Namespace Isolation & RBAC

> **You will build:** Namespace isolation - developers can only touch their own XRs, tested with `kubectl auth can-i`.

So far every `WebService` you have created has lived in the `default` namespace and your `kubectl` user has full cluster access. Real platforms need team isolation: Team A can only see and create resources in their namespace; Team B cannot touch Team A's services.

This chapter covers:
1. How Crossplane's `Namespaced` scope (already in your XRD) works - each team creates XRs in their own namespace
2. How to create team namespaces with RBAC that limits what each team can do
3. How to verify isolation with `kubectl auth can-i`

---

## The History of Claims (v1 vs v2)

**Crossplane v1 (older):** Had a distinction between:
- **Composite Resources (XRs):** Cluster-scoped, owned by the platform team
- **Claims:** Namespace-scoped wrappers that developers created

Each XRD had to define a `claimNames` block, and there were two separate Kinds.

**Crossplane v2 (current - used in this repo):** The XRD has `spec.scope: Namespaced`. There is only one resource Kind, and it lives in a namespace. The separation is simpler - you just apply an XR object into a team's namespace. No separate Claim Kind is needed.

Your `xrd.yaml` already uses `scope: Namespaced`. So any `WebService` deployed into a namespace is inherently scoped to that namespace.

---

## RBAC in Kubernetes

RBAC (Role-Based Access Control) answers: **who can do what to which resources?**

The building blocks:

| Object | Scope | What it does |
|--------|-------|-------------|
| `Role` | Namespace | Defines permitted verbs (get, list, create, delete) on resources |
| `ClusterRole` | Cluster | Same as Role but cluster-wide (or reusable across namespaces) |
| `RoleBinding` | Namespace | Binds a Role (or ClusterRole) to a user/group/ServiceAccount in a namespace |
| `ClusterRoleBinding` | Cluster | Binds a ClusterRole to a subject for the entire cluster |

### Example: Team Developer Role

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: team-alpha
  name: webservice-developer
rules:
# Team developers can create, view, and delete WebService XRs in their namespace
- apiGroups: ["platform.example.io"]
  resources: ["webservices"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
# They can read the Deployments and Services created from their XRs
- apiGroups: ["apps"]
  resources: ["deployments"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["services", "configmaps"]
  verbs: ["get", "list", "watch"]
# They CANNOT create raw Deployments - only WebService XRs
```

The key platform engineering principle: **developers interact only with the custom XR API, not raw Kubernetes resources.**

---

## Crossplane's Own RBAC

Crossplane creates ClusterRoles automatically when you apply an XRD. You can see them:

```bash
kubectl get clusterroles | grep crossplane
```

These roles let Crossplane's own service accounts reconcile XRs across namespaces. Your RBAC work in this chapter is about **restricting human developers**, not Crossplane itself.

---

## Hands-On: Multi-Team Namespace Isolation

You will create two team namespaces (`team-alpha` and `team-beta`) with RBAC, then deploy `WebService` resources as each team and verify isolation.

```bash
mkdir -p practice/ch08
```

### Step 1: Ensure the XRD and Composition Are Applied

```bash
kubectl apply -f practice/ch05/webservice-xrd.yaml
kubectl apply -f practice/ch04/function-go-templating.yaml
kubectl apply -f practice/ch06/webservice-composition.yaml

kubectl get xrds
kubectl get functions.pkg.crossplane.io
# Both should show ready state
```

### Step 2: Create Team Namespaces

Create `practice/ch08/namespaces.yaml`:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: team-alpha
  labels:
    team: alpha
    managed-by: platform
---
apiVersion: v1
kind: Namespace
metadata:
  name: team-beta
  labels:
    team: beta
    managed-by: platform
```

Apply:

```bash
kubectl apply -f practice/ch08/namespaces.yaml
kubectl get namespaces | grep team-
```

### Step 3: Create Roles for Each Team

Create `practice/ch08/rbac.yaml`:

```yaml
# ── Role (namespace-scoped) for team-alpha ────────────────────────────────────
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: team-alpha
  name: webservice-developer
rules:
- apiGroups: ["platform.example.io"]
  resources: ["webservices"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["apps"]
  resources: ["deployments"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["services", "configmaps", "pods"]
  verbs: ["get", "list", "watch"]
---
# ── Role (namespace-scoped) for team-beta - identical permissions, different namespace ──
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: team-beta
  name: webservice-developer
rules:
- apiGroups: ["platform.example.io"]
  resources: ["webservices"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["apps"]
  resources: ["deployments"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["services", "configmaps", "pods"]
  verbs: ["get", "list", "watch"]
---
# ── RoleBinding: bind "alice" (ServiceAccount) to team-alpha's Role ──────────
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  namespace: team-alpha
  name: alice-webservice-developer
subjects:
- kind: ServiceAccount
  name: alice
  namespace: team-alpha
roleRef:
  kind: Role
  name: webservice-developer
  apiGroup: rbac.authorization.k8s.io
---
# ── RoleBinding: bind "bob" (ServiceAccount) to team-beta's Role ─────────────
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  namespace: team-beta
  name: bob-webservice-developer
subjects:
- kind: ServiceAccount
  name: bob
  namespace: team-beta
roleRef:
  kind: Role
  name: webservice-developer
  apiGroup: rbac.authorization.k8s.io
---
# ── ServiceAccounts representing two developers ───────────────────────────────
apiVersion: v1
kind: ServiceAccount
metadata:
  name: alice
  namespace: team-alpha
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: bob
  namespace: team-beta
```

Apply:

```bash
kubectl apply -f practice/ch08/rbac.yaml
kubectl get roles,rolebindings,serviceaccounts -n team-alpha
kubectl get roles,rolebindings,serviceaccounts -n team-beta
```

### Step 4: Deploy a WebService as Team Alpha

Create `practice/ch08/team-alpha-service.yaml`:

```yaml
apiVersion: platform.example.io/v1alpha1
kind: WebService
metadata:
  name: alpha-frontend
  namespace: team-alpha
spec:
  image: nginx:alpine
  replicas: 1
  port: 80
  environment: staging
  config:
    APP_TEAM: alpha
    LOG_LEVEL: info
```

Apply:

```bash
kubectl apply -f practice/ch08/team-alpha-service.yaml
```

See the resources:

```bash
kubectl get deployments,services,configmaps -n team-alpha
```

### Step 5: Deploy a WebService as Team Beta

Create `practice/ch08/team-beta-service.yaml`:

```yaml
apiVersion: platform.example.io/v1alpha1
kind: WebService
metadata:
  name: beta-api
  namespace: team-beta
spec:
  image: nginx:alpine
  replicas: 2
  port: 8080
  environment: production
  config:
    APP_TEAM: beta
    LOG_LEVEL: warn
```

Apply:

```bash
kubectl apply -f practice/ch08/team-beta-service.yaml
kubectl get deployments,services -n team-beta
```

### Step 6: Verify Namespace Isolation

List all WebServices across all namespaces:

```bash
kubectl get webservices -A
```

Expected output:

```
NAMESPACE    NAME             SYNCED   READY   ...
team-alpha   alpha-frontend   True     True
team-beta    beta-api         True     True
```

Each team's resource is in its own namespace. Now verify team-alpha cannot see team-beta's resources:

```bash
# Simulate alice (team-alpha ServiceAccount) trying to list team-beta's WebServices
kubectl auth can-i list webservices \
  --namespace team-beta \
  --as system:serviceaccount:team-alpha:alice
```

Expected: `no`

```bash
# Alice CAN list WebServices in her own namespace
kubectl auth can-i list webservices \
  --namespace team-alpha \
  --as system:serviceaccount:team-alpha:alice
```

Expected: `yes`

```bash
# Alice CANNOT create raw Deployments (only WebService XRs)
kubectl auth can-i create deployments \
  --namespace team-alpha \
  --as system:serviceaccount:team-alpha:alice
```

Expected: `no`

This is the key platform engineering constraint: developers use the `WebService` API, not raw Kubernetes resources. The platform team controls deployment standards through the Composition.

### Step 7: Clean Up

```bash
kubectl delete -f practice/ch08/team-alpha-service.yaml
kubectl delete -f practice/ch08/team-beta-service.yaml
```

Leave the namespaces and RBAC in place - Chapter 09 will reuse them.

---

## What You Built

- Two team namespaces with Roles that grant access only to the `WebService` API
- Verified that ServiceAccounts representing developers cannot cross namespace boundaries
- Verified that developers cannot create raw Kubernetes objects (enforcing platform standards)
- Deployed the same `WebService` XRD in two different namespaces, each fully isolated

The next chapter dives deeper into Go templating: conditional HPA creation, environment-based replica scaling, and reading observed state to implement idempotent updates.

---

➡️ [Crossplane 09: Advanced Go Templating](09-advanced-go-templating.md)
