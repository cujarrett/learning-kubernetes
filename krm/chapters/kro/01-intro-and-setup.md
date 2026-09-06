# kro 01: Intro, Setup & Your First ResourceGraphDefinition

> **You will build:** kro installed on minikube; a `WebService` ResourceGraphDefinition that produces a Deployment and Service from a single user-facing CR.

---

## What Is kro?

kro (Kube Resource Orchestrator) is a Kubernetes SIG Cloud Provider project that lets you define a custom API by writing a single YAML file - a **ResourceGraphDefinition (RGD)**. You describe the schema users interact with and the Kubernetes resources each instance should produce. kro parses your CEL expressions, infers the dependency graph, generates a CRD, and stands up a controller - all at install time without any Go code.

```
You write one RGD YAML
        │
        ▼
kro installs → generates a CRD + controller for your type
        │
        ▼
User applies a CR of that type
        │
        ▼
kro reconciles the downstream resources (Deployment, Service, …)
```

---

## kro vs Crossplane at a Glance

Both tools solve the same core problem: define a custom Kubernetes API that produces downstream resources. They take different approaches:

| | kro | Crossplane |
|---|---|---|
| **Definition type** | `ResourceGraphDefinition` (one object) | `XRD` (schema) + `Composition` (resources) |
| **Expression language** | CEL (`${service.status.ip}`) | Go templates / CEL (pipeline functions) |
| **Dependency ordering** | Inferred automatically from CEL expressions | Explicit via Function pipeline steps |
| **Custom logic** | CEL only (non-Turing-complete) | Full Go functions possible |
| **Provider ecosystem** | Works with any CRD (ACK, ASO, etc.) | Upbound provider packages |
| **Schema syntax** | SimpleSchema (`string \| default=nginx`) | OpenAPI v3 (verbose but precise) |
| **Origin** | Kubernetes SIG project | CNCF project (Upbound) |

Use kro when your composition logic fits CEL and you want the simplest possible authoring experience. Use Crossplane when you need Go functions, provider packages with their own CRD libraries, or detailed revision tracking.

---

## Prerequisites

- Foundations Chapter 01 read - you understand CRDs, CRs, and the reconciliation loop
- minikube running (`minikube start`)
- `kubectl` configured to talk to minikube
- `helm` installed

---

## Install kro

```bash
# Add the kro Helm chart repo
helm repo add kro https://kro.run/charts && helm repo update

# Install into the kro-system namespace
helm install kro kro/kro \
  --namespace kro-system \
  --create-namespace \
  --wait

# Verify the controller is running
kubectl get pods -n kro-system
# NAME                   READY   STATUS    RESTARTS   AGE
# kro-xxxxx-xxxxx        1/1     Running   0          30s

# Verify the RGD CRD was installed
kubectl get crd resourcegraphdefinitions.kro.run
```

---

## Anatomy of a ResourceGraphDefinition

An RGD has two top-level parts under `spec`:

```
spec
├── schema        ← defines your custom API surface (what users write)
│     ├── apiVersion
│     ├── kind
│     ├── spec    ← input fields (SimpleSchema syntax)
│     └── status  ← output fields written back (CEL expressions)
│
└── resources     ← the downstream resources each instance produces
      ├── id      ← reference name used by CEL expressions
      └── template ← the Kubernetes resource manifest, fields can reference CEL
```

### SimpleSchema

Instead of OpenAPI boilerplate, kro uses a terse inline syntax:

```yaml
spec:
  schema:
    spec:
      image: string | default=nginx         # type, with default
      replicas: integer | default=1 minimum=1 maximum=20
      port: integer | default=80 required=true
      enableMetrics: boolean | default=false
```

| Modifier | Meaning |
|----------|---------|
| `default=<value>` | Value if the user omits the field |
| `required=true` | kro rejects the CR if field is missing |
| `minimum=` / `maximum=` | Numeric bounds |

### CEL Expressions

CEL expressions appear inside `${}` in resource templates and status fields. They reference:
- `schema.spec.<field>` - inputs from the user's CR
- `schema.metadata.<field>` - name, namespace, labels
- `<resourceId>.status.<field>` - status from another resource in the RGD

```yaml
# In a resource template
image: ${schema.spec.image}
name: ${schema.metadata.name}-deployment

# In a status field (reads output of another resource)
status:
  clusterIP: ${service.status.clusterIP}
```

kro reads all expressions, builds a dependency graph, and waits for upstream resources to exist before reconciling downstream ones. You never declare order.

---

## Hands-On: Your First RGD

### Step 1 - Write the RGD

Create `practice/kro/ch01/webservice-rgd.yaml`:

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: webservice
spec:
  schema:
    apiVersion: kro.run/v1alpha1
    kind: WebService
    spec:
      image: string | default=nginx
      port: integer | default=80
      replicas: integer | default=1 minimum=1
    status:
      clusterIP: ${service.status.clusterIP}
      ready: ${deployment.status.readyReplicas == schema.spec.replicas}

  resources:

  - id: deployment
    template:
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        name: ${schema.metadata.name}
      spec:
        replicas: ${schema.spec.replicas}
        selector:
          matchLabels:
            app: ${schema.metadata.name}
        template:
          metadata:
            labels:
              app: ${schema.metadata.name}
          spec:
            containers:
            - name: app
              image: ${schema.spec.image}
              ports:
              - containerPort: ${schema.spec.port}

  - id: service
    template:
      apiVersion: v1
      kind: Service
      metadata:
        name: ${schema.metadata.name}
      spec:
        selector: ${deployment.spec.selector.matchLabels}
        ports:
        - port: ${schema.spec.port}
          targetPort: ${schema.spec.port}
```

Notice `service` references `deployment.spec.selector.matchLabels`. kro sees that expression, infers `service` depends on `deployment`, and creates the Deployment first.

### Step 2 - Apply the RGD

```bash
kubectl apply -f practice/kro/ch01/webservice-rgd.yaml

# kro generates a CRD for your WebService type
kubectl get crd webservices.kro.run

# The RGD itself shows its status - kro validates CEL at install time
kubectl get resourcegraphdefinition webservice
# NAME         APIVERSION          KIND         AGE   TOPOLOGICALORDER
# webservice   kro.run/v1alpha1    WebService   10s   ["deployment","service"]
```

The `TOPOLOGICALORDER` column shows the dependency order kro inferred from your CEL expressions.

### Step 3 - Create an Instance

Create `practice/kro/ch01/my-webservice.yaml`:

```yaml
apiVersion: kro.run/v1alpha1
kind: WebService
metadata:
  name: my-app
  namespace: default
spec:
  image: nginx:alpine
  port: 80
  replicas: 2
```

```bash
kubectl apply -f practice/kro/ch01/my-webservice.yaml

# Watch kro reconcile the downstream resources
kubectl get webservice my-app
# NAME     READY   AGE
# my-app   True    15s

# Verify the downstream resources exist
kubectl get deployment,service my-app
# NAME                    READY   UP-TO-DATE   AVAILABLE
# deployment.apps/my-app  2/2     2            2
#
# NAME             TYPE        CLUSTER-IP     PORT(S)
# service/my-app   ClusterIP   10.96.x.x      80/TCP

# Read back the status kro wrote
kubectl get webservice my-app -o jsonpath='{.status}'
# {"clusterIP":"10.96.x.x","ready":true}
```

### Step 4 - Verify Reconciliation

Edit the WebService and watch the Deployment update:

```bash
kubectl patch webservice my-app --type=merge -p '{"spec":{"replicas":3}}'

kubectl get deployment my-app -w
# NAME     READY   UP-TO-DATE   AVAILABLE
# my-app   2/3     3            2
# my-app   3/3     3            3
```

### Step 5 - Clean Up

```bash
# Deleting the WebService CR deletes all downstream resources kro created
kubectl delete webservice my-app

# Verify downstream gone
kubectl get deployment,service my-app
# Error from server (NotFound)

# To uninstall the RGD entirely
kubectl delete resourcegraphdefinition webservice
```

---

## Conditional Resources

A resource can be conditionally included using a `readyOn` field (evaluated as a CEL boolean). If false, kro skips that resource - and any resource whose CEL expressions depend on it.

```yaml
resources:

- id: deployment
  template:
    # ...

# Only create an HPA if replicas > 1
- id: hpa
  readyOn: ${schema.spec.replicas > 1}
  template:
    apiVersion: autoscaling/v2
    kind: HorizontalPodAutoscaler
    metadata:
      name: ${schema.metadata.name}
    spec:
      scaleTargetRef:
        apiVersion: apps/v1
        kind: Deployment
        name: ${deployment.metadata.name}
      minReplicas: ${schema.spec.replicas}
      maxReplicas: 10
      metrics:
      - type: Resource
        resource:
          name: cpu
          target:
            type: Utilization
            averageUtilization: 70
```

This is equivalent to the conditional HPA you wrote in Crossplane Chapter 05 with Go template `{{- if gt .spec.replicas 1 }}` blocks - but expressed as pure CEL without template syntax.

---

## How kro Handles Dependencies

kro performs **static analysis** of every CEL expression before accepting an RGD. At install time it:

1. Parses all `${}` expressions in the `resources` and `schema.status` sections
2. Builds a directed acyclic graph of which resources reference which
3. Rejects the RGD if there are circular dependencies
4. Stores the resolved topological order (visible in `kubectl get rgd`)

At reconciliation time it creates resources in topological order, waiting for each to reach a ready state before moving to dependents. If an upstream resource's status field is not yet populated, kro re-queues and waits - no polling loops for you to write.

---

## Key Takeaways

- A **ResourceGraphDefinition** is kro's single object - schema + resources in one YAML file. Applied once by the platform team.
- **SimpleSchema** is a terse alternative to OpenAPI boilerplate: `string | default=nginx`.
- **CEL expressions** (`${}`) connect inputs to resource fields and infer dependency ordering automatically.
- kro is **non-Turing-complete by design** - CEL always terminates, has no side effects, and is validated at apply time.
- Deleting a CR deletes all kro-managed downstream resources (owned via Kubernetes `ownerReferences`).
- Use kro when CEL is expressive enough. Reach for Crossplane when you need Go functions, provider packages, or composition revisions.

[Back to README](../../README.md)
