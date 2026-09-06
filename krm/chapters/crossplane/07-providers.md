# Chapter 07: Providers & Managed Resources

> **You will build:** A `FeatureBranch` XRD backed by `provider-github` that creates a branch and seeds it with a config file from a single `kubectl apply`.

Up to now, every resource Crossplane has created has been a Kubernetes resource - Deployments, Services, ConfigMaps. This chapter introduces **Providers**, which extend Crossplane to manage **external** systems: GitHub, AWS, GCP, Helm releases, or anything with an API.

The GitHub provider is a great first provider: no cloud costs, and you can watch Crossplane make real changes to a GitHub repository directly from Kubernetes YAML.

---

## What is a Provider?

A Provider is a Crossplane package that ships two things:

1. **CRDs** - new `kind:` types for external resources (e.g., `Issue`, `Repository`)
2. **A controller** - a reconciler that watches those CRDs and calls the external API to make reality match spec

```
                     Provider package installed
                             │
              ┌──────────────┴──────────────┐
              │                             │
         CRDs installed              Controller running
    (Branch, RepositoryFile,      (watches for Branch CRs,
      Repository, ...)             calls GitHub REST API)
              │                             │
              ▼                             ▼
   You create:                    Controller creates/updates/deletes
   kind: Branch                   the branch on github.com
   spec.forProvider:
     repository: my-repo
     branch: feature/my-thing
```

The pattern is always the same regardless of the provider:

```
Provider install  →  ProviderConfig (auth)  →  Managed Resource (the actual thing)
```

---

## Managed Resources vs Compositions

| | Managed Resource | Composition |
|--|-----------------|-------------|
| **What it is** | A CRD for one external resource | A platform API that may render many resources |
| **Created by** | Provider package | You (Chapter 04) |
| **Renders** | One real external thing | Kubernetes resources OR Managed Resources |
| **Example** | `kind: Issue` → one GitHub issue | `kind: BugReport` → one `Issue` MR via template |

A Composition's Go template can render Managed Resource YAML just as easily as it renders Deployment YAML - the template produces the spec, the provider controller creates the real thing.

---

## Upbound Marketplace

Upbound hosts provider packages at https://marketplace.upbound.io. Browse to see what's available:

- [provider-github](https://marketplace.upbound.io/providers/coopnorge/provider-github) - Repositories, Branches, Files, Teams
- [provider-kubernetes](https://marketplace.upbound.io/providers/upbound/provider-kubernetes) - create K8s resources in other clusters
- [provider-helm](https://marketplace.upbound.io/providers/upbound/provider-helm/) - manage Helm releases as CRDs
- [provider-family-aws](https://marketplace.upbound.io/providers/upbound/provider-family-aws), [provider-family-gcp](https://marketplace.upbound.io/providers/upbound/provider-family-gcp/), [provider-family-azure](https://marketplace.upbound.io/providers/upbound/provider-family-azure/) - cloud infrastructure

Every provider page shows the package reference you paste into the `Provider` manifest.

---

## What `provider-github` Actually Manages

The coopnorge GitHub provider wraps the Terraform GitHub provider, so it covers _repository infrastructure_ - not workflow items like issues or pull request comments. Key managed resource types:

| Kind | API Group | What it manages |
|------|-----------|-----------------|
| `Repository` | `repo.github.upbound.io` | Create/configure repos (visibility, topics, description) |
| `Branch` | `repo.github.upbound.io` | Create branches from a source ref |
| `RepositoryFile` | `repo.github.upbound.io` | Commit files to a specific branch |
| `BranchProtection` | `repo.github.upbound.io` | Required reviews, status checks |
| `Team` | `team.github.upbound.io` | GitHub org teams |
| `TeamMembership` | `team.github.upbound.io` | Add/remove team members |

> **GitHub Issues are not available** in this provider - issues are a workflow tool, not repository infrastructure. If you were following an older version of this chapter that used `kind: Issue`, that's why it failed.

**Other things you could build with this provider:**

- **`ServiceRepo` XRD** - `Repository` + `BranchProtection` on `main` + seed a `CODEOWNERS` file, all from one YAML apply
- **Multi-branch fan-out** - one Composition that creates `staging` and `release` branches simultaneously, each with different config files
- **Config drift protection** - manage infrastructure config files across multiple repos from a single Composition; Crossplane re-commits if someone manually deletes or edits the file

---

## Hands-On: Branch and File Management

You will install `provider-github`, authenticate with a Personal Access Token, create a branch and a file as raw Managed Resources, then wrap both into a `FeatureBranch` XRD so any team can seed a new branch with a standard config file from a single YAML apply.

```bash
mkdir -p practice/ch07
```

### Step 1: Create a GitHub Personal Access Token

1. Go to **GitHub → Settings → Developer settings → Personal access tokens → Tokens (classic)**
2. Click **Generate new token (classic)**
3. Give it a descriptive name: `crossplane-learning`
4. Set expiry (7 days is fine for learning)
5. Select only the scopes you need:
   - `repo` - full access to repositories (required for creating branches and committing files)
   - If using a fine-grained token instead: grant **Contents: Read and write** on the target repository
6. Click **Generate token** and copy the value - you will not see it again

---

**Optional:** Set your token as an environment variable for convenience (recommended):

```bash
export GITHUB_TOKEN='ghp_YOUR_TOKEN_HERE'
```

This makes it easier to reference your token in scripts or when creating the Kubernetes Secret.


### Step 2: Store the Token in a Kubernetes Secret


The Upbound GitHub provider expects the token in a JSON object. You can use your environment variable to avoid pasting the token directly:

```bash
kubectl create secret generic github-credentials \
  --namespace crossplane-system \
  --from-literal=credentials="{\"token\":\"$GITHUB_TOKEN\"}"
```

Verify it was created (do NOT print the value):

```bash
kubectl get secret github-credentials -n crossplane-system
```

### Step 3: Install the GitHub Provider


Check the current version at https://marketplace.upbound.io/providers/coopnorge/provider-github/.

Create a file named `practice/ch07/provider-github.yaml` with the following contents:

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-github
spec:
  package: xpkg.upbound.io/coopnorge/provider-github:v0.13.0
```

> **Version note:** Replace `v0.13.0` with the latest version shown on the Upbound Marketplace. The installation process won't change.

Apply it and wait for healthy:

```bash
kubectl apply -f practice/ch07/provider-github.yaml
kubectl get providers.pkg.crossplane.io --watch
# Wait for HEALTHY=True, INSTALLED=True, then Ctrl+C
```

While it installs, check what CRDs it added:

```bash
kubectl get crds | grep github.upbound.io
```

You should see entries like `branches.repo.github.upbound.io`, `repositoryfiles.repo.github.upbound.io`, `repositories.repo.github.upbound.io`, and others. Anything ending in `.github.upbound.io` is managed by this provider.

### Step 4: Create the ProviderConfig

A `ProviderConfig` tells the provider controller which credentials to use. Create `practice/ch07/provider-config.yaml`:

```yaml
apiVersion: github.upbound.io/v1beta1
kind: ProviderConfig
metadata:
  name: default
spec:
  credentials:
    source: Secret
    secretRef:
      namespace: crossplane-system
      name: github-credentials
      key: credentials
```

The name `default` is special - any Managed Resource without an explicit `providerConfigRef` uses the config named `default`.

```bash
kubectl apply -f practice/ch07/provider-config.yaml
```

### Step 5: Create a Branch as a Managed Resource

Start with a raw Managed Resource - no XRD, no Composition. You are talking directly to the provider.

Create `practice/ch07/test-branch.yaml`. Replace `YOUR_REPO_NAME` with a real repository you own on GitHub:

```yaml
apiVersion: repo.github.upbound.io/v1alpha1
kind: Branch
metadata:
  name: crossplane-test-branch
spec:
  forProvider:
    repository: YOUR_REPO_NAME  # just the repo name, not the full URL
    branch: feature/crossplane-test
    sourceBranch: main
  providerConfigRef:
    name: default
```

Apply it:

```bash
kubectl apply -f practice/ch07/test-branch.yaml
```

Watch the Managed Resource reconcile:

```bash
kubectl get branches.repo.github.upbound.io crossplane-test-branch -w
# Watch READY and SYNCED columns. Ctrl+C when both are True.
```

Go to your repository on GitHub - `feature/crossplane-test` should now exist branching off `main`.

```bash
# See the full status written back by the provider
kubectl describe branch.repo.github.upbound.io crossplane-test-branch
# Look at Status.atProvider: etag, sourceSHA
```

The provider writes observed state back into the resource's status - the same observed → desired pattern as everything else in Crossplane.

---

### Step 6: Commit a File to That Branch

A branch is just a pointer. Now commit a file onto it. Create `practice/ch07/test-file.yaml`. Replace `YOUR_REPO_NAME`:

```yaml
apiVersion: repo.github.upbound.io/v1alpha1
kind: RepositoryFile
metadata:
  name: crossplane-test-config
spec:
  forProvider:
    repository: YOUR_REPO_NAME
    file: config/crossplane.yaml
    content: |
      # Managed by Crossplane
      environment: test
      managed_by: crossplane
    branch: feature/crossplane-test
    commitMessage: "chore: add config file via Crossplane"
    overwriteOnCreate: true
  providerConfigRef:
    name: default
```

Apply it:

```bash
kubectl apply -f practice/ch07/test-file.yaml
kubectl get repositoryfiles.repo.github.upbound.io crossplane-test-config -w
# Ctrl+C when READY=True and SYNCED=True
```

Go to your Github repository, switch to `feature/crossplane-test` - `config/crossplane.yaml` should be committed there.

> **Ordering note:** Crossplane creates both resources concurrently when they appear in a Composition. If the `RepositoryFile` controller tries to commit before the `Branch` is ready, it will fail and retry automatically. This is expected - SYNCED will be False briefly until the branch exists.

---

### Step 7: Update the File - Edit the Content

Edit `test-file.yaml` and change the `content` field, then re-apply:

```bash
kubectl apply -f practice/ch07/test-file.yaml
```

Or patch it directly:

```bash
kubectl patch repositoryfile.repo.github.upbound.io crossplane-test-config \
  --type merge \
  -p '{"spec":{"forProvider":{"content":"# Managed by Crossplane\nenvironment: test\nmanaged_by: crossplane\nversion: v2\n"}}}'
```

Crossplane reconciles the change and commits the updated file to GitHub. Refresh the file on GitHub - you should see a new commit and the updated content.

---

### Step 8: Delete

```bash
# Delete the file first, then the branch
kubectl delete -f practice/ch07/test-file.yaml
kubectl delete -f practice/ch07/test-branch.yaml
```

The controller removes the file from the branch, then deletes the branch on GitHub.

> **Order matters:** Delete the RepositoryFile before deleting the Branch. The provider needs the branch to exist to delete the file from it.

---

## Wrapping Two Managed Resources in a Composition

Direct Managed Resources are useful, but the real power is giving teams a clean XRD that hides the provider details. A `FeatureBranch` XRD lets any team create a branch seeded with a standard config file - hiding the `Branch` and `RepositoryFile` managed resources entirely.

### Step 9: Define the `FeatureBranch` XRD

Create `practice/ch07/feature-branch-xrd.yaml`:

```yaml
apiVersion: apiextensions.crossplane.io/v2
kind: CompositeResourceDefinition
metadata:
  name: featurebranches.platform.example.io
spec:
  group: platform.example.io
  names:
    kind: FeatureBranch
    plural: featurebranches
  scope: Cluster
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
            required: [repository, branchName]
            properties:
              repository:
                type: string
                description: GitHub repository name (not the full URL)
              branchName:
                type: string
                description: Name of the branch to create (e.g. feature/my-thing)
              sourceBranch:
                type: string
                default: main
                description: Branch to branch from
              configContent:
                type: string
                description: Content written to config/branch.yaml on the new branch
                default: "# Managed by Crossplane FeatureBranch\n"
          status:
            type: object
            properties:
              branchReady:
                type: boolean
              fileReady:
                type: boolean
```

Apply it and wait for the API to be established:

```bash
kubectl apply -f practice/ch07/feature-branch-xrd.yaml
kubectl get xrds --watch
# Wait for ESTABLISHED=True, then Ctrl+C
```

Once established, `FeatureBranch` is a real API type in your cluster:

```bash
kubectl api-resources | grep featurebranch
```

### Step 10: Create the Composition

The Composition renders two Managed Resources - a `Branch` and a `RepositoryFile` - from a single XR. Create `practice/ch07/feature-branch-composition.yaml`:

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: feature-branch-composition
spec:
  compositeTypeRef:
    apiVersion: platform.example.io/v1alpha1
    kind: FeatureBranch
  mode: Pipeline
  pipeline:
  - step: render-branch-and-file
    functionRef:
      name: function-go-templating
    input:
      apiVersion: gotemplating.fn.crossplane.io/v1beta1
      kind: GoTemplate
      source: Inline
      inline:
        template: |
          {{- $name    := .observed.composite.resource.metadata.name }}
          {{- $spec    := .observed.composite.resource.spec }}
          {{- $source  := $spec.sourceBranch | default "main" }}
          {{- $content := $spec.configContent | default "# Managed by Crossplane FeatureBranch\n" }}
          ---
          apiVersion: repo.github.upbound.io/v1alpha1
          kind: Branch
          metadata:
            name: {{ $name }}-branch
            annotations:
              gotemplating.fn.crossplane.io/composition-resource-name: feature-branch
          spec:
            forProvider:
              repository: {{ $spec.repository }}
              branch: {{ $spec.branchName }}
              sourceBranch: {{ $source }}
            providerConfigRef:
              name: default
          ---
          apiVersion: repo.github.upbound.io/v1alpha1
          kind: RepositoryFile
          metadata:
            name: {{ $name }}-config
            annotations:
              gotemplating.fn.crossplane.io/composition-resource-name: branch-config-file
          spec:
            forProvider:
              repository: {{ $spec.repository }}
              file: config/branch.yaml
              content: {{ $content | quote }}
              branch: {{ $spec.branchName }}
              commitMessage: "chore: seed config for {{ $spec.branchName }} via Crossplane"
              overwriteOnCreate: true
            providerConfigRef:
              name: default
```

Apply it:

```bash
kubectl apply -f practice/ch07/feature-branch-composition.yaml
kubectl get compositions
```

### Step 11: The Consumer-Facing API

A team that wants a new feature branch doesn't write `Branch` or `RepositoryFile` YAML - they write this. Create `practice/ch07/my-feature-branch.yaml`. Replace `YOUR_REPO_NAME`:

```yaml
apiVersion: platform.example.io/v1alpha1
kind: FeatureBranch
metadata:
  name: analytics-feature
spec:
  repository: YOUR_REPO_NAME
  branchName: feature/add-analytics
  sourceBranch: main
  configContent: |
    # Managed by Crossplane
    feature: analytics
    enabled: true
    owner: platform-team
```

With the XRD and Composition in place, this is the only YAML a consuming team ever needs - none of the `Branch` or `RepositoryFile` details visible to them.

We stop short of applying this because of a bug in Crossplane v2.2.0: cluster-scoped XRDs (required here because `Branch` and `RepositoryFile` are cluster-scoped managed resources) fail to add a finalizer to the composite resource, so the XR is created but never reconciles. The XRD and Composition themselves apply and validate correctly - the pattern is sound. This will work as written once the bug is fixed in a future patch release.

### Step 12: Clean Up

```bash
kubectl delete -f practice/ch07/feature-branch-composition.yaml
kubectl delete -f practice/ch07/feature-branch-xrd.yaml
kubectl delete -f practice/ch07/provider-github.yaml
kubectl delete secret github-credentials -n crossplane-system
```

---

## What You Built

- Installed an Upbound Provider (`provider-github`) and understood what a Provider is
- Created a `ProviderConfig` to authenticate with the GitHub API
- Created two raw Managed Resources - a `Branch` and a `RepositoryFile` - and watched Crossplane reconcile them against the real GitHub API
- Updated a managed file and saw Crossplane commit the diff to GitHub
- Defined a `FeatureBranch` XRD backed by a Composition - a clean platform API that hides provider details from consumers

**Extending this pattern:** The same approach works for any Upbound provider. Replace `provider-github` with `provider-helm` to manage Helm releases, `provider-kubernetes` to create resources in a remote cluster, or `provider-aws` when you are ready to provision real infrastructure. The Provider → ProviderConfig → Managed Resource pattern is always the same.

---

➡️ [Crossplane 08: Namespace Isolation & RBAC](08-claims-and-rbac.md)
