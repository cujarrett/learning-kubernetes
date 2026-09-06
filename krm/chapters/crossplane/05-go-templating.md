# Chapter 05: Go Templating Deep Dive

> **You will build:** That same Composition rewritten using Go templates (`function-go-templating`).

You wrote your first Go template Composition in Chapter 04. This chapter covers the patterns you will reach for as templates grow more complex: Sprig helpers, nil-safe map access, writing status back to the XR, and reusable template blocks.

---

## Sprig: The Helper Library Built Into `function-go-templating`

[Sprig](http://masterminds.github.io/sprig/) provides over 100 helper functions available in every `function-go-templating` template. Here are the ones you will use most:

### `default` - Safe Fallback Values

```
{{ $spec.replicas | default 1 }}
{{ $spec.environment | default "production" }}
{{ $spec.image | default "nginx:alpine" }}
```

Without `default`, accessing a field that was not provided in the XR spec will render as `<no value>` in the YAML, which usually breaks the resource.

### `quote` - Wrap a Value in Quotes

```
{{ $val | quote }}
```

Use this in ConfigMap `data:` and anywhere the value must be a YAML string, not an unquoted scalar.

### `upper` / `lower` / `title` - Case Transforms

```
{{ $spec.environment | upper }}        # development → DEVELOPMENT
{{ $spec.name | lower }}
{{ $spec.tier | title }}               # web → Web
```

### `trunc` - Truncate to the Kubernetes Label Limit

Kubernetes label values must be 63 characters or fewer:

```
{{ $name | trunc 63 | trimSuffix "-" }}
```

The `trimSuffix` removes a trailing dash that `trunc` may leave if the name ends mid-word.

### `printf` - Format Strings

```
{{ printf "%s-%s" $ns $name }}
{{ printf "app/%s:v%s" $spec.image $spec.version }}
```

Equivalent to `fmt.Sprintf` in Go.

### `int` - Convert to Integer for Arithmetic

```
{{ $spec.replicas | int | mul 2 }}     # double the replica count
{{ add 1 ($spec.port | int) }}         # port + 1
```

### `has` - Check if a Value is in a List

```
{{- if has $spec.environment (list "production" "staging") }}
  # only render in production or staging
{{- end }}
```

### `typeOf` - Inspect the Type of a Value

Useful when debugging templates that behave unexpectedly:

```
{{ typeOf $spec.replicas }}   # outputs "float64" (JSON numbers decode as float64)
```

---

## Nil-Safe Map Access With `default dict`

When an XR spec contains a nested object that the user might not provide, accessing its fields directly will panic the template engine:

```yaml
# XRD schema defines:
spec:
  autoscaling:
    enabled: bool
    minReplicas: int
    maxReplicas: int
```

If a user creates an XR without `spec.autoscaling`, then `$spec.autoscaling.enabled` panics with a nil pointer error.

**The fix: `default dict`**

```
{{- $autoscaling := $spec.autoscaling | default dict }}
```

`dict` produces an empty map `{}`. `default dict` returns that empty map if `$spec.autoscaling` is nil or missing. After this line, `$autoscaling.enabled` safely returns `<no value>` / falsy instead of panicking.

Full pattern:

```
{{- $autoscaling := $spec.autoscaling | default dict }}
{{- if $autoscaling.enabled }}
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: {{ $name }}
  namespace: {{ $ns }}
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: {{ $name }}
  minReplicas: {{ $autoscaling.minReplicas | default 2 }}
  maxReplicas: {{ $autoscaling.maxReplicas | default 10 }}
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
{{- end }}
```

Apply `default dict` to any optional nested object in `$spec` before accessing its sub-fields.

---

## Writing Status Back to the XR

Your Composition can write computed values back to the XR's `status` fields. This surfaces runtime data (ClusterIP, actual replica count, readiness) to anything watching the XR.

The special resource name `composite` in the desired composed resources targets the XR itself:

```yaml
template: |
  {{- $name := .observed.composite.resource.metadata.name }}
  {{- $ns   := .observed.composite.resource.metadata.namespace }}
  {{- $spec := .observed.composite.resource.spec }}
  ---
  apiVersion: apps/v1
  kind: Deployment
  ...
  ---
  # Write status back to the XR. The name "composite" is special.
  apiVersion: {{ .observed.composite.resource.apiVersion }}
  kind: {{ .observed.composite.resource.kind }}
  metadata:
    name: {{ $name }}
    namespace: {{ $ns }}
    annotations:
      gotemplating.fn.crossplane.io/composition-resource-name: composite
  status:
    endpoint: "http://{{ $name }}.{{ $ns }}.svc.cluster.local:8080"
    environment: {{ $spec.environment | default "production" }}
```

After applying the XR and waiting for reconcile, the status fields are readable via jsonpath (you'll verify this in the hands-on below):

```bash
# Run this after completing the hands-on steps
kubectl get webservice svc-production -n default -o jsonpath='{.status.endpoint}'
# http://svc-production.default.svc.cluster.local:8080
```

You can define whatever status fields you want as long as they are declared in the XRD's `spec.versions[].schema.openAPIV3Schema.properties.status.properties` block.

---

## Reusable Template Blocks With `define` and `include`

When the same YAML fragment appears in multiple resources (standard labels, owner references, resource limits), you can define it once and include it:

```yaml
template: |
  {{- $name := .observed.composite.resource.metadata.name }}
  {{- $ns   := .observed.composite.resource.metadata.namespace }}
  {{- $spec := .observed.composite.resource.spec }}

  {{/* Define a reusable labels block. 'dot' is passed as context. */}}
  {{- define "commonLabels" }}
  labels:
    app: {{ .name }}
    environment: {{ .env | default "production" }}
    managed-by: crossplane
  {{- end }}

  ---
  apiVersion: apps/v1
  kind: Deployment
  metadata:
    name: {{ $name }}
    namespace: {{ $ns }}
    {{- include "commonLabels" (dict "name" $name "env" $spec.environment) | indent 4 }}
  spec:
    replicas: {{ $spec.replicas | default 1 }}
    selector:
      matchLabels:
        app: {{ $name }}
    template:
      metadata:
        {{- include "commonLabels" (dict "name" $name "env" $spec.environment) | indent 8 }}
      ...
  ---
  apiVersion: v1
  kind: Service
  metadata:
    name: {{ $name }}
    namespace: {{ $ns }}
    {{- include "commonLabels" (dict "name" $name "env" $spec.environment) | indent 4 }}
  ...
```

For example, `{{- include "commonLabels" (dict "name" "my-webservice" "env" "production") | indent 4 }}` renders to:

```yaml
    labels:
      app: my-webservice
      environment: production
      managed-by: crossplane
```

The `indent 4` shifts every line right by 4 spaces, aligning the block under `metadata:`. The `indent 8` call used inside `template.metadata` shifts it by 8 instead.

`include` returns the rendered string which you can pipe through `indent N` to get the right YAML indentation. `dict` constructs an ad-hoc map to pass as the template context.

This pattern is identical to Helm's `_helpers.tpl` approach - if you have used Helm before, it is the same idea.

---

## Environment-Driven Replica Defaults

A common pattern: different replica defaults per environment, overridable by the user:

```
{{- $env      := $spec.environment | default "production" }}
{{- $replicas := $spec.replicas | default 0 | int }}

{{/* Set environment-appropriate defaults if the user did not specify */}}
{{- if eq $replicas 0 }}
  {{- if eq $env "production" }}
    {{- $replicas = 3 }}
  {{- else if eq $env "staging" }}
    {{- $replicas = 2 }}
  {{- else }}
    {{- $replicas = 1 }}
  {{- end }}
{{- end }}
```

Then use `$replicas` in the Deployment spec:

```
spec:
  replicas: {{ $replicas }}
```

Note that `$replicas = 3` (without `:=`) is a reassignment of an existing variable. This only works in Go templates if the variable was declared with `:=` in the same or outer scope.

---

## Hands-On: A Multi-Environment WebService With Status Writeback

You will extend the WebService Composition from Chapter 04 with environment-driven replica defaults, nil-safe HPA, and status writeback. This uses the same XRD from Chapter 03 - but requires its own schema.

```bash
mkdir -p practice/ch05
```

### Prerequisite: Grant Crossplane Permission to Manage HPAs

Crossplane's default install does not include RBAC rules for `horizontalpodautoscalers`. Without this, Crossplane will reconcile forever but the HPA will never appear - the only indication is a `forbidden` warning in the XR's events.

Run this once before starting the hands-on:

```bash
kubectl create clusterrole crossplane-hpa-manager \
  --verb=get,list,watch,create,update,patch,delete \
  --resource=horizontalpodautoscalers

kubectl create clusterrolebinding crossplane-hpa-manager \
  --clusterrole=crossplane-hpa-manager \
  --serviceaccount=crossplane-system:crossplane
```

### Step 0: Apply the Chapter 05 XRD

This chapter needs two schema changes that would break Chapter 03's hands-on if made in place, so ch05 ships its own XRD.

**Change 1 - remove `default: 1` from `replicas`.** The ch03 XRD defaults replicas to 1. Kubernetes applies schema defaults server-side, before the template runs - so `$spec.replicas` is always `1`, never nil. The template's `$spec.replicas | default 0` sentinel never fires and the environment-driven logic is permanently skipped. Removing the default leaves the field nil when omitted, so the sentinel works.

**Change 2 - add the `autoscaling` object.** XRDs reject any field not declared in the schema. Without an `autoscaling` entry, applying `svc-staging` with `spec.autoscaling` would fail with a strict decoding error.

Create `practice/ch05/webservice-xrd.yaml`:

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
                minimum: 1                         # Kubernetes rejects values below this
                maximum: 10                        # Kubernetes rejects values above this
                description: Number of pod replicas to run. Omit to use environment-driven defaults.
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
              autoscaling:
                type: object
                description: Optional autoscaling configuration. Omit to use a static replica count.
                properties:
                  enabled:
                    type: boolean
                    description: When true, an HPA is created and replicas is managed dynamically.
                  minReplicas:
                    type: integer
                    minimum: 1
                    description: Minimum number of replicas for the HPA.
                  maxReplicas:
                    type: integer
                    minimum: 1
                    description: Maximum number of replicas for the HPA.
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

Apply it:

```bash
kubectl apply -f practice/ch05/webservice-xrd.yaml
```

### Step 1: Write the Enhanced Composition

Create `practice/ch05/webservice-advanced-composition.yaml`:

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: webservice-advanced-composition
  labels:
    channel: advanced
spec:
  compositeTypeRef:
    apiVersion: platform.example.io/v1alpha1
    kind: WebService
  mode: Pipeline
  pipeline:
  - step: render-webservice
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
          {{- $env  := $spec.environment | default "production" }}

          {{/* Environment-driven replica default */}}
          {{- $replicas := $spec.replicas | default 0 | int }}
          {{- if eq $replicas 0 }}
            {{- if eq $env "production" }}
              {{- $replicas = 3 }}
            {{- else if eq $env "staging" }}
              {{- $replicas = 2 }}
            {{- else }}
              {{- $replicas = 1 }}
            {{- end }}
          {{- end }}

          {{/* Nil-safe autoscaling object */}}
          {{- $autoscaling := $spec.autoscaling | default dict }}

          ---
          apiVersion: apps/v1
          kind: Deployment
          metadata:
            name: {{ $name }}
            namespace: {{ $ns }}
            annotations:
              gotemplating.fn.crossplane.io/composition-resource-name: deployment
            labels:
              app: {{ $name }}
              environment: {{ $env }}
              managed-by: crossplane
          spec:
            {{- if not $autoscaling.enabled }}
            replicas: {{ $replicas }}
            {{- end }}
            selector:
              matchLabels:
                app: {{ $name }}
            template:
              metadata:
                labels:
                  app: {{ $name }}
                  environment: {{ $env }}
              spec:
                containers:
                - name: app
                  image: {{ $spec.image }}
                  ports:
                  - containerPort: {{ $spec.port | default 80 }}
          ---
          apiVersion: v1
          kind: Service
          metadata:
            name: {{ $name }}
            namespace: {{ $ns }}
            annotations:
              gotemplating.fn.crossplane.io/composition-resource-name: service
            labels:
              app: {{ $name }}
          spec:
            selector:
              app: {{ $name }}
            ports:
            - port: 8080
              targetPort: {{ $spec.port | default 80 }}
              protocol: TCP
          {{- if $autoscaling.enabled }}
          ---
          apiVersion: autoscaling/v2
          kind: HorizontalPodAutoscaler
          metadata:
            name: {{ $name }}
            namespace: {{ $ns }}
            annotations:
              gotemplating.fn.crossplane.io/composition-resource-name: hpa
            labels:
              app: {{ $name }}
          spec:
            scaleTargetRef:
              apiVersion: apps/v1
              kind: Deployment
              name: {{ $name }}
            minReplicas: {{ $autoscaling.minReplicas | default 2 }}
            maxReplicas: {{ $autoscaling.maxReplicas | default 10 }}
            metrics:
            - type: Resource
              resource:
                name: cpu
                target:
                  type: Utilization
                  averageUtilization: 70
          {{- end }}
          {{- if $spec.config }}
          ---
          apiVersion: v1
          kind: ConfigMap
          metadata:
            name: {{ $name }}-config
            namespace: {{ $ns }}
            annotations:
              gotemplating.fn.crossplane.io/composition-resource-name: configmap
            labels:
              app: {{ $name }}
          data:
          {{- range $key, $val := $spec.config }}
            {{ $key }}: {{ $val | quote }}
          {{- end }}
          {{- end }}
```

Apply it:

```bash
kubectl apply -f practice/ch05/webservice-advanced-composition.yaml
```

### Step 2: Create a Production WebService (No Autoscaling)

Create `practice/ch05/svc-production.yaml`:

> **Note:** Two compositions now target `WebService` (the ch04 one and this chapter's `webservice-advanced-composition`). Without an explicit selector, Crossplane picks one arbitrarily. Use `compositionSelector` to ensure the right one is used.

```yaml
apiVersion: platform.example.io/v1alpha1
kind: WebService
metadata:
  name: svc-production
  namespace: default
spec:
  crossplane:
    compositionSelector:
      matchLabels:
        channel: advanced
  image: nginx:alpine
  port: 80
  environment: production
  config:
    LOG_LEVEL: warn
    APP_NAME: svc-production
```

```bash
kubectl apply -f practice/ch05/svc-production.yaml
kubectl get deployments -n default --watch
# Ctrl+C when svc-production Deployment shows 3/3 READY
```

### Step 3: Check the Replica Count

```bash
kubectl get deployment svc-production -o jsonpath='{.spec.replicas}'
# 3
```

No `spec.replicas` was set - the environment-driven default of 3 applied.

### Step 4: Create a Staging WebService With Autoscaling

Create `practice/ch05/svc-staging.yaml`:

```yaml
apiVersion: platform.example.io/v1alpha1
kind: WebService
metadata:
  name: svc-staging
  namespace: default
spec:
  crossplane:
    compositionSelector:
      matchLabels:
        channel: advanced
  image: nginx:alpine
  port: 80
  environment: staging
  autoscaling:
    enabled: true
    minReplicas: 1
    maxReplicas: 5
```

```bash
kubectl apply -f practice/ch05/svc-staging.yaml
kubectl get deployments -n default --watch
# Ctrl+C when svc-staging Deployment appears READY
```

Then check that the HPA was created:

```bash
kubectl get hpa -n default
```

Verify the Deployment's replica field was not set statically by the template (Kubernetes defaults it to `1`; the HPA then owns it from that point on):

```bash
kubectl get deployment svc-staging -o jsonpath='{.spec.replicas}'
# 1 - Kubernetes injected the default since the template omitted replicas:
# The HPA now controls scaling, not the Deployment spec directly
```

### Step 5: Update Autoscaling Config In-Place

```bash
kubectl patch webservice svc-staging -n default \
  --type merge \
  -p '{"spec":{"autoscaling":{"minReplicas":2,"maxReplicas":8}}}'
```

```bash
kubectl get hpa svc-staging -n default
# MINPODS should now show 2, MAXPODS 8
```

### Step 6: Clean Up

```bash
# XRs (deletes all composed resources via cascade)
kubectl delete webservice svc-production svc-staging -n default --ignore-not-found

# HPA RBAC grant added in the prerequisite step
kubectl delete clusterrolebinding crossplane-hpa-manager --ignore-not-found
kubectl delete clusterrole crossplane-hpa-manager --ignore-not-found

# Composition and XRD from this chapter
kubectl delete composition webservice-advanced-composition --ignore-not-found
kubectl delete -f practice/ch05/webservice-xrd.yaml --ignore-not-found
```

---

## What You Built

- Updated the XRD to remove the `replicas` default so environment-driven logic can take effect
- Environment-driven replica defaults using template variable reassignment
- Nil-safe `default dict` pattern for optional nested spec objects
- Conditional HPA: created when `spec.autoscaling.enabled` is true, absent otherwise
- Deployment `replicas:` field omitted when HPA is active (so HPA can own it)
- The `range` loop and conditional `ConfigMap` pattern from Chapter 04, now in a larger template
- `compositionSelector` on XRs to route to a specific composition when multiple exist

Chapter 06 covers Composition Revisions - how to roll out Composition changes safely and pin specific XR instances to a specific version.

---

➡️ [Crossplane 06: Composition Revisions](06-composition-revisions.md)
