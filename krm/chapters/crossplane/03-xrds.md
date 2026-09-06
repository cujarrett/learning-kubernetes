# Chapter 03: XRDs - Your Platform API

> **You will build:** A `WebService` XRD with schema validation, defaults, enum constraints, and status fields.

A **CompositeResourceDefinition (XRD)** is how you create your own Kubernetes custom resource type. When platform teams say "we have a `MicroService` API" or "developers create a `WebService` XR in Backstage", they are describing a type that was defined in an XRD.

When Crossplane processes your XRD it does two things:
1. Creates a **CRD** in the cluster so `kubectl` accepts your new Kind
2. Starts watching for instances of that Kind to reconcile against a matching Composition

---

## Full Anatomy of an XRD

Let us dissect the `xrd.yaml` from this repo line by line:

```yaml
apiVersion: apiextensions.crossplane.io/v2         # Crossplane v2 XRD API
kind: CompositeResourceDefinition
metadata:
  name: apps.example.crossplane.io                 # MUST be <plural>.<group> - Kubernetes enforces this
spec:
  scope: Namespaced                                 # The XR lives in a namespace (Crossplane v2 feature)
  group: example.crossplane.io                      # API group - use your org's domain e.g. platform.company.io
  names:
    kind: App                                       # PascalCase Kind developers write in their YAML
    plural: apps                                    # Lowercase plural used in API URL paths
  versions:
  - name: v1                                        # The version string - stability-stage versioning rather than semver
    served: true                                    # The API server accepts this version
    referenceable: true                             # Compositions can reference this version
    schema:
      openAPIV3Schema:                              # JSON Schema - Kubernetes validates submitted resources against this
        type: object
        properties:
          spec:
            type: object
            properties:
              image:
                description: The app's OCI container image.
                type: string
            required:
            - image                                 # Kubernetes rejects the resource if image is missing
          status:
            type: object
            properties:
              replicas:
                description: The number of available app replicas.
                type: integer
              address:
                description: The app's IP address.
                type: string
```

---

## Key Concepts Explained

### `metadata.name` Rule

The name **must always** be `<plural>.<group>`. Kubernetes enforces this:

```yaml
# group: platform.example.io, names.plural: webservices
metadata:
  name: webservices.platform.example.io   # ✅ correct

metadata:
  name: webservice-api                    # ❌ will fail
```

### `spec.scope`

Controls whether the composite resource lives in a namespace or is cluster-wide:

| Value | Meaning | Use case |
|-------|---------|----------|
| `Namespaced` | The XR lives in a namespace, like a Deployment | Multi-tenant platforms where teams own namespaces |
| `Cluster` | The XR is cluster-wide | Shared infrastructure, cluster-level resources |

The Crossplane v2 example in this repo uses `Namespaced`. Your team's developers create `WebService` objects inside their own namespace.

### `spec.group`

Your organization's domain, used as the API group for all your custom resources. Convention is to use your company's domain reversed, like a Go module path. Examples:
- `platform.company.io`
- `infra.myorg.dev`
- `example.crossplane.io` (as used in this repo)

Keep it consistent across all your XRDs - it is the namespace for your entire platform API.

### `spec.versions`

You can run multiple versions simultaneously, giving teams time to migrate:

```yaml
versions:
- name: v1alpha1
  served: true
  referenceable: false       # Old version - still accepted but Compositions use v1
- name: v1
  served: true
  referenceable: true        # Compositions reference this version
```

`referenceable: true` means a Composition's `spec.compositeTypeRef` can point at this version. Only one version should be `referenceable` at a time.

### `spec` vs `status` Fields

By strong convention in Crossplane:

```
spec:   INPUTS  - what the developer writes
status: OUTPUTS - what Crossplane writes back after creating resources
```

The `status` fields in the XRD schema get populated by `ToCompositeFieldPath` patches in the Composition (you will see these in Chapter 04). The XR becomes the single source of truth for both config and runtime state.

---

## OpenAPI v3 Schema Reference

The `openAPIV3Schema` section is JSON Schema with Kubernetes extensions. It defines what fields are allowed (and required) on your custom resource.

The snippets below are **abbreviated** - they show only the `properties` block. In a real XRD they live here:

```
spec.versions[*].schema.openAPIV3Schema.properties.spec.properties
```

That is, each field you define under `spec:` in your XRD schema maps to one entry in that `properties` block.

### Basic Types

```yaml
properties:
  image:
    type: string
    description: OCI container image with tag.

  replicas:
    type: integer
    default: 1
    minimum: 1
    maximum: 20

  enabled:
    type: boolean
    default: false

  weight:
    type: number            # float64
```

### Constraints

```yaml
properties:
  environment:
    type: string
    default: production
    enum:                   # Only these exact values are accepted
    - development
    - staging
    - production

  port:
    type: integer
    default: 80
    enum: [80, 443, 8080, 8443]

  image:
    type: string
    pattern: '^[a-z0-9/._-]+:[a-z0-9._-]+$'   # Must include a tag (regex)
```

### Objects (Nested Maps)

```yaml
properties:
  autoscaling:
    type: object
    properties:
      enabled:
        type: boolean
        default: false
      minReplicas:
        type: integer
        default: 2
      maxReplicas:
        type: integer
        default: 10
```

### Free-Form Maps (`map[string]string`)

```yaml
properties:
  config:
    type: object
    description: Key-value pairs injected as environment variables.
    additionalProperties:
      type: string
```

This lets developers pass arbitrary environment variables:

```yaml
spec:
  config:
    LOG_LEVEL: debug
    FEATURE_FLAGS: "auth,payments"
    MAX_POOL_SIZE: "20"
```

### Arrays

```yaml
properties:
  ports:
    type: array
    items:
      type: integer
```

---

## How Versions and CRDs Relate

When Crossplane processes your XRD, it creates one CRD that serves all your versions. The CRD's `spec.versions` mirrors your XRD's `spec.versions`. This is how Kubernetes supports multiple API versions for the same resource type simultaneously.

```bash
# After applying an XRD, inspect the generated CRD
kubectl get crd webservices.platform.example.io -o yaml | grep -A 5 "versions:"
```

---

## Hands-On: Build a WebService XRD

You will define a `WebService` resource that is richer than the starter `App` - with replicas, port, environment, free-form config, and meaningful status fields.

```bash
mkdir -p practice/ch03
```

### Step 1: Write the XRD

Create `practice/ch03/webservice-xrd.yaml`:

```yaml
apiVersion: apiextensions.crossplane.io/v2         # Crossplane v2 XRD API
kind: CompositeResourceDefinition                  # Tells Crossplane to create a CRD and watch for this Kind
metadata:
  name: webservices.platform.example.io            # MUST be <plural>.<group> - Kubernetes enforces this
spec:
  scope: Namespaced                                # XR objects live in a namespace (not cluster-wide)
  group: platform.example.io                       # API group - your org's domain
  names:
    kind: WebService                               # PascalCase - what developers write in their YAML
    plural: webservices                            # Lowercase plural - used in kubectl and API URL paths
  versions:
  - name: v1alpha1                                 # Version string - signals this API is not yet stable
    served: true                                   # API server accepts this version
    referenceable: true                            # Compositions can reference this version
    schema:
      openAPIV3Schema:                             # Everything below here is JSON Schema for validation
        type: object                               # The root resource is an object (always true)
        properties:
          spec:                                    # Defines the shape of spec: (developer inputs)
            type: object
            description: WebService desired configuration.
            properties:
              image:
                type: string                       # Must be a string
                description: "OCI container image with tag, e.g. nginx:alpine."
              replicas:
                type: integer                      # Must be a whole number
                default: 1                         # Used if the developer omits this field
                minimum: 1                         # Kubernetes rejects values below this
                maximum: 10                        # Kubernetes rejects values above this
                description: Number of pod replicas to run.
              port:
                type: integer
                default: 80                        # Defaults to port 80 if omitted
                description: Port the container listens on.
              environment:
                type: string
                default: production                # Defaults to production if omitted
                enum:                              # Only these exact values are accepted
                - development
                - staging
                - production
                description: Runtime environment name used for labeling and defaults.
              config:
                type: object                       # A nested object (map) of arbitrary keys
                description: Key-value pairs injected into the container as environment variables.
                additionalProperties:
                  type: string                     # Every value in the map must be a string
            required:
            - image                                # Kubernetes rejects the resource if image is missing
          status:                                  # Defines the shape of status: (Crossplane-written outputs)
            type: object
            description: WebService observed state.
            properties:
              ready:
                type: boolean                      # True/false - Crossplane writes this after reconciling
                description: True when all replicas are available.
              replicas:
                type: integer                      # Crossplane copies this from the Deployment's status
                description: Number of currently available pod replicas.
              clusterIP:
                type: string                       # Crossplane copies this from the Service's spec
                description: Internal ClusterIP assigned to the Service.
```

### Step 2: Apply the XRD

```bash
kubectl apply -f practice/ch03/webservice-xrd.yaml
```

Watch Crossplane establish the CRD:

```bash
kubectl get xrds --watch
```

Wait until `ESTABLISHED` shows `True`:

```
NAME                              ESTABLISHED   OFFERED   AGE
webservices.platform.example.io   True                    8s
```

Press `Ctrl+C`.

### Step 3: Verify the CRD Was Created

```bash
kubectl get crds | grep platform.example.io
```

```
webservices.platform.example.io   2025-01-01T00:00:00Z
```

Crossplane created this CRD from your XRD. The Kubernetes API server now accepts `kind: WebService`.

### Step 4: Inspect the CRD Schema

```bash
kubectl get crd webservices.platform.example.io -o jsonpath='{.spec.versions[0].schema.openAPIV3Schema.properties.spec}' | python3 -m json.tool | head -40
```

You will see the full validated schema Kubernetes uses for admission control.

### Step 5: Test Schema Validation - Invalid Resource

Try applying a `WebService` missing the required `image` field:

```bash
kubectl apply -f - <<EOF
apiVersion: platform.example.io/v1alpha1
kind: WebService
metadata:
  name: invalid-test
  namespace: default
spec:
  replicas: 2
EOF
```

You should get an admission error immediately:

```
The WebService "invalid-test" is invalid: spec.image: Required value
```

Kubernetes rejected it before Crossplane ever saw it. This is schema-first API design: catch misconfigurations at commit/apply time, not at runtime.

### Step 6: Test Schema Validation - Wrong Enum Value

```bash
kubectl apply -f - <<EOF
apiVersion: platform.example.io/v1alpha1
kind: WebService
metadata:
  name: invalid-env
  namespace: default
spec:
  image: nginx:alpine
  environment: production-eu
EOF
```

Expected error:

```
The WebService "invalid-env" is invalid: spec.environment: Unsupported value:
  "production-eu": supported values: "development", "staging", "production"
```

### Step 7: Apply a Valid WebService

This will be accepted but will sit `Synced: False` until you write the Composition in Chapter 04:

```bash
kubectl apply -f - <<EOF
apiVersion: platform.example.io/v1alpha1
kind: WebService
metadata:
  name: schema-test
  namespace: default
spec:
  image: nginx:alpine
  replicas: 1
  environment: development
EOF
```

Check its status:

```bash
kubectl describe webservice schema-test -n default
```

Look at the `Conditions` - you will see something like `Synced: False, Reason: CompositionNotFound` because no Composition matches yet. That is expected - you will fix it in Chapter 04.

Delete it for now:

```bash
kubectl delete webservice schema-test -n default
```

---

## What You Built

- A `WebService` XRD with five input fields: `image`, `replicas`, `port`, `environment`, `config`
- Schema validation for required fields, integer range constraints, and enum values
- Three status output fields: `ready`, `replicas`, `clusterIP`
- The CRD that Crossplane auto-generated and registered with the API server
- Hands-on proof that the schema validation fires at admission time, not reconciliation time

The XRD is the API contract between your platform team and your developers. In Chapter 04 you write the Composition that fulfills that contract.

---

➡️ [Crossplane  04: Compositions & Go Templating](04-compositions.md)
