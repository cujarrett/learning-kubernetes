# Chapter 09: Advanced Go Templating

> **You will build:** A `MicroService` XRD with optional HPA using Go template conditionals and nil-safe patterns.

This chapter builds a production-grade `MicroService` XRD and Composition using Go templates. You will apply conditionals, environment-driven defaults, and an optional HPA - all from a single Go template.

By the end, you will have a custom resource that your imaginary company's developers would genuinely use.

---

## New XRD: `MicroService`

The `MicroService` XRD adds concepts not in `WebService`:

```
spec:
  image              string   required
  replicas           int      default 2
  port               int      default 8080
  environment        string   enum: development | staging | production
  config             map      optional - env vars
  autoscaling:
    enabled          bool     default false
    minReplicas      int      default 2
    maxReplicas      int      default 10
    targetCPU        int      default 70   (percent)
```

The Composition produces:
- Always: Deployment, Service, ConfigMap
- Only when `autoscaling.enabled=true`: HorizontalPodAutoscaler

And the environment drives replica defaults:
- `development` → 1 replica
- `staging` → 2 replicas
- `production` → 3 replicas (overridden by `spec.replicas` if provided)

---

## Template Patterns Covered in This Chapter

### Pattern 1: Environment-Driven Defaults with a Variable

```go
{{- $replicas := .observed.composite.resource.spec.replicas | default 2 }}
{{- if eq .observed.composite.resource.spec.environment "production" }}
  {{- $replicas = 3 }}
{{- else if eq .observed.composite.resource.spec.environment "development" }}
  {{- $replicas = 1 }}
{{- end }}
```

Note: `$replicas = 3` (no `:=`) because the variable was already declared. This mirrors Go variable reuse after the initial declaration.

### Pattern 2: Conditional Resource Block

```go
{{- if .observed.composite.resource.spec.autoscaling.enabled }}
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
...
{{- end }}
```

The `---` document separator is inside the `if` block - the entire YAML document is omitted when the condition is false.

### Pattern 3: Conditional Replicas Field

When autoscaling is enabled, do NOT set a static `replicas` on the Deployment - the HPA manages that value. Setting `replicas` would fight with the HPA:

```go
{{- if not .observed.composite.resource.spec.autoscaling.enabled }}
  replicas: {{ $replicas }}
{{- end }}
```

### Pattern 4: Nested Object Nil-Safety

When accessing a nested field like `.observed.composite.resource.spec.autoscaling.minReplicas`, if `autoscaling` itself is nil (not provided), the template will panic.

Use `hasKey` from Sprig to guard it:

```go
{{- if and (hasKey .observed.composite.resource.spec "autoscaling") .observed.composite.resource.spec.autoscaling.enabled }}
```

Or use a safe accessor helper by assigning the nested object to a variable first:

```go
{{- $as := .observed.composite.resource.spec.autoscaling | default dict }}
{{- $asEnabled := $as.enabled | default false }}
```

`dict` is a Sprig function that returns an empty map `{}`. This makes the later `$as.enabled` safe even when `autoscaling` is absent from the spec.

### Pattern 5: Writing Status Back

`function-go-templating` can render a special patched XR document that writes status fields back. Include it as the last document in your template:

```go
---
apiVersion: {{ .observed.composite.resource.apiVersion }}
kind: {{ .observed.composite.resource.kind }}
metadata:
  name: {{ .observed.composite.resource.metadata.name }}
  namespace: {{ .observed.composite.resource.metadata.namespace }}
status:
  ready: true
  message: "Rendered by go-templating"
```

---

## Hands-On: Build the MicroService Custom Resource

```bash
mkdir -p practice/ch09
```

### Prerequisites

This chapter needs `function-go-templating` (from Chapter 04) and the `team-alpha` namespace (from Chapter 08):

```bash
kubectl apply -f practice/ch04/function-go-templating.yaml
kubectl get functions.pkg.crossplane.io --watch
# Wait for HEALTHY=True, then Ctrl+C

kubectl apply -f practice/ch08/namespaces.yaml
```

### Step 1: Write the MicroService XRD

Create `practice/ch09/microservice-xrd.yaml`:

```yaml
apiVersion: apiextensions.crossplane.io/v2
kind: CompositeResourceDefinition
metadata:
  name: microservices.platform.example.io
spec:
  scope: Namespaced
  group: platform.example.io
  names:
    kind: MicroService
    plural: microservices
  versions:
  - name: v1alpha1
    served: true
    referenceable: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              image:
                type: string
                description: "Container image including tag."
              replicas:
                type: integer
                description: "Replica count. If autoscaling is enabled, this sets the initial value."
                minimum: 1
                maximum: 20
              port:
                type: integer
                description: "Port the container listens on."
                default: 8080
              environment:
                type: string
                default: production
                enum:
                - development
                - staging
                - production
              config:
                type: object
                description: "Key-value pairs injected as environment variables."
                additionalProperties:
                  type: string
              autoscaling:
                type: object
                description: "Horizontal autoscaling settings."
                properties:
                  enabled:
                    type: boolean
                    default: false
                  minReplicas:
                    type: integer
                    default: 2
                    minimum: 1
                  maxReplicas:
                    type: integer
                    default: 10
                    minimum: 1
                  targetCPUUtilization:
                    type: integer
                    default: 70
                    minimum: 10
                    maximum: 100
                    description: "Target average CPU utilization percent to trigger scaling."
            required:
            - image
          status:
            type: object
            properties:
              ready:
                type: boolean
              replicas:
                type: integer
              clusterIP:
                type: string
              autoscalingEnabled:
                type: boolean
```

Apply it:

```bash
kubectl apply -f practice/ch09/microservice-xrd.yaml
kubectl get xrds --watch
# Wait for microservices.platform.example.io ESTABLISHED=True, then Ctrl+C
```

### Step 2: Write the MicroService Go Template Composition

Create `practice/ch09/microservice-composition.yaml`:

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: microservice-composition
spec:
  compositeTypeRef:
    apiVersion: platform.example.io/v1alpha1
    kind: MicroService
  mode: Pipeline
  pipeline:
  - step: render-microservice
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

          {{- /* ── Autoscaling config with safe nil defaults ── */}}
          {{- $as         := $spec.autoscaling | default dict }}
          {{- $asEnabled  := $as.enabled           | default false }}
          {{- $asMin      := $as.minReplicas        | default 2 }}
          {{- $asMax      := $as.maxReplicas        | default 10 }}
          {{- $asCPU      := $as.targetCPUUtilization | default 70 }}

          {{- /* ── Environment-driven replica default ── */}}
          {{- $replicas := $spec.replicas | default 2 }}
          {{- if eq $spec.environment "development" }}
            {{- $replicas = 1 }}
          {{- else if eq $spec.environment "production" }}
            {{- $replicas = 3 }}
          {{- end }}
          {{- /* spec.replicas explicitly set overrides the environment default above */}}
          {{- if $spec.replicas }}
            {{- $replicas = $spec.replicas }}
          {{- end }}

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
              environment: {{ $spec.environment | default "production" }}
              managed-by: crossplane
          spec:
            {{- /* Do not set replicas when HPA manages them */}}
            {{- if not $asEnabled }}
            replicas: {{ $replicas }}
            {{- end }}
            selector:
              matchLabels:
                app: {{ $name }}
            template:
              metadata:
                labels:
                  app: {{ $name }}
                  environment: {{ $spec.environment | default "production" }}
              spec:
                containers:
                - name: app
                  image: {{ $spec.image }}
                  ports:
                  - containerPort: {{ $spec.port | default 8080 }}
                  resources:
                    requests:
                      cpu: 50m
                      memory: 64Mi
                    limits:
                      cpu: 200m
                      memory: 256Mi
                  {{- if $spec.config }}
                  envFrom:
                  - configMapRef:
                      name: {{ $name }}-config
                  {{- end }}

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
              managed-by: crossplane
          spec:
            selector:
              app: {{ $name }}
            ports:
            - port: 8080
              targetPort: {{ $spec.port | default 8080 }}
              protocol: TCP

          {{- if $spec.config }}
          ---
          apiVersion: v1
          kind: ConfigMap
          metadata:
            name: {{ $name }}-config
            namespace: {{ $ns }}
            annotations:
              gotemplating.fn.crossplane.io/composition-resource-name: config
            labels:
              app: {{ $name }}
              managed-by: crossplane
          data:
          {{- range $key, $val := $spec.config }}
            {{ $key }}: {{ $val | quote }}
          {{- end }}
          {{- end }}

          {{- if $asEnabled }}
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
              managed-by: crossplane
          spec:
            scaleTargetRef:
              apiVersion: apps/v1
              kind: Deployment
              name: {{ $name }}
            minReplicas: {{ $asMin }}
            maxReplicas: {{ $asMax }}
            metrics:
            - type: Resource
              resource:
                name: cpu
                target:
                  type: Utilization
                  averageUtilization: {{ $asCPU }}
          {{- end }}

          {{- $depAvailable := 0 }}
          {{- if .observed.resources.deployment }}
            {{- $depAvailable = .observed.resources.deployment.resource.status.availableReplicas | default 0 }}
          {{- end }}

          ---
          apiVersion: {{ .observed.composite.resource.apiVersion }}
          kind: {{ .observed.composite.resource.kind }}
          metadata:
            name: {{ .observed.composite.resource.metadata.name }}
            namespace: {{ .observed.composite.resource.metadata.namespace }}
          status:
            conditions:
            - lastTransitionTime: "1970-01-01T00:00:00Z"
              {{- if gt (int $depAvailable) 0 }}
              reason: Available
              status: "True"
              {{- else }}
              reason: Creating
              status: "False"
              message: "Waiting for Deployment to have available replicas"
              {{- end }}
              type: Ready
```

Apply it:

```bash
kubectl apply -f practice/ch09/microservice-composition.yaml
```

### Step 3: Deploy a MicroService Without Autoscaling

Create `practice/ch09/svc-development.yaml`:

```yaml
apiVersion: platform.example.io/v1alpha1
kind: MicroService
metadata:
  name: payments-service
  namespace: team-alpha
spec:
  image: nginx:alpine
  environment: development
  port: 8080
  config:
    LOG_LEVEL: debug
    SERVICE_NAME: payments
    TIMEOUT_SECONDS: "30"
```

Apply it:

```bash
kubectl apply -f practice/ch09/svc-development.yaml
kubectl get deployments,services,configmaps -n team-alpha
```

Check the replica count - `development` environment should have set `replicas: 1`:

```bash
kubectl get deployment payments-service -n team-alpha -o jsonpath='{.spec.replicas}'
```

Expected: `1`

No HPA should exist:

```bash
kubectl get hpa -n team-alpha
# Should print: No resources found in team-alpha namespace.
```

### Step 4: Grant Crossplane Permission to Manage HPAs

By default, Crossplane's service account can manage Deployments and Services but does not have permission to create `HorizontalPodAutoscaler` resources. Without this, any XR with `autoscaling.enabled=true` will fail with a forbidden error.

Create `practice/ch09/crossplane-hpa-rbac.yaml`:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: crossplane-hpa-manager
rules:
- apiGroups:
  - autoscaling
  resources:
  - horizontalpodautoscalers
  - horizontalpodautoscalers/status
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: crossplane-hpa-manager
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: crossplane-hpa-manager
subjects:
- kind: ServiceAccount
  name: crossplane
  namespace: crossplane-system
```

Apply it:

```bash
kubectl apply -f practice/ch09/crossplane-hpa-rbac.yaml
```

### Step 5: Deploy a MicroService With Autoscaling

Create `practice/ch09/svc-production.yaml`:

```yaml
apiVersion: platform.example.io/v1alpha1
kind: MicroService
metadata:
  name: orders-service
  namespace: team-alpha
spec:
  image: nginx:alpine
  environment: production
  port: 8080
  autoscaling:
    enabled: true
    minReplicas: 2
    maxReplicas: 8
    targetCPUUtilization: 60
  config:
    LOG_LEVEL: warn
    SERVICE_NAME: orders
```

Apply it:

```bash
kubectl apply -f practice/ch09/svc-production.yaml
kubectl get deployments,services,configmaps,hpa -n team-alpha
```

Verify the HPA was created:

```bash
kubectl get hpa orders-service -n team-alpha -o yaml
```

Confirm the Deployment has **no static replicas set** (the HPA owns that):

```bash
kubectl get deployment orders-service -n team-alpha -o jsonpath='{.spec.replicas}'
# May show 2 (the HPA's minReplicas default) or nil
```

### Step 6: Update Autoscaling at Runtime

```bash
kubectl patch microservice orders-service -n team-alpha \
  --type merge \
  -p '{"spec":{"autoscaling":{"maxReplicas":15,"targetCPUUtilization":50}}}'
```

Watch the HPA update:

```bash
kubectl get hpa orders-service -n team-alpha --watch
# You should see maxReplicas change to 15, then Ctrl+C
```

Crossplane reconciled the change from the MicroService XR to the HPA - you never touched the HPA directly.

### Step 7: Disable Autoscaling - XR-Driven Rollback

```bash
kubectl patch microservice orders-service -n team-alpha \
  --type merge \
  -p '{"spec":{"autoscaling":{"enabled":false}}}'
```

Watch the HPA disappear:

```bash
kubectl get hpa -n team-alpha --watch
# HPA is deleted by Crossplane, then Ctrl+C
```

And the Deployment should now have a static replica count:

```bash
kubectl get deployment orders-service -n team-alpha -o jsonpath='{.spec.replicas}'
# Should now show 3 (production environment default)
```

### Step 8: Clean Up

```bash
kubectl delete microservice payments-service -n team-alpha
kubectl delete microservice orders-service   -n team-alpha
```

---

## What You Built

- A `MicroService` XRD with nested `autoscaling` object and schema validation
- A Go template that conditionally creates an HPA and conditionally omits `replicas` from the Deployment when autoscaling is active
- Environment-driven replica defaults (`dev=1`, `staging=2`, `prod=3`) using template variables
- Nil-safe access to nested objects using Sprig's `default dict` pattern
- Verified live update propagation: changing the XR spec triggers Crossplane to update or delete the HPA

You now have the core skills to build a real internal developer platform API with Crossplane.

---

➡️ [Crossplane 10: Write a Composition Function in Go](10-write-function-in-go.md)
