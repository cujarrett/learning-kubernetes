# Chapter 04: Compositions - Structure, Functions & Go Templating

> **You will build:** A Composition that turns a `WebService` into a Deployment + Service + ConfigMap using Patch & Transform.

A **Composition** is the implementation behind an XRD. It tells Crossplane: "When someone creates a `WebService`, run this pipeline and produce these Kubernetes resources."

Compositions use `mode: Pipeline` - a sequence of Function steps where each step runs a plugin (a Function pod) that builds or transforms the desired resources. The primary tool you will use is **Go templating**. This chapter covers how Compositions work structurally, explains Patch & Transform briefly so you can read it in existing code, and then does the hands-on entirely in Go templates.

---

## Composition Architecture

```
XR Instance (WebService "my-api"):
  spec.image: nginx:alpine
  spec.replicas: 2
  spec.port: 80
        │
        ▼
  Composition
  ┌──────────────────────────────────────────────────────────┐
  │  spec.compositeTypeRef → WebService v1alpha1             │
  │  spec.mode: Pipeline                                     │
  │                                                          │
  │  Pipeline Step: "render-resources"                       │
  │  ┌────────────────────────────────────────────────────┐  │
  │  │  Function: function-go-templating                  │  │
  │  │  Input: Go template string (inline YAML)           │  │
  │  │  Output: rendered Kubernetes resource manifests    │  │
  │  └────────────────────────────────────────────────────┘  │
  └──────────────────────────────────────────────────────────┘
        │
        ▼
  Composed Resources (owned by the XR):
  - Deployment "my-api"
  - Service "my-api"
  - ConfigMap "my-api-config"   ← only if spec.config was provided
```

A Composition's `spec.compositeTypeRef` is how Crossplane matches the Composition to an XRD. Crossplane reads the XR's `apiVersion` and `kind`, finds the Composition that declares a matching `compositeTypeRef`, and runs its pipeline.

You can have multiple Compositions for the same XRD - selected by `spec.crossplane.compositionRef` or `spec.crossplane.compositionSelector` on the XR:

```yaml
# XR chooses a specific Composition by name
spec:
  crossplane:
    compositionRef:
      name: webservice-go-composition
```

```yaml
# XR selects a Composition by label (useful for channel-based rollout)
spec:
  crossplane:
    compositionSelector:
      matchLabels:
        channel: stable
```

---

## How Composition Functions Work

Every Function is a **gRPC server** running as a pod in the cluster. Crossplane calls each pipeline step's Function with a `RunFunctionRequest` and receives a `RunFunctionResponse`.

```
Crossplane Controller
        │
        │  gRPC: RunFunctionRequest
        │  ┌─────────────────────────────────┐
        │  │  observed.composite.resource    │  ← the live XR from the cluster
        │  │  observed.composed.resources    │  ← live state of child resources
        │  │  desired.composite.resource     │  ← accumulated from earlier steps
        │  │  desired.composed.resources     │  ← accumulated from earlier steps
        │  │  input                          │  ← the YAML under the step's input:
        │  └─────────────────────────────────┘
        ▼
  Function Pod (e.g. function-go-templating)
        │
        │  gRPC: RunFunctionResponse
        │  ┌─────────────────────────────────┐
        │  │  desired.composite.resource     │  ← updated status fields
        │  │  desired.composed.resources     │  ← resources to create/update
        │  │  results                        │  ← warning/error messages
        │  └─────────────────────────────────┘
        ▼
Crossplane applies the diff to the cluster
```

### Observed vs Desired

| | Observed | Desired |
|-|----------|---------|
| **Composite** | The XR as it exists right now in the cluster | Status fields the pipeline wants to write back |
| **Composed** | Current live state of every child resource | The resources that should exist after this reconcile |

The `desired.composed.resources` from the **last pipeline step** is what Crossplane applies. If a resource is in `observed` but absent from `desired` after all steps complete, Crossplane **deletes** it. This is cascade delete: remove a resource from your template and Crossplane removes it from the cluster on the next reconcile.

### Multi-Step Pipelines

Each step receives the accumulated `desired` state from all previous steps and can add to it. In practice you will usually have one step. Multi-step pipelines let you mix functions:

```yaml
pipeline:
- step: render-resources          # Step 1: Go templates produce Deployment + Service
  functionRef:
    name: function-go-templating
  input: ...

- step: auto-ready                # Step 2: mark XR Ready when all composed resources are Ready
  functionRef:
    name: function-auto-ready
```

---

## Patch & Transform - Read It Once

Platform teams often use Go templating. But you will encounter more basic `function-patch-and-transform` in existing Compositions, community examples, and Crossplane docs - so it is worth being able to read it.

**The concept:** Instead of a template, you write a list of `base` resource skeletons with `patches` that copy fields from the XR onto each resource.

```yaml
pipeline:
- step: create-resources
  functionRef:
    name: function-patch-and-transform
  input:
    apiVersion: pt.fn.crossplane.io/v1beta1
    kind: Resources
    resources:
    - name: deployment
      base:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 1                    # Static base - overridden by the patch below
          template:
            spec:
              containers:
              - name: app
                image: placeholder       # Placeholder - overridden by the patch below
      patches:
      # FromCompositeFieldPath: copy FROM the XR spec INTO the composed resource
      - type: FromCompositeFieldPath
        fromFieldPath: spec.image        # XR.spec.image   ──▶   Deployment container image
        toFieldPath: spec.template.spec.containers[0].image

      # ToCompositeFieldPath: copy FROM the composed resource BACK to XR status
      - type: ToCompositeFieldPath
        fromFieldPath: status.availableReplicas    # Deployment.status
        toFieldPath: status.replicas               # ──▶  XR.status.replicas

      readinessChecks:
      - type: MatchCondition
        matchCondition:
          type: Available
          status: "True"
```

**Patch & Transform vs. Go templates:**

| Need | Patch & Transform | Go Template |
|------|-------------------|-------------|
| Copy field A to field B | Native | Works but verbose |
| Create a resource only when a field is set | Not supported | `{{- if $spec.autoscaling.enabled }}` |
| Loop over `spec.ports` | Not supported | `{{- range $spec.ports }}` |
| Build a string from two fields | `CombineFromComposite` | `{{ printf "%s-%s" $ns $name }}` |

You now know enough to read any Patch & Transform Composition you encounter. The rest of this course uses Go templates.

---

## Go Templating: The Template Variables

`function-go-templating` runs your template with two top-level variables:

| Variable | What it contains |
|----------|----------------|
| `.observed` | A Go struct (fixed fields defined by the function library) - top-level entry point |
| `.observed.composite.resource` | A map (`map[string]interface{}`) - the XR deserialized from YAML, with keys `metadata`, `spec`, `status` |
| `.observed.composite.resource.metadata.name` | The XR's name |
| `.observed.composite.resource.spec.<field>` | Any field from the XR spec |
| `.observed.resources` | Observed composed resources (current cluster state of all child resources) |

The template is embedded in the Composition under `input.inline.template` using a YAML block scalar (`|`). Each `---` separator in the rendered output produces a separate Kubernetes resource:

```yaml
pipeline:
- step: render-resources
  functionRef:
    name: function-go-templating
  input:
    apiVersion: gotemplating.fn.crossplane.io/v1beta1
    kind: GoTemplate
    source: Inline
    inline:
      template: |
        {{- $name := .observed.composite.resource.metadata.name }}
        {{- $ns   := .observed.composite.resource.metadata.namespace }}
        {{- $spec := .observed.composite.resource.spec }}
        ---
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: {{ $name }}
          namespace: {{ $ns }}
        spec:
          replicas: {{ $spec.replicas | default 1 }}
          ...
        ---
        apiVersion: v1
        kind: Service
        ...
```

The `{{- $name := ... }}` variable assignments at the top avoid repeating `.observed.composite.resource.metadata.name` throughout the template.

---

## Hands-On: Build the WebService Composition With Go Templating

You will install `function-go-templating` and write a Composition that creates a Deployment, Service, and optional ConfigMap.

```bash
mkdir -p practice/ch04
```

### Step 1: Apply the Chapter 03 XRD

If you cleaned up after Chapter 03:

```bash
kubectl apply -f practice/ch03/webservice-xrd.yaml
kubectl get xrds --watch
# Wait for ESTABLISHED=True, then Ctrl+C
```

### Step 2: Install `function-go-templating`

Create `practice/ch04/function-go-templating.yaml`:

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Function
metadata:
  name: function-go-templating
spec:
  package: xpkg.crossplane.io/crossplane-contrib/function-go-templating:v0.12.0
```

Apply and wait for healthy:

```bash
kubectl apply -f practice/ch04/function-go-templating.yaml
kubectl get functions.pkg.crossplane.io --watch
# Wait for HEALTHY=True, then Ctrl+C
```

### Step 3: Write the Composition

Create `practice/ch04/webservice-composition.yaml`:

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: webservice-composition
  labels:
    channel: stable                           # Used by compositionSelector on the XR to pick this Composition
spec:
  compositeTypeRef:
    apiVersion: platform.example.io/v1alpha1  # Must match the XRD's group/version
    kind: WebService                          # Must match the XRD's names.kind - this is how Crossplane links them
  mode: Pipeline                              # Run a sequence of Function steps
  pipeline:
  - step: render-webservice                   # Arbitrary name - shows up in events/logs
    functionRef:
      name: function-go-templating            # Must match the metadata.name of the installed Function object
    input:
      apiVersion: gotemplating.fn.crossplane.io/v1beta1
      kind: GoTemplate
      source: Inline                          # Template is embedded here (alternative: ConfigMap)
      inline:
        template: |
          {{- $name := .observed.composite.resource.metadata.name }}       # Alias the XR name - avoids repeating the long path
          {{- $ns   := .observed.composite.resource.metadata.namespace }}  # Alias the XR namespace
          {{- $spec := .observed.composite.resource.spec }}                # Alias the XR spec map - all developer inputs live here
          ---                                               # "---" separator tells the function this is a new resource
          apiVersion: apps/v1
          kind: Deployment
          metadata:
            name: {{ $name }}
            namespace: {{ $ns }}
            annotations:
              gotemplating.fn.crossplane.io/composition-resource-name: deployment  # Required by function-go-templating v0.8+
            labels:
              app: {{ $name }}
              environment: {{ $spec.environment | default "production" }}  # | default - safe fallback if field omitted
          spec:
            replicas: {{ $spec.replicas | default 1 }}
            selector:
              matchLabels:
                app: {{ $name }}              # Selector must match the pod template labels below
            template:
              metadata:
                labels:
                  app: {{ $name }}            # These labels are what the Service selector targets
                  environment: {{ $spec.environment | default "production" }}
              spec:
                containers:
                - name: app
                  image: {{ $spec.image }}    # Required field - no default, XRD schema enforces it
                  ports:
                  - containerPort: {{ $spec.port | default 80 }}
          ---                                 # Second resource starts here
          apiVersion: v1
          kind: Service
          metadata:
            name: {{ $name }}
            namespace: {{ $ns }}
            annotations:
              gotemplating.fn.crossplane.io/composition-resource-name: service  # Required by function-go-templating v0.8+
            labels:
              app: {{ $name }}
          spec:
            selector:
              app: {{ $name }}                # Routes traffic to pods with label app={{ $name }}
            ports:
            - port: 8080                      # Port the Service listens on inside the cluster
              targetPort: {{ $spec.port | default 80 }}  # Port on the pod to forward to
              protocol: TCP
          {{- if $spec.config }}              # Only render the ConfigMap if spec.config was provided
          ---
          apiVersion: v1
          kind: ConfigMap
          metadata:
            name: {{ $name }}-config
            namespace: {{ $ns }}
            annotations:
              gotemplating.fn.crossplane.io/composition-resource-name: configmap  # Required by function-go-templating v0.8+
            labels:
              app: {{ $name }}
          data:
          {{- range $key, $val := $spec.config }}   # Iterate over the free-form map[string]string
            {{ $key }}: {{ $val | quote }}           # | quote wraps the value in quotes - safe for strings with spaces
          {{- end }}
          {{- end }}                          # End of the if block - no ConfigMap rendered if config was absent
```

Apply it:

```bash
kubectl apply -f practice/ch04/webservice-composition.yaml
```

### Step 4: Create a WebService Instance

Create `practice/ch04/my-webservice.yaml`:

```yaml
apiVersion: platform.example.io/v1alpha1
kind: WebService
metadata:
  name: my-webservice
  namespace: default
spec:
  image: nginx:alpine
  replicas: 2
  port: 80
  environment: development
  config:
    LOG_LEVEL: debug
    APP_NAME: my-webservice
```

Apply and watch resources appear:

```bash
kubectl apply -f practice/ch04/my-webservice.yaml
kubectl get deployments --watch
# Wait for 2/2 READY, then Ctrl+C
kubectl get deployments,services,configmaps
# All three should now be present
```

### Step 5: Verify the Template Rendered Correctly

```bash
# Check Deployment labels and image
kubectl get deployment my-webservice -o yaml | grep -E "image:|environment:|replicas:"

# Inspect the ConfigMap - values from spec.config
kubectl get configmap my-webservice-config -n default -o yaml
```

Expected ConfigMap `data`:

```yaml
data:
  APP_NAME: "my-webservice"
  LOG_LEVEL: "debug"
```

### Step 6: Test the Conditional - No Config

```bash
kubectl apply -f - <<'EOF'
apiVersion: platform.example.io/v1alpha1
kind: WebService
metadata:
  name: bare-webservice
  namespace: default
spec:
  image: nginx:alpine
  replicas: 1
  port: 80
EOF
```

```bash
kubectl get deployments,services,configmaps
```

`bare-webservice` gets a Deployment and Service but **no ConfigMap** - the `{{- if $spec.config }}` block was falsy with no `config` provided.

### Step 7: Test Cascade Delete

```bash
kubectl delete webservice my-webservice -n default
kubectl get deployments --watch
# Watch the Deployment disappear, then Ctrl+C
kubectl get deployments,services,configmaps
# All three should be gone

kubectl delete webservice bare-webservice -n default
```

### Step 8: Debug Tips

When a Composition fails to render, check the XR events:

```bash
kubectl describe webservice my-webservice -n default
# Scroll to Events: - template errors appear here
```

Common errors:
- `nil pointer evaluating` - a field was not provided; fix with `| default "value"`
- `template: ...: unexpected "}"` - YAML indentation broke the template string

Function pod logs:

```bash
kubectl get pods -n crossplane-system | grep go-templating
kubectl logs <pod-name> -n crossplane-system
```

---

## What You Built

- `function-go-templating` installed as a Crossplane Function
- A Composition rendering Deployment + Service + conditional ConfigMap via Go templates
- Confirmed cascade delete: removing the XR removes all composed resources
- Read and understood P&T syntax - you can now read any Composition that uses it

### Clean Up

Remove everything applied in this chapter:

```bash
kubectl delete webservice my-webservice bare-webservice -n default --ignore-not-found
kubectl delete -f practice/ch04/webservice-composition.yaml --ignore-not-found
```

Leave the XRD and function installed - Chapter 05 builds on them.

Chapter 05 goes deeper: Sprig helpers, nil-safe map patterns, writing status back to the XR, and template `define`/`include` blocks for reuse.

---

➡️ [Crossplane 05: Go Templating Deep Dive](05-go-templating.md)
