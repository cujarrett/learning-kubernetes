# Foundations 01: CRDs, CRs, and Controllers - The Full Picture

> **You will understand:** The three-layer model (CRD → CR → Controller), how tools like Crossplane and kro fit into it, and when to write your own controller vs reach for a higher-level tool.

This chapter steps back from any specific tool and looks at the **Kubernetes Resource Model (KRM)** - the broader pattern that Crossplane, kro, Argo CD, Flux, cert-manager, and every Kubernetes operator are all built on. Once you see the pattern clearly, you will know exactly which tool to reach for in a given situation.

---

## The Three Layers

Everything in Kubernetes - whether built-in or custom - works through the same three-layer model:

```
┌──────────────────────────────────────────────────────────────┐
│  Layer 1: CRD (CustomResourceDefinition)                     │
│                                                              │
│  Registers a new *type* with the API server.                 │
│  "I want the cluster to understand kind: Database"           │
│                                                              │
│  Lives at: apiextensions.k8s.io/v1                           │
│  Scope: cluster-wide (always)                                │
└──────────────────────────────┬───────────────────────────────┘
                               │ defines schema for
                               ▼
┌──────────────────────────────────────────────────────────────┐
│  Layer 2: CR (Custom Resource)                               │
│                                                              │
│  An *instance* of the type the CRD defined.                  │
│  "I want one Database named my-postgres"                     │
│                                                              │
│  Stored in etcd just like a Deployment or Service.           │
│  Has spec (desired state) + status (observed state).         │
└──────────────────────────────┬───────────────────────────────┘
                               │ watched by
                               ▼
┌──────────────────────────────────────────────────────────────┐
│  Layer 3: Controller                                         │
│                                                              │
│  A reconciliation loop that watches for CRs and makes        │
│  the world match what the CR says it wants.                  │
│                                                              │
│  Runs as a Pod in the cluster. Calls the API server and      │
│  downstream systems (cloud APIs, other k8s objects, etc).    │
└──────────────────────────────────────────────────────────────┘
```

No layer is optional. A CRD with no controller means resources get stored but nothing acts on them. A controller with no CRD means there is no type to watch. CRs are the user-facing surface - the YAML you or your team commits to Git.

---

## Definitions Side by Side

| | CRD | CR | Controller |
|---|---|---|---|
| **What it is** | A type registration | An instance of a type | A reconciliation process |
| **Who creates it** | Platform/operator author | End user or automation | Platform/operator author |
| **Where it lives** | Cluster-wide (not namespaced) | Namespaced or cluster-wide | Runs as a Pod |
| **Persisted in etcd?** | Yes | Yes | No - it's running code |
| **Analogous to** | A Go struct definition | A Go struct value | The function acting on that value |
| **YAML key** | `kind: CustomResourceDefinition` | `kind: Database` (your type) | Deployment manifest for the operator |

---

## The Reconciliation Loop

Every controller - whether you write it yourself or use Crossplane - implements the same pattern:

```
                  ┌─────────────────────────────────────┐
                  │         etcd (API server)           │
                  │                                     │
                  │  CR: Database/my-postgres           │
                  │    spec.engine: postgres            │
                  │    spec.version: "16"               │
                  │    status.ready: false              │
                  └───────────────┬─────────────────────┘
                                  │  Watch / List
                                  ▼
                  ┌─────────────────────────────────────┐
                  │         Controller (your Pod)       │
                  │                                     │
                  │  1. GET current CR from API server  │
                  │  2. Observe actual world state      │
                  │  3. Compute diff                    │
                  │  4. Act: create / update / delete   │
                  │  5. Patch status with result        │
                  │  6. Re-queue (loop forever)         │
                  └───────────────┬─────────────────────┘
                                  │  Creates / updates
                                  ▼
                  ┌─────────────────────────────────────┐
                  │  Actual resources                   │
                  │  (other k8s objects, cloud APIs,    │
                  │   external services, databases …)   │
                  └─────────────────────────────────────┘
```

Step 3 is the crux: the controller is always computing **desired state minus current state** and only acting on the difference. This is why Kubernetes-style systems are *eventually consistent* - the loop keeps running until the diff is zero.

---

## What Crossplane Actually Is

Crossplane is a **controller framework**. It ships a set of controllers that watch its own CRDs (XRDs, Compositions, XRs) and reconcile them. When you write an XRD, Crossplane's controller creates a new CRD for you. When someone submits a CR of that type, Crossplane's composition engine runs the Function pipeline and reconciles the downstream resources.

```
┌──────────────────────────────────────────────────────────────┐
│  Standard Kubernetes approach                                │
│                                                              │
│  You write: CRD + Controller code (Go, operator-sdk, etc.)   │
│  You deploy: CRD manifest + controller Pod                   │
└──────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────┐
│  Crossplane approach                                         │
│                                                              │
│  You write: XRD + Composition (+ optional Go Function)       │
│  You deploy: Just YAML - Crossplane's controller does rest   │
│                                                              │
│  Crossplane handles: CRD creation, watching, reconciliation, │
│  status rollup, composition revision tracking, RBAC claims   │
└──────────────────────────────────────────────────────────────┘
```

Crossplane eliminates the need to write a Go controller for the common case of "watch this CR and produce these downstream resources." You only write Go (see Crossplane Chapter 10) when you need logic that YAML cannot express.

---

## When Something "Uses Controllers"

When a system says it "uses controllers," it means it ships its own CRDs and controller Pods that implement the reconciliation loop for its domain. Examples:

| System | Its CRDs (examples) | What the controller does |
|--------|---------------------|--------------------------|
| cert-manager | `Certificate`, `Issuer`, `ClusterIssuer` | Requests TLS certs from ACME/Vault, writes them as Secrets |
| Argo CD | `Application`, `AppProject` | Syncs Git repo contents to the cluster |
| Flux | `GitRepository`, `Kustomization`, `HelmRelease` | Pulls from Git/Helm and applies to cluster |
| Crossplane | `XRD`, `Composition`, `XR` | Creates CRDs, runs Function pipelines, reconciles managed resources |
| kro | `ResourceGraphDefinition` | Parses CEL expressions, infers dependency graph, generates CRD, reconciles instances |
| AWS Controllers for K8s (ACK) | `Bucket`, `Queue`, `RDSInstance` | Calls AWS APIs to provision cloud resources |
| Your custom operator | Whatever you define | Whatever you implement |

When you need to *provision* something that is managed by one of these systems, you interact via its CRs - not by calling the controller directly.

---

## Decision Framework: Controller vs Crossplane vs Nothing

Use this when deciding how to expose a new internal capability:

```
Does the capability need to be expressed as a declarative Kubernetes API?
│
├── No → Use a script, Terraform, or another tool. Don't force KRM on it.
│
└── Yes → Does the logic fit a pattern of "given these inputs, create these resources"?
           │
           ├── Yes, and resources are Kubernetes or provider-managed objects
           │    └── Use Crossplane XRD + Composition (± Function in Go if complex)
           │
           ├── Yes, but requires deep Kubernetes API interaction, admission webhooks,
           │   custom status tracking, or real-time event handling
           │    └── Write a controller (kubebuilder or operator-sdk)
           │
           └── Unsure - start with Crossplane, migrate to a controller if you hit limits
```

### Crossplane is the right fit when:

- Output is a set of Kubernetes resources (Deployments, ConfigMaps, PVCs, etc.)
- Output is cloud or provider resources (GitHub repos, S3 buckets, DNS records)
- You want platform teams to offer self-service APIs without writing Go
- You want composition, revision tracking, and RBAC out of the box

### Write your own controller when:

- You need to react to external events in real time (webhooks, queues)
- You need to implement admission/validation logic (use admission webhooks instead of a reconciler)
- Your reconciliation requires complex branching logic that Go Templating or CEL cannot handle
- You need tight control over requeueing behavior, caching, or leader election
- The "resource" lifecycle does not map cleanly to create/update/delete (e.g., long-running jobs)

---

## A Concrete Example: Provisioning a Kafka Topic

Suppose your platform team wants developers to provision Kafka topics. Here are three ways to model it:

### Option A - Custom Controller (full control)

```yaml
# CRD you define
apiVersion: kafka.platform.example.io/v1alpha1
kind: Topic
metadata:
  name: order-events
spec:
  partitions: 12
  replicationFactor: 3
  retentionMs: 604800000
```

You write a Go controller that:
1. Watches for `Topic` CRs
2. Calls the Kafka Admin API to create/update the topic
3. Writes status with broker assignments
4. Handles deletion with a finalizer

Use this if Kafka's Admin API behavior is complex enough that a Function pipeline would be awkward.

### Option B - Crossplane Provider (Crossplane handles the loop)

Upbound or the community may already publish a `provider-kafka`. Then:

```yaml
# Managed Resource from the provider's CRD
apiVersion: kafka.crossplane.io/v1alpha1
kind: Topic
metadata:
  name: order-events
spec:
  forProvider:
    partitions: 12
    replicationFactor: 3
```

The provider's controller (packaged as a Crossplane Provider) handles the Kafka API calls. You get RBAC, status, and composition support for free.

### Option C - Crossplane XRD wrapping Option B

```yaml
# Your platform's opinionated API, composed on top of the provider MR
apiVersion: platform.example.io/v1
kind: KafkaTopic
metadata:
  name: order-events
spec:
  retentionDays: 7     # human-friendly field, not raw milliseconds
  env: production      # picks partition count from a Composition policy
```

Developers see a clean API. The Composition translates `retentionDays` → `retentionMs` and sets partition count based on `env`. The underlying provider MR never surfaces to developers.

---

## The KRM Contract: spec vs status

Whether you use a CRD + controller, a Crossplane XR, or a built-in Kubernetes resource, every object follows the same contract:

```
spec    → What you want (desired state)     - written by you, committed to Git
status  → What exists (observed state)      - written by the controller, never committed to Git
```

This is why you should **never commit status fields to Git**. Controllers own status. If you commit status fields, a `kubectl apply` may try to set fields the controller considers authoritative, causing conflicts.

```yaml
# ✅ What you commit to Git
spec:
  replicas: 3
  image: nginx:alpine

# ❌ Never commit this - the controller writes it
status:
  readyReplicas: 3
  conditions:
  - type: Ready
    status: "True"
```

---

## Hands-On: Inspect the Real CRDs Crossplane Created

Apply the XRD from Crossplane Chapter 03 if it is not already in your cluster, then inspect what Crossplane generated:

```bash
# See all CRDs - Crossplane creates one CRD per XRD
kubectl get crds | grep example.crossplane.io

# Describe the generated CRD to see the schema Crossplane derived from your XRD
kubectl describe crd apps.example.crossplane.io

# See the controller Pods that implement the reconciliation loop
kubectl get pods -n crossplane-system

# Watch the reconciliation events for a specific XR instance
kubectl describe app my-app -n default | grep -A 20 Events
```

Now look at a built-in CRD from cert-manager (if installed) or the Crossplane provider:

```bash
# Compare schema structure - all CRDs follow the same OpenAPI v3 schema pattern
kubectl get crd apps.example.crossplane.io -o yaml | head -80
```

Notice: the structure is identical to what Crossplane generates from your XRD. That is the point - **Crossplane XRDs are just a higher-level API for authoring CRDs without writing the CRD YAML yourself.**

---

## Key Takeaways

- **CRD** = type registration. The API server schema. Written once by the platform author.
- **CR** = an instance. The user-facing YAML. Stored in etcd. Has `spec` (desired) and `status` (observed).
- **Controller** = running process. Watches CRs, reconciles the world, writes status.
- **Crossplane** is a controller framework: it ships controllers that turn XRDs into CRDs, and Compositions + Functions into reconciliation logic - without you writing Go for the common case.
- When something "uses controllers," it means it ships CRDs + controller Pods. You interact with it by submitting CRs.
- The KRM contract (`spec` = desired, `status` = observed, never commit status) applies universally across all three layers.

➡️ [Crossplane 01: Setup and Big Picture](../crossplane/01-setup-and-big-picture.md)
