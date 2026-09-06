# Chapter 00: YAML Primer - Objects, Arrays & Reading Compositions

> **Goal:** Understand YAML syntax well enough to read `xrd.yaml` and `composition.yaml` with confidence.

---

## YAML vs JSON - Side by Side

YAML and JSON represent the same data structures. YAML just uses indentation instead of braces and brackets.

### Object (key → value pairs)

```json
{
  "name": "my-api",
  "port": 8080,
  "enabled": true
}
```

```yaml
name: my-api
port: 8080
enabled: true
```

A YAML object is just lines of `key: value`. No braces, no quotes needed (unless the value contains special characters).

---

### Nested Object (object inside object)

```json
{
  "spec": {
    "image": "nginx",
    "replicas": 2
  }
}
```

```yaml
spec:
  image: nginx
  replicas: 2
```

Nesting is done with **indentation** (2 spaces per level is standard). The parent key (`spec`) has no value on its own line - its children are indented beneath it.

---

### Array (list of values)

```json
{
  "ports": [80, 443, 8080]
}
```

```yaml
ports:
- 80
- 443
- 8080
```

Each item in a YAML array starts with `- `. The dash is the equivalent of a `[...]` entry.

---

### Array of Objects

```json
{
  "containers": [
    { "name": "app", "image": "nginx" },
    { "name": "sidecar", "image": "envoy" }
  ]
}
```

```yaml
containers:
- name: app
  image: nginx
- name: sidecar
  image: envoy
```

Each `- ` starts a new object in the array. The fields of that object are indented under the dash.

---

### Deeply Nested - What You See in Kubernetes YAML

```json
{
  "spec": {
    "template": {
      "spec": {
        "containers": [
          {
            "name": "app",
            "image": "nginx",
            "ports": [{ "containerPort": 80 }]
          }
        ]
      }
    }
  }
}
```

```yaml
spec:
  template:
    spec:
      containers:
      - name: app
        image: nginx
        ports:
        - containerPort: 80
```

Each level of nesting = 2 more spaces of indentation.

---

## How Field Paths Work

When you see `spec.template.spec.containers[0].image` in a Crossplane patch, it is just dot-notation for navigating the nested structure above:

```
spec
  └── template
        └── spec
              └── containers    (an array)
                    └── [0]     (first item in the array)
                          └── image
```

`[0]` means "the first item" (arrays are zero-indexed). `containers[0].image` means "the `image` field of the first container object."

---

## Tracing `spec.image` From XRD → Composition → Deployment

This is the full journey of the `image` field in this repo.

### Step 1 - XRD declares `image` as a valid input field

In [xrd.yaml](../../practice/ch01/xrd.yaml):

```yaml
spec:
  group: example.crossplane.io
  names:
    kind: App          # ← The custom type this XRD defines
  versions:
  - name: v1
    schema:
      openAPIV3Schema:
        properties:
          spec:
            properties:
              image:           # ← "image" is declared here as a string field
                type: string
            required:
            - image            # ← It is required - you must provide it
```

This tells Kubernetes: "An `App` object must have a `spec.image` field of type string." Without this, `kubectl apply` would reject it.

---

### Step 2 - Developer provides the value in their XR

In [app.yaml](../../practice/ch01/app.yaml), the developer writes:

```yaml
apiVersion: example.crossplane.io/v1
kind: App
metadata:
  name: my-app
spec:
  image: nginx        # ← Developer sets the value here
```

Crossplane stores this as the live XR object in the cluster. `spec.image` is now `"nginx"`.

---

### Step 3 - Composition copies `spec.image` into the Deployment

In [composition.yaml](../../practice/ch01/composition.yaml), this is the relevant patch:

```yaml
patches:
- type: FromCompositeFieldPath   # ← Direction: FROM the XR, INTO the composed resource
  fromFieldPath: spec.image      # ← Read this path on the XR   → value: "nginx"
  toFieldPath: spec.template.spec.containers[0].image  # ← Write it here on the Deployment
```

Breaking down `toFieldPath: spec.template.spec.containers[0].image`:

```
Deployment YAML structure:

spec:                        ← spec
  template:                  ← .template
    spec:                    ← .spec
      containers:            ← .containers
      - name: app            ← [0]  (first item in the containers array)
        image: ???           ← .image   ← THIS is where "nginx" lands
```

The `base` in the Composition defines the skeleton Deployment. It has no `image` set:

```yaml
base:
  apiVersion: apps/v1
  kind: Deployment
  spec:
    replicas: 2
    template:
      spec:
        containers:
        - name: app
          ports:
          - containerPort: 80
          # ↑ No image here - it will be filled in by the patch
```

After the patch runs, Crossplane produces a Deployment equivalent to:

```yaml
apiVersion: apps/v1
kind: Deployment
spec:
  replicas: 2
  template:
    spec:
      containers:
      - name: app
        image: nginx        # ← Patched in from XR.spec.image
        ports:
        - containerPort: 80
```

---

## The Two Patch Directions

| Type | Direction | What it does |
|------|-----------|--------------|
| `FromCompositeFieldPath` | XR → composed resource | Copies input values from the developer's YAML into the resources Crossplane creates |
| `ToCompositeFieldPath` | composed resource → XR | Writes status back onto the XR (e.g. the Deployment's `availableReplicas` appears on `App.status.replicas`) |

The `ToCompositeFieldPath` patches are how the XR becomes a single source of truth - you can `kubectl describe app my-app` and see both what you put in *and* what the cluster reported back.

---

## Quick Reference: YAML Gotchas

| Situation | Rule |
|-----------|------|
| Value starts with `{` or `[` | Wrap in quotes: `name: "[my-app]"` |
| Value is `true`/`false`/`null` as a string | Wrap in quotes: `enabled: "true"` |
| Multiline string (e.g. a template) | Use `\|` block scalar - each indented line is part of the string |
| Tabs vs spaces | **Always spaces.** YAML does not allow tabs for indentation. |
| Array item fields must align | The fields under a `- ` must be indented further than the dash |

---

➡️ [Chapter 01: CRD's and Controllers](01-crds-crs-controllers.md)
