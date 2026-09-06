# Learning KRM

Hands-on practice for building platform APIs on Kubernetes. Write YAML, apply to minikube, iterate. No CI/CD, no cloud account.

Start with Foundations, then pick Crossplane, kro, or both.

## The Three Layers

Every tool here is the same three things: a **schema** saying what fields exist, an **instance** declaring what you want, and a **controller** that makes reality match. Only the names change.

| | Raw Kubernetes | Crossplane | kro |
|---|---|---|---|
| Schema | CRD | XRD | RGD `schema` block |
| Logic | your Go reconcile loop | Composition | RGD `resources` block |
| Instance | CR | XR | CR of the generated kind |
| Running code | your controller binary | Crossplane, generic | kro, generic |
| Authored in | Go | YAML + Go functions | YAML + CEL |

Crossplane and kro both generate a real CRD and run the controller for you. The app team's experience is the same in all three cases: commit a small YAML file. The choice only decides who maintains the logic, and in what language.

Three traps:

- A **CRD** does nothing. It only makes the API server willing to store a shape.
- An **XRD** with no Composition validates beautifully and produces nothing. Shape and behaviour are separate files.
- A **kro RGD** infers dependency order from CEL references, so a resource nothing references may be created earlier than you assumed.

Depth in [Foundations 01](chapters/foundations/01-crds-crs-controllers.md).

## Foundations

| Chapter | Key Concepts | Est. Time |
|---------|--------------|-----------|
| [00 - YAML Primer](chapters/foundations/00-yaml-basics.md) | Objects, arrays, multi-line strings, reading compositions | ~20 min |
| [01 - CRDs, CRs & Controllers](chapters/foundations/01-crds-crs-controllers.md) | KRM three-layer model, reconciliation loop, spec/status contract | ~45 min |

## Crossplane

[Crossplane](https://crossplane.io) defines platform APIs and wires them to real resources through Compositions and Functions, with no Go controller for the common case.

| Chapter | Key Concepts | Est. Time |
|---------|--------------|-----------|
| [01 - Setup & The Local Workflow](chapters/crossplane/01-setup-and-big-picture.md) | minikube, Helm, Crossplane install, starter project tour | ~45 min |
| [02 - Kubernetes Resources Refresher](chapters/crossplane/02-kubernetes-refresher.md) | GVK, CRDs, Deployments, Services, labels | ~30 min |
| [03 - XRDs - Composite Resource Definitions](chapters/crossplane/03-xrds.md) | XRD schema, versions, spec vs status | ~45 min |
| [04 - Compositions & Go Templating](chapters/crossplane/04-compositions.md) | Pipeline mode, how Functions work (gRPC protocol), first Go template hands-on | ~45 min |
| [05 - Go Templating Deep Dive](chapters/crossplane/05-go-templating.md) | Sprig helpers, nil-safe `default dict`, status writeback, `define`/`include` blocks, conditional HPA | ~60 min |
| [06 - Composition Revisions](chapters/crossplane/06-composition-revisions.md) | CompositionRevision objects, Automatic vs Manual update policy | ~30 min |
| [07 - Providers & Managed Resources](chapters/crossplane/07-providers.md) | Upbound provider model, `provider-github`, ProviderConfig, direct MRs (`Branch` + `RepositoryFile`) | ~45 min |
| [08 - Namespace Isolation & RBAC](chapters/crossplane/08-claims-and-rbac.md) | Namespaced XRs, Roles, RoleBindings, `kubectl auth can-i` | ~30 min |
| [09 - Advanced Go Templating](chapters/crossplane/09-advanced-go-templating.md) | HPA conditionals, nil-safe patterns, loops, MicroService XRD | ~60 min |
| [10 - Write a Composition Function in Go](chapters/crossplane/10-write-function-in-go.md) | Custom Go function, RunFunction handler, local image load | ~90 min |
| [11 - Functions with HTTP](chapters/crossplane/11-functions-with-http.md) | Outbound HTTP calls from RunFunction, graceful degradation, httptest unit tests | ~45 min |

## kro

[kro](https://kro.run) (Kube Resource Orchestrator) is a Kubernetes SIG project. One `ResourceGraphDefinition` in YAML, and kro infers the dependency graph, generates a CRD, and runs the controller.

| Chapter | Key Concepts | Est. Time |
|---------|--------------|-----------|
| [01 - Intro, Setup & Your First RGD](chapters/kro/01-intro-and-setup.md) | ResourceGraphDefinition, SimpleSchema, CEL expressions, dependency ordering, kro vs Crossplane | ~45 min |

## When to Use Each

| Need | Reach for |
|-----------|-----------|
| Simple wiring, no Go | **kro** |
| Cloud resources via provider packages, or revision tracking and rollback | **Crossplane** |
| Logic that outgrows YAML and CEL | **Crossplane** + Go Function |
| State machines, admission webhooks, external HTTP calls | custom Go controller |

They are not mutually exclusive - both can run in the same cluster.

## Prerequisites

- **macOS** with [Homebrew](https://brew.sh)
- **Docker Desktop** running (minikube uses it as the driver)
- Basic Go familiarity and `kubectl` knowledge (`get`, `apply`, `describe`, `logs`)

Start with [Foundations 00 →](chapters/foundations/00-yaml-basics.md)
