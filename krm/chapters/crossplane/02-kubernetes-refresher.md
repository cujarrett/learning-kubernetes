# Chapter 02: Kubernetes Resources Refresher

> **You will build:** A Deployment + Service applied manually so the automation in later chapters feels concrete.

Crossplane creates and manages Kubernetes resources on your behalf. Before building custom resource APIs, you need a solid mental model of *what* Crossplane is creating under the hood and *how* those resources work together.

This chapter covers the Kubernetes building blocks you will see throughout this guide, then lets you deploy them manually so the automation in later chapters feels concrete, not magic.

---

## Group / Version / Kind (GVK)

Every resource in Kubernetes is uniquely identified by three coordinates:

| Concept | YAML key | Example | Value casing |
|---------|----------|---------|--------------|
| Group | part of `apiVersion` | `apps` | lowercase |
| Version | part of `apiVersion` | `v1` | lowercase |
| Kind | `kind` | `Deployment` | PascalCase |

In YAML this appears at the top of every manifest:

```yaml
apiVersion: apps/v1    # group/version
kind: Deployment       # kind
```

For core Kubernetes resources (`Pod`, `Service`, `ConfigMap`, `Namespace`) the group is empty, so `apiVersion` is just the version:

```yaml
apiVersion: v1     # no group prefix = the "core" API group
kind: Service
```

### Why GVK Matters for Crossplane

When you create an XRD in Chapter 03, you are defining a new GVK:
- Group: `platform.example.io`
- Version: `v1alpha1`
- Kind: `WebService`

So `apiVersion: platform.example.io/v1alpha1, kind: WebService` is just Kubernetes receiving a resource object - Crossplane's controller reacts to it.

---

## CustomResourceDefinitions (CRDs)

A **CRD** (CustomResourceDefinition) extends the Kubernetes API server so it understands a new Kind. Without a CRD, the API server rejects any resource with an unknown `apiVersion/kind` combination.

When you applied `xrd.yaml` in Chapter 01, Crossplane generated a CRD behind the scenes:

```bash
kubectl get crds | grep example.crossplane.io
# apps.example.crossplane.io   2025-01-01T00:00:00Z
```

That CRD is why `kubectl apply -f app.yaml` works - the API server now knows what `kind: App` is. Without the XRD (and therefore without the CRD), you would get:

```
error: no kind "App" is registered for version "example.crossplane.io/v1"
```

**Key insight: XRDs are a Crossplane abstraction that creates and manages CRDs automatically.** You never write CRDs by hand in Crossplane - XRDs do it for you.

---

## The Resources Crossplane Will Manage

In this guide, all Compositions create standard Kubernetes resources. Here is a reference for each one.

### Deployment

Manages a set of identical Pod replicas. The Deployment controller ensures the desired number of replicas are always running, restarting Pods if they crash.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: default
spec:
  replicas: 2                   # Number of pods to run
  selector:
    matchLabels:
      app: my-app               # Must match the template's labels
  template:
    metadata:
      labels:
        app: my-app             # Pods get this label
    spec:
      containers:
      - name: app
        image: nginx:alpine     # Container image
        ports:
        - containerPort: 80     # Port the container listens on
        resources:
          requests:
            memory: "64Mi"
            cpu: "50m"
          limits:
            memory: "128Mi"
            cpu: "100m"
```

### Service

Creates a stable DNS name and ClusterIP that routes traffic to a set of Pods selected by labels. The IP of a Pod changes when the Pod restarts; the Service IP is stable.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-app
  namespace: default
spec:
  selector:
    app: my-app        # Routes traffic to pods with this label
  ports:
  - port: 8080         # Port clients connect to
    targetPort: 80     # Port on the pod container
    protocol: TCP
  type: ClusterIP      # Internal-only (default). Not exposed outside cluster.
```

### ConfigMap

Stores key-value configuration data that Pods can read as environment variables or as mounted files.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-app-config
  namespace: default
data:
  LOG_LEVEL: info
  MAX_CONNECTIONS: "100"
  DATABASE_URL: postgres://db:5432/myapp
```

A Pod consumes it as environment variables:

```yaml
spec:
  containers:
  - name: app
    envFrom:
    - configMapRef:
        name: my-app-config     # All keys become env vars
```

### HorizontalPodAutoscaler (HPA)

Automatically scales a Deployment's `replicas` up and down based on CPU or memory utilization. You will use this in Chapter 07.

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: my-app
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: my-app
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70     # Scale up when avg CPU > 70%
```

---

## The Label / Selector Pattern

This is the most important pattern to understand before automating with Crossplane.

**How a Service finds its Pods:**

```
Pod:
  labels:
    app: my-app     ← Pod has this label

Service:
  selector:
    app: my-app     ← Service selects pods where app=my-app
```

Traffic sent to the Service's ClusterIP is forwarded to any Pod that matches the selector labels.

**How a Deployment manages its Pods:**

```
Deployment.spec.selector.matchLabels:
  app: my-app     ← Deployment claims pods with this label

Deployment.spec.template.metadata.labels:
  app: my-app     ← Pods it creates will have this label
```

The selector in `spec.selector.matchLabels` must match the labels in `spec.template.metadata.labels`. If they don't match, the Deployment controller rejects it.

In your Compositions, you will use the XR's `metadata.name` as the label value so that all resources created from one XR link together consistently.

---

## Hands-On: Deploy Resources Manually

Before Crossplane automates this, deploy a Deployment + Service manually. This makes the automation in later chapters feel concrete rather than abstract.

```bash
mkdir -p practice/ch02
```

### Step 1: Write the Deployment

Create `practice/ch02/nginx-deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: learn-nginx
  namespace: default
  labels:
    app: learn-nginx
spec:
  replicas: 2
  selector:
    matchLabels:
      app: learn-nginx
  template:
    metadata:
      labels:
        app: learn-nginx
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        ports:
        - containerPort: 80
        resources:
          requests:
            memory: "64Mi"
            cpu: "50m"
          limits:
            memory: "128Mi"
            cpu: "100m"
```

### Step 2: Write the Service

Create `practice/ch02/nginx-service.yaml`:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: learn-nginx
  namespace: default
spec:
  selector:
    app: learn-nginx
  ports:
  - port: 8080
    targetPort: 80
    protocol: TCP
  type: ClusterIP
```

### Step 3: Apply Both Resources

```bash
kubectl apply -f practice/ch02/nginx-deployment.yaml
kubectl apply -f practice/ch02/nginx-service.yaml
```

Watch the pods come up:

```bash
kubectl get pods --watch
# Wait until both pods show STATUS: Running, then Ctrl+C
```

### Step 4: Verify the Service Got a ClusterIP

```bash
kubectl get service learn-nginx
```

Expected output:

```
NAME          TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)    AGE
learn-nginx   ClusterIP   10.96.45.123   <none>        8080/TCP   10s
```

That `CLUSTER-IP` is a stable internal address. Crossplane will write this IP back to the XR's status field in later chapters.

### Step 5: Test With Port-Forward

```bash
kubectl port-forward service/learn-nginx 8080:8080
```

Open `http://localhost:8080` in your browser. You should see the nginx welcome page.

Press `Ctrl+C` to stop the port-forward.

### Step 6: Examine the Label Connection

```bash
# Show pods with their labels
kubectl get pods --show-labels

# Describe the service - look at "Endpoints" and "Selector"
kubectl describe service learn-nginx
```

In the `describe` output you will see:
- `Selector: app=learn-nginx` - the service's selector
- `Endpoints: 10.x.x.x:80, 10.x.x.x:80` - the actual pod IPs that matched

This endpoint list is populated automatically by Kubernetes whenever a pod with `app=learn-nginx` is running and ready.

### Step 7: Test Cascade Delete

```bash
kubectl delete -f practice/ch02/nginx-service.yaml
kubectl delete -f practice/ch02/nginx-deployment.yaml

kubectl get pods
# Pods are terminating
```

When Crossplane creates these resources inside a Composition, it sets ownership references on them - so deleting the XR (the parent) cascades down and deletes the Deployment and Service (the children) automatically. That is the same ownership model as Kubernetes' built-in owner references.

---

## What You Built

- A Deployment running two nginx pods
- A Service routing traffic to those pods via label selection
- Verified the label/selector linking pattern that all future Compositions will use
- Tested connectivity via port-forward
- Understood how CRDs extend the Kubernetes API for custom resource types

In the next chapter you will define your own `WebService` custom resource type with a rich schema that validates developer inputs before anything gets created.

---

➡️ [Crossplane 03: XRDs - Your Platform API](03-xrds.md)
