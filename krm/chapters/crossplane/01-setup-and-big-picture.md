# Chapter 01: Setup & The Big Picture

> **You will build:** Crossplane running on minikube; starter `App` XRD deployed and tested.

## What Is Crossplane?

Crossplane is a Kubernetes extension - it runs as pods inside your cluster and lets you define your own **custom APIs** for anything you want to provision: Kubernetes resources, cloud resources, internal tooling. Those APIs look like ordinary Kubernetes objects. You write YAML, `kubectl apply` it, and a controller reconciles the desired state into reality automatically.

Think of it as: **your own Kubernetes API server extension, without writing Go controller code.**

---

## The Mental Model: XRD → Composition → Resources

Crossplane has three core building blocks:

```
                             ┌──────────────────────────────────────────────────────────────┐
                             │  PLATFORM TEAM (deployed once, sits in the cluster)          │
                             │                                                              │
                             │  ┌──────────────────────────┐  ┌──────────────────────────┐  │
                             │  │  XRD                     │  │  Composition             │  │
                             │  │  Defines the schema      │  │  Defines what to CREATE  │  │
                             │  │  Registers type as CRD   │  │  Runs Function pipeline  │  │
                             │  └──────────────────────────┘  └──────────────────────────┘  │
                             └──────────────┬──────────────────────────────┬────────────────┘
                                            │ validates                    │ drives
                                            │                              │
  DEVELOPER                                 ▼                              ▼
                                     ┌──────┴──────────────────────────────┴──────┐
  # XR - namespaced YAML             │  XR (Composite Resource)                   │
  # that the developer writes        │  Namespaced live object                    │
  # and commits to Git.    ────────► │  Holds inputs & outputs                    │
  apiVersion: example.io/v1alpha1    └──────────────────────┬─────────────────────┘
  kind: WebService                                          │
  metadata:                                                 │ Crossplane reconciles
    name: my-api                                            ▼
    namespace: team-a                ┌────────────────────────────────────────┐
  spec:                              │  Kubernetes Resources                  │
    image: ...                       │  Deployment, Service, ConfigMap, etc.  │
    replicas: 2                      └────────────────────────────────────────┘
```

**Platform team** owns the XRD and Composition - they define what `WebService` means.
**Developer** just writes the `WebService` YAML with the fields they care about.

---

## The Local Development Loop

Every chapter in this guide follows the same tight loop:

```
1. Write or edit a YAML file (XRD, Composition, or XR instance)
   └─ Your editor

2. Apply it to minikube
   └─ kubectl apply -f <file.yaml>

3. Crossplane reconciles
   └─ Reads the desired state, runs the Composition pipeline,
      creates/updates Kubernetes resources

4. Inspect the result
   └─ kubectl describe <xr-name>
      kubectl get deployments,services
      kubectl get events --sort-by=.metadata.creationTimestamp

5. Tweak and repeat
```

That is it. No CI/CD, no cloud, no GitOps tooling. When you understand Crossplane deeply through this local loop, hooking it into a GitOps pipeline later is straightforward - you are just automating step 2 (the `kubectl apply`).

---

## Crossplane's Key Terms

| Term | What It Is | Analogy |
|------|-----------|---------|
| **XRD** | Defines a custom resource type and its schema | TypeScript type / Go struct definition |
| **Composition** | Defines what resources to create for a given XRD | The function body that implements the interface |
| **XR** (Composite Resource) | An instance - what the developer applies | An instantiated object |
| **Function** | A plugin that runs inside the Composition pipeline | Middleware / transformer |
| **Provider** | Installs managed resource types for AWS/GCP/K8s/etc. | An SDK for a cloud API |

In this guide: no Provider, no cloud. All Compositions produce Kubernetes-native resources (Deployments, Services, ConfigMaps).

---

## The Starter Project Files

Look at the four files in the root of this repo:

**`xrd.yaml`** - defines an `App` custom resource with one input field: `spec.image`

**`composition.yaml`** - when an `App` is created, run the `function-patch-and-transform` function to create a Deployment and a Service. Uses `FromCompositeFieldPath` patches to copy `spec.image` from the XR into the Deployment's container image.

**`function.yaml`** - installs `function-patch-and-transform` from the Crossplane contrib registry. Functions are downloaded as OCI images and run as pods inside the cluster.

**`app.yaml`** - an `App` instance in the `default` namespace with `spec.image: nginx`. Applying this triggers the Composition pipeline.

---

## Hands-On: Install Crossplane on Minikube

### Step 1: Install the Tools

```bash
# minikube - local Kubernetes cluster
brew install minikube

# kubectl - the Kubernetes CLI (you may already have this)
brew install kubectl

# helm - package manager for Kubernetes (we use it to install Crossplane)
brew install helm

# Optional: the Crossplane CLI (adds `crossplane` subcommands)
brew install crossplane
```

Verify Docker Desktop is running. Minikube uses Docker as the VM driver on macOS by default.

### Step 2: Create a Dedicated Minikube Cluster

```bash
minikube start \
  --profile crossplane \
  --memory=4096 \
  --cpus=2 \
  --driver=docker
```

This creates a cluster named `crossplane` (separate from any other minikube clusters you have). When it is ready:

```bash
kubectl cluster-info --context crossplane
```

You should see the control plane URL printed. Minikube automatically sets your `kubectl` context to the new cluster.

Check the context is active:

```bash
kubectl config current-context
# Should print: crossplane
```

### Step 3: Install Crossplane via Helm

```bash
# Add the Crossplane Helm chart repository
helm repo add crossplane-stable https://charts.crossplane.io/stable
helm repo update

# Install Crossplane into its own namespace
helm install crossplane crossplane-stable/crossplane \
  --namespace crossplane-system \
  --create-namespace \
  --wait
```

The `--wait` flag blocks until all Crossplane pods are running. This takes about 60 seconds.

Verify Crossplane pods are healthy:

```bash
kubectl get pods -n crossplane-system
```

Expected output (pod name suffixes will differ):

```
NAME                                       READY   STATUS    RESTARTS   AGE
crossplane-7d8b9f4d6c-xk2zp                1/1     Running   0          60s
crossplane-rbac-manager-5b8f9c6d7-r9abc    1/1     Running   0          60s
```

### Step 4: Apply the Starter Project Files

Apply the files from `practice/ch01/` **in order**. Order matters because the Composition references the Function by name (the Function must exist first), and the XRD must exist before Crossplane can match an XR to a Composition.

```bash
# 1. Install the Function plugin (Crossplane will download the OCI image)
kubectl apply -f practice/ch01/function.yaml

# 2. Register the App custom resource type
kubectl apply -f practice/ch01/xrd.yaml

# 3. Define what an App creates
kubectl apply -f practice/ch01/composition.yaml
```

### Step 5: Wait for the Function to Be Healthy

Functions are OCI images pulled from the registry. This takes a moment:

```bash
kubectl get functions.pkg.crossplane.io --watch
```

Watch until `HEALTHY` shows `True`:

```
NAME                                              INSTALLED   HEALTHY   PACKAGE
crossplane-contrib-function-patch-and-transform   True        True      xpkg.crossplane.io/...
```

Press `Ctrl+C` to stop watching.

### Step 6: Create Your First App

```bash
kubectl apply -f practice/ch01/app.yaml
```

Watch Crossplane provision the resources:

```bash
kubectl get deployments --watch
```

Within 10–20 seconds you should see a Deployment appear and reach `2/2 READY`. Press `Ctrl+C`.

Also check the Service:

```bash
kubectl get services
```

You should see `my-app` (or similar) in the list with a ClusterIP.

### Step 7: Inspect What Crossplane Built

```bash
# See the App XR object
kubectl get apps -n default

# Full details including status
kubectl describe app my-app -n default
```

In the `Status` section look for:
- `replicas` - populated from the Deployment's `status.availableReplicas`
- `address` - populated from the Service's `spec.clusterIP`

These are **outputs written back by the Composition** using `ToCompositeFieldPath` patches. The XR becomes the single source of truth for both inputs and outputs.

```bash
# See what was actually created
kubectl get deployments,services
```

### Step 8: Verify Cascade Delete

One of Crossplane's most powerful behaviors: deleting the XR deletes everything it created.

```bash
kubectl delete -f practice/ch01/app.yaml

# Watch the Deployment disappear
kubectl get deployments --watch
# Ctrl+C after a few seconds
```

Crossplane owns the composed resources. When the owner (the XR) is deleted, everything it composed is cleaned up automatically.

---

## Troubleshooting

**Function stays `HEALTHY: False`**
```bash
kubectl describe function crossplane-contrib-function-patch-and-transform
# Look at Events and Conditions - often a pull rate limit or network issue
```

**XR stays in `Synced: False`**
```bash
kubectl describe app my-app -n default
# Check the Events section at the bottom
```

**Minikube context is wrong**
```bash
kubectl config use-context crossplane
```

---

## What You Built

- A dedicated minikube cluster named `crossplane` with 4GB RAM and 2 CPUs
- Crossplane installed in `crossplane-system` via Helm
- The `App` XRD - your first custom Kubernetes API
- A Composition that creates a Deployment and Service from an `App`
- An `App` instance running nginx with two replicas
- An understanding of what Crossplane does

---

➡️ [Crossplane 02: Kubernetes Resources Refresher](02-kubernetes-refresher.md)