# Chapter 06: Composition Revisions

> **You will build:** Pin an XR to a specific Composition version and test a safe rollout.

Every time you update a Composition, Crossplane creates a **CompositionRevision** - an immutable snapshot of that Composition at a point in time. This gives you a safe rollout mechanism: new XR instances pick up the latest revision automatically, but existing XRs can be pinned to an older revision until you decide to migrate them.

---

## Why Revisions Exist

Imagine you have 50 teams each with a `WebService` XR. You update the Composition to change a default. Without revisions, all 50 XRs would re-reconcile immediately with the new Composition - potentially changing running workloads without team consent. With revisions you can:

- Publish a new Composition version
- Let new deployments automatically use it
- Let existing deployments keep running the old version
- Migrate teams one at a time by updating their `compositionRevisionRef`

---

## The Revision Model

```
  Composition "webservice-composition"
  (mutable - you edit this)
          │
          │ Crossplane creates an immutable snapshot on each change
          ▼
  CompositionRevision "webservice-composition-abc12"   ← revision 1 (oldest)
  CompositionRevision "webservice-composition-def34"   ← revision 2
  CompositionRevision "webservice-composition-ghi56"   ← revision 3 (latest)

  XR "my-api" ──── compositionRevisionRef: ghi56  (automatic: always latest)
  XR "legacy" ──── compositionRevisionRef: abc12  (manual: pinned to revision 1)
```

CompositionRevisions are **read-only**. You never edit them. You edit the Composition and Crossplane creates a new revision.

---

## XR Composition Update Policy

Each XR controls how it handles new revisions via `spec.crossplane.compositionUpdatePolicy`:

| Value | Behavior |
|-------|---------|
| `Automatic` | (default) XR always tracks the latest CompositionRevision. On every reconcile it uses the most recent revision. |
| `Manual` | XR stays on its current `compositionRevisionRef` until you explicitly update that field. |

Setting it on an individual XR:

```yaml
apiVersion: platform.example.io/v1alpha1
kind: WebService
metadata:
  name: my-stable-api
  namespace: default
spec:
  crossplane:
    compositionUpdatePolicy: Manual          # Pin to current revision
    compositionRevisionRef:
      name: webservice-composition-abc12     # The specific revision name
  image: nginx:alpine
  replicas: 2
```

Setting `Manual` without specifying `compositionRevisionRef` means the XR pins to whatever revision it reconciled against most recently.

---

## Selecting a Composition by Labels

Instead of naming a Composition directly with `spec.crossplane.compositionRef`, XRs can select one dynamically using labels:

```yaml
spec:
  crossplane:
    compositionSelector:
      matchLabels:
        channel: stable        # Only use Compositions labelled channel=stable
```

You then label your Compositions:

```yaml
metadata:
  name: webservice-composition-v2
  labels:
    channel: stable
```

This pattern lets you run `channel: stable` and `channel: canary` Compositions side by side and migrate XRs by changing the label selector rather than the `spec.crossplane.compositionRef` name.

---

## Versioning XRDs, Compositions, and XRs

Every time you `kubectl apply` a Composition - even a one-character whitespace change - Crossplane creates a new immutable **CompositionRevision** snapshot. This is automatic and always happens. The `compositionUpdatePolicy` on each XR then controls what happens next:

- **`Automatic`** (default): the XR picks up the new revision on its next reconcile loop with no action from you. This is fine for most routine Composition edits - tweaking templates, adjusting labels, changing defaults.
- **`Manual`**: the XR stays pinned to its current revision until you explicitly update `compositionRevisionRef`. Use this when a Composition change is significant enough that you want to migrate XRs deliberately, one at a time.

CompositionRevisions cover everything within a single Composition object. But when the *XRD schema itself* needs a breaking change - renaming a field, removing one, or adding a required field - Revisions aren't enough. That requires introducing a new XRD version, which is a different mechanism described below.

### The Versioning Scheme

Kubernetes and Crossplane use **stability-stage versioning** rather than semver. The version string encodes how mature and stable the API is:

| Version | Stability | Meaning |
|---------|-----------|---------|
| `v1alpha1`, `v1alpha2` | Alpha | Experimental - may change or be removed without notice |
| `v1beta1`, `v1beta2` | Beta | Mostly stable - breaking changes unlikely but possible |
| `v1`, `v2` | GA (stable) | Stable - breaking changes require a new major version |

The number suffix (`alpha1` → `alpha2`) increments when the schema changes within the same stage. When the API is mature enough to graduate, the stage name changes (`v1alpha2` → `v1beta1`).

A typical lifecycle looks like:

```
v1alpha1 → v1alpha2 → v1beta1 → v1beta2 → v1
                                            ↓ breaking change later
                                           v2alpha1 → ...
```

Unlike semver, multiple versions can coexist in the same XRD simultaneously (via `spec.versions`), letting teams on the old version and teams on the new version run side by side during a migration - no hard cutover required.

---

### How the Pieces Connect

Before looking at when and how to version, it helps to understand how XRDs, Compositions, and XRs relate to each other.

**XRD → XR (the schema bridge)**

The XRD is the schema definition - it registers a new custom resource Kind (`WebService`) with the Kubernetes API server and declares what fields it accepts. An XR is simply an *instance* of that Kind. The XRD version (`v1alpha1`, `v1beta1`) is what appears in the XR's `apiVersion` field:

```
XRD defines:  platform.example.io/v1alpha1  Kind: WebService
                          ↓
XR uses:      apiVersion: platform.example.io/v1alpha1
              kind: WebService
              spec:
                image: nginx:alpine    ← fields validated against the XRD schema
                replicas: 2
```

When you add a new version to the XRD, you're telling the API server: "this Kind now also exists at `v1beta1` with this different schema." Existing XRs at `v1alpha1` are unaffected until you explicitly update their `apiVersion`.

**XRD → Composition (the reconciliation bridge)**

A Composition is the implementation of an XRD version. It declares which XRD version it handles via `compositeTypeRef`, defined in the **Composition** file:

```yaml
# webservice-composition.yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: webservice-composition
spec:
  compositeTypeRef:
    apiVersion: platform.example.io/v1alpha1   # ← must match an XRD version
    kind: WebService
```

This is the binding between the two: when Crossplane reconciles an XR at `v1alpha1`, it looks for a Composition whose `compositeTypeRef` matches `platform.example.io/v1alpha1 / WebService`. The Composition is *not* versioned by its name - it is versioned by which XRD version its `compositeTypeRef` points to.

```
┌─────────────────────────────────────────────────────┐
│ XRD                                                 │
│   group: platform.example.io                        │
│   names.kind: WebService                            │
│   spec.versions:                                    │
│     - name: v1alpha1                                │
└─────────────────────────────────────────────────────┘
                         ▲
          Composition targets this XRD by matching
          both group+version and kind (see below)
                         │
┌─────────────────────────────────────────────────────┐
│ Composition                                         │
│   metadata.name: webservice-composition             │
│                                                     │
│   spec.compositeTypeRef:                            │
│     apiVersion: platform.example.io/v1alpha1        │
│     kind: WebService                                │
└─────────────────────────────────────────────────────┘
                         │
                         │ reconciles
                         ▼
┌─────────────────────────────────────────────────────┐
│ XR                                                  │
│   apiVersion: platform.example.io/v1alpha1     ─────│──► matches compositeTypeRef.apiVersion
│   kind: WebService                             ─────│──► matches compositeTypeRef.kind
│   spec:                                             │
│     crossplane:                                     │
│       compositionRef:                               │
│         name: webservice-composition           ─────│──► matches Compositions metadata.name
└─────────────────────────────────────────────────────┘
```

When you introduce `v1beta1` in the XRD, you need a separate Composition with `compositeTypeRef.apiVersion: platform.example.io/v1beta1` to handle those XRs:

```
┌──────────────────────────────────┐   ┌──────────────────────────────────┐
│ XRD                              │   │ XRD                              │
│   spec.versions:                 │   │   spec.versions:                 │
│     - name: v1alpha1             │   │     - name: v1alpha1             │
│     - name: v1beta1              │   │     - name: v1beta1              │
└──────────────────────────────────┘   └──────────────────────────────────┘
               ▲                                      ▲
    platform.example.io/v1alpha1           platform.example.io/v1beta1
               │                                      │
┌──────────────────────────────────┐   ┌──────────────────────────────────┐
│ Composition  webservice-v1       │   │ Composition  webservice-v2       │
│   compositeTypeRef:              │   │   compositeTypeRef:              │
│     apiVersion: .../v1alpha1     │   │     apiVersion: .../v1beta1      │
└──────────────────────────────────┘   └──────────────────────────────────┘
               │                                      │
               │ reconciles                           │ reconciles
               ▼                                      ▼
┌──────────────────────────────────┐   ┌──────────────────────────────────┐
│ XR  apiVersion: .../v1alpha1     │   │ XR  apiVersion: .../v1beta1      │
└──────────────────────────────────┘   └──────────────────────────────────┘
```

**Summary:** The XRD version is the shared contract - it appears in both the XR (`apiVersion`) and the Composition (`compositeTypeRef`).

---

### When to Version

| Change Type | Situation | Action |
|-------------|-----------|--------|
| XRD schema | Adding an optional field with a safe default | No version bump needed - existing XRs continue to work |
| XRD schema | Renaming a field or changing its type | **New XRD version** - the old field is a breaking change |
| XRD schema | Removing a field consumers depend on | **New XRD version** with a deprecation period |
| Composition | Changing Composition logic without touching the XRD schema | No version bump needed - let Composition Revisions handle it |
| Composition | Major implementation restructure (e.g. swapping cloud resources) | Create a new file (e.g. `webservice-composition-v2.yaml`) with a new `metadata.name` but the **same** `compositeTypeRef`, apply it to the cluster, then update each XR's `compositionRef` to point to the new name |

**Rule of thumb:** version the XRD when the *schema* (the fields users write in their YAML) changes in a breaking way. Create a new named Composition when the *implementation* changes significantly but the schema stays the same.

---

### XRD Version Flags

Each version entry in `spec.versions` has two important flags:

| Flag | Meaning |
|------|---------|
| `served: true` | The Kubernetes API server accepts and returns XR objects at this version. Set to `false` to stop accepting new objects at that version. |
| `referenceable: true` | Crossplane uses this version as the target for Composition reconciliation - it is the version a Composition's `compositeTypeRef` must point to for active reconciling. **Exactly one version may be `referenceable: true`** at a time. Setting `referenceable: false` on a new version means you can apply it to the cluster safely; Crossplane will not try to reconcile XRs against it until you flip this to `true`. |

---

### Breaking Change Migration Workflows

#### Scenario A: Breaking XRD schema change (field renamed, removed, or required)

_Example: renaming `image` to `containerImage`, or adding a new required field `owner`._

This scenario involves changing the XRD schema, so a new XRD version is required. CompositionRevisions alone cannot help here - they snapshot the Composition implementation, not the schema contract.

```
1. Add v1beta1 to the XRD with your new schema (served=true, referenceable=false)
   → kubectl apply -f xrd.yaml
   ↳ Safe: v1beta1 is known to the API server but Crossplane won't reconcile
     against it yet. All existing v1alpha1 XRs keep reconciling normally.

2. Create a new Composition file (e.g. webservice-composition-v2.yaml) with a
   new metadata.name and compositeTypeRef pointing to v1beta1
   → kubectl apply -f webservice-composition-v2.yaml
   ↳ No XRs use it yet. Crossplane creates a CompositionRevision for it automatically.
   ↳ The new metadata.name is what separates it from the old Composition - both
     exist in the cluster simultaneously until you finish migrating.

3. Test: create one new XR with apiVersion: v1beta1, pointing to the new Composition
   → kubectl apply -f test-xr-v1beta1.yaml
   ↳ Verify it reconciles correctly before touching any existing XRs.

4. Flip referenceable: true on v1beta1 and false on v1alpha1 in the XRD
   → kubectl apply -f xrd.yaml
   ↳ Crossplane now routes reconciliation through v1beta1. The Composition from
     step 2 must already exist - otherwise XRs will sit unreconciled.

5. Migrate existing XRs one by one: update their apiVersion to v1beta1 and add
   any new required fields, then kubectl apply each one.
   ↳ Policy note: compositionUpdatePolicy does not help here. It controls which
     CompositionRevision an XR uses, not which XRD version. You must manually
     update the apiVersion field in each XR manifest.

6. Once all XRs are on v1beta1, set v1alpha1 served=false → apply XRD.
   ↳ Rejects any new objects at v1alpha1. Existing migrated XRs are unaffected.

7. After a deprecation period, remove v1alpha1 from spec.versions → apply XRD.
```

#### Scenario B: Breaking Composition change (no XRD schema change)

_Example: replacing a Deployment with a StatefulSet, or restructuring the resource pipeline so significantly that you can't safely roll it out to all XRs at once._

Because the XRD schema isn't changing, **no new XRD version is needed**. You create a new named Composition targeting the same XRD version, then migrate XRs to it one by one.

This is where `compositionUpdatePolicy` matters most:
- XRs with **`Automatic`** policy will immediately pick up any new revision of their current Composition on the next reconcile - so if you edit the existing Composition in place, all Automatic XRs will get the change at once. If that's too risky, create a new named Composition instead (this scenario).
- XRs with **`Manual`** policy stay pinned to their current `compositionRevisionRef` regardless of what you change in the Composition. You migrate them explicitly by updating `compositionRef` or `compositionRevisionRef`.

```
1. Create a new Composition (e.g. webservice-composition-v2) targeting the
   existing XRD version (compositeTypeRef: v1alpha1)
   → kubectl apply -f webservice-composition-v2.yaml
   ↳ No XRs use it yet. Crossplane creates a CompositionRevision for it automatically.

2. Test: create one new XR pointing to it explicitly
   spec:
     crossplane:
       compositionRef:
         name: webservice-composition-v2
   → kubectl apply -f test-xr.yaml

3. Migrate existing XRs one by one by patching their compositionRef:
   kubectl patch webservice <name> -n <ns> --type merge \
     -p '{"spec":{"crossplane":{"compositionRef":{"name":"webservice-composition-v2"}}}}'
   ↳ Once patched, Automatic XRs will start reconciling against the new Composition
     immediately. Manual XRs will also switch Compositions but stay on their pinned
     revision of the new one until you update compositionRevisionRef.

4. Once all XRs are migrated, delete the old Composition.
   kubectl delete composition webservice-composition
```

---

## Hands-On: Create and Inspect Revisions

```bash
mkdir -p practice/ch06
```

### Step 1: Apply the WebService XRD and a Starting Composition

If Chapter 03 and 04 resources are gone:

```bash
kubectl apply -f practice/ch03/webservice-xrd.yaml
kubectl apply -f practice/ch04/function-go-templating.yaml
kubectl get xrds --watch
# Wait for ESTABLISHED=True, then Ctrl+C
kubectl get functions.pkg.crossplane.io --watch
# Wait for HEALTHY=True, then Ctrl+C
```

This chapter relies on `webservice-composition` starting at **Revision 1**. If you've worked through earlier chapters, previous applies will have already incremented the revision counter. Delete the Composition (and its revisions) first so you get a clean slate:

```bash
kubectl delete composition webservice-composition --ignore-not-found
kubectl delete composition webservice-advanced-composition --ignore-not-found
```

This chapter edits the Composition to create a second revision. Copy it into `practice/ch06/` first so the ch04 file stays untouched:

```bash
cp practice/ch04/webservice-composition.yaml practice/ch06/webservice-composition.yaml
kubectl apply -f practice/ch06/webservice-composition.yaml
```

### Step 2: Inspect the First Revision

```bash
kubectl get compositionrevisions
```

Expected output:

```
NAME                                REVISION   XR-KIND      XR-APIVERSION                    AGE
webservice-composition-<hash>       1          WebService   platform.example.io/v1alpha1     10s
```

Get the full details:

```bash
kubectl describe compositionrevision -l crossplane.io/composition-name=webservice-composition
```

Look for:
- `Revision: 1` in the Spec section

### Step 3: Create a WebService with Manual Update Policy

Create `practice/ch06/pinned-webservice.yaml`:

```yaml
apiVersion: platform.example.io/v1alpha1
kind: WebService
metadata:
  name: pinned-svc
  namespace: default
spec:
  crossplane:
    compositionUpdatePolicy: Manual
  image: nginx:alpine
  replicas: 1
  port: 80
  environment: production
```

Apply it:

```bash
kubectl apply -f practice/ch06/pinned-webservice.yaml
```

Check what revision it was pinned to automatically:

```bash
kubectl get webservice pinned-svc -n default -o jsonpath='{.spec.crossplane.compositionRevisionRef.name}'
```

Crossplane fills in the `compositionRevisionRef` with the current latest revision at the time the XR was first reconciled. Because the policy is `Manual`, it will now stay on that revision even when you publish a new one.

### Step 4: Create a Second WebService with Automatic Policy (the Default)

Create `practice/ch06/auto-webservice.yaml`:

```yaml
apiVersion: platform.example.io/v1alpha1
kind: WebService
metadata:
  name: auto-svc
  namespace: default
spec:
  crossplane:
    compositionUpdatePolicy: Automatic
  image: nginx:alpine
  replicas: 1
  port: 80
  environment: staging
```

Apply it:

```bash
kubectl apply -f practice/ch06/auto-webservice.yaml
```

### Step 5: Update the Composition - Create Revision 2

Edit `practice/ch06/webservice-composition.yaml` (the copy you made in Step 1 - not the ch04 original). Find the Deployment's `metadata.labels` block and add a label:

```yaml
          metadata:
            name: {{ $name }}
            namespace: {{ $ns }}
            annotations:
              gotemplating.fn.crossplane.io/composition-resource-name: deployment
            labels:
              app: {{ $name }}
              environment: {{ $spec.environment | default "production" }}
              revision: "2"           # ← add this line
```

Apply the updated Composition:

```bash
kubectl apply -f practice/ch06/webservice-composition.yaml
```

### Step 6: Inspect the Two Revisions

```bash
kubectl get compositionrevisions --sort-by=.spec.revision
```

Expected:

```
NAME                                REVISION   XR-KIND      XR-APIVERSION                    AGE
webservice-composition-<hash1>      1          WebService   platform.example.io/v1alpha1     5m
webservice-composition-<hash2>      2          WebService   platform.example.io/v1alpha1     10s
```

Revision 2 is the latest - new XRs with `Automatic` policy will use it.

### Step 7: Verify the Manual XR Did NOT Update

```bash
kubectl get webservice pinned-svc -n default -o jsonpath='{.spec.crossplane.compositionRevisionRef.name}'
# Should still point to the older revision hash
```

Check the Deployment created from `pinned-svc` - it should NOT have the `revision: "2"` label:

```bash
kubectl get deployment pinned-svc -n default -o jsonpath='{.metadata.labels}'
# Should not contain revision:2
```

### Step 8: Verify the Automatic XR DID Update

```bash
kubectl get webservice auto-svc -n default -o jsonpath='{.spec.crossplane.compositionRevisionRef.name}'
# Should point to the latest revision hash
```

Check its Deployment for the new label:

```bash
kubectl get deployment auto-svc -n default -o jsonpath='{.metadata.labels}'
# Should contain revision:2
```

### Step 9: Manually Migrate the Pinned XR to Revision 2

Get the name of revision 2:

```bash
REV2=$(kubectl get compositionrevisions \
  --sort-by=.spec.revision \
  -o jsonpath='{.items[-1].metadata.name}')
echo $REV2
```

Patch the pinned XR to use revision 2:

```bash
kubectl patch webservice pinned-svc -n default \
  --type merge \
  -p "{\"spec\":{\"crossplane\":{\"compositionRevisionRef\":{\"name\":\"$REV2\"}}}}"
```

Watch it reconcile:

```bash
kubectl get events -n default --sort-by=.metadata.creationTimestamp | tail -10
```

Verify the Deployment now has the revision 2 label:

```bash
kubectl get deployment pinned-svc -n default -o jsonpath='{.metadata.labels}'
# Should now contain revision:2
```

### Step 10: Clean Up

```bash
# XRs (deletes all composed resources via cascade)
kubectl delete -f practice/ch06/pinned-webservice.yaml --ignore-not-found
kubectl delete -f practice/ch06/auto-webservice.yaml --ignore-not-found

# Composition applied in this chapter (also deletes its revisions)
kubectl delete composition webservice-composition --ignore-not-found
```

---

## What You Built

- Created two WebService XRs with different update policies: `Manual` and `Automatic`
- Updated the Composition and watched Crossplane auto-create Revision 2
- Verified the `Manual` XR stayed on Revision 1 while the `Automatic` XR upgraded immediately
- Manually migrated the pinned XR to Revision 2 by updating `compositionRevisionRef`

Composition Revisions are the safety net for platform changes at scale. The next chapter covers namespace isolation - restricting which teams can create XRs in which namespaces.

---

➡️ [Crossplane 07: Providers & Managed Resources](07-providers.md)
