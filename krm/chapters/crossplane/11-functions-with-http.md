# Chapter 11: Calling External APIs from a Composition Function

> **You will build:** A Go Composition Function that makes an outbound HTTP call during `RunFunction` - fetching team metadata from an internal service and embedding it in the composed resource.

This builds on Chapter 10's `function-app-config`. You will add an HTTP client to the existing function, write unit tests using `httptest.NewServer` (no cluster needed), and deploy a mock service to verify end-to-end.

---

## Why HTTP in a Function?

Go templates and YAML patches can only work with data already present in the XR - they cannot make network calls. A Go function has no such restriction. During `RunFunction`, you can call any network service: an internal metadata API, Vault, a CMDB, a cloud SDK endpoint. This is one of the key reasons teams choose Go functions over template-based approaches for complex platform logic.

---

## Prerequisites

Ensure the Chapter 10 function and XR are applied and healthy:

```bash
cd "$(git rev-parse --show-toplevel)"
kubectl apply -f practice/ch10/appconfig-xrd.yaml
kubectl apply -f practice/ch10/appconfig-composition.yaml
kubectl apply -f practice/ch10/function-install.yaml
kubectl get functions.pkg.crossplane.io --watch
# Wait for function-app-config HEALTHY=True, then Ctrl+C
```

---

## Step 1: Deploy a Mock Team Metadata Service

In a real platform you might call an internal API to fetch team ownership metadata: cost center, Slack channel, compliance tier. Deploy a mock using nginx:

```bash
mkdir -p practice/ch11
```

Create `practice/ch11/team-metadata-mock.yaml`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: team-metadata-nginx-config
  namespace: default
data:
  default.conf: |
    server {
      listen 80;
      location / {
        default_type application/json;
        return 200 '{"costCenter":"CC-1234","slackChannel":"#platform","tier":"gold"}';
      }
    }
---
apiVersion: v1
kind: Pod
metadata:
  name: team-metadata-mock
  namespace: default
  labels:
    app: team-metadata-mock
spec:
  containers:
  - name: nginx
    image: nginx:alpine
    volumeMounts:
    - name: config
      mountPath: /etc/nginx/conf.d/default.conf
      subPath: default.conf
  volumes:
  - name: config
    configMap:
      name: team-metadata-nginx-config
---
apiVersion: v1
kind: Service
metadata:
  name: team-metadata-service
  namespace: default
spec:
  selector:
    app: team-metadata-mock
  ports:
  - port: 80
    targetPort: 80
```

Apply and verify:

```bash
kubectl apply -f practice/ch11/team-metadata-mock.yaml
kubectl wait pod/team-metadata-mock --for=condition=Ready -n default --timeout=60s

# Verify the service returns JSON
kubectl run curl-test --image=curlimages/curl:latest --rm -it --restart=Never -- \
  curl -s http://team-metadata-service.default.svc.cluster.local/
# Expected: {"costCenter":"CC-1234","slackChannel":"#platform","tier":"gold"}
```

---

## Step 2: Copy the Function Directory

Start from a clean copy of the Chapter 10 function so ch10 stays untouched:

```bash
cd "$(git rev-parse --show-toplevel)"
cp -r practice/ch10/function-app-config practice/ch11/function-app-config
```

Now replace `practice/ch11/function-app-config/fn.go` with the version below, which adds the HTTP client:

```go
package main

import (
	"context"
	"encoding/json" // NEW: for decoding the HTTP response
	"fmt"
	"net/http" // NEW: for making HTTP calls
	"time"     // NEW: for the request timeout

	"github.com/crossplane/function-sdk-go/errors"
	"github.com/crossplane/function-sdk-go/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/resource/composed"
	"github.com/crossplane/function-sdk-go/response"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type Function struct {
	fnv1.UnimplementedFunctionRunnerServiceServer
	log logging.Logger
}

// NEW: TeamMetadata is the response shape from the team metadata service.
type TeamMetadata struct {
	CostCenter   string `json:"costCenter"`
	SlackChannel string `json:"slackChannel"`
	Tier         string `json:"tier"`
}

// NEW: fetchTeamMetadata calls the team metadata service for a given owner.
func fetchTeamMetadata(ctx context.Context, baseURL, owner string) (*TeamMetadata, error) {
	ctx5s, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/teams/%s", baseURL, owner)
	req, err := http.NewRequestWithContext(ctx5s, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("team metadata service returned HTTP %d", resp.StatusCode)
	}
	var meta TeamMetadata
	return &meta, json.NewDecoder(resp.Body).Decode(&meta)
}

func replicaHint(env string) int {
	switch env {
	case "production":
		return 3
	case "staging":
		return 2
	default:
		return 1
	}
}

// CHANGED: ctx replaces _ so the HTTP call can respect cancellation.
func (f *Function) RunFunction(ctx context.Context, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	f.log.Info("Running function", "tag", req.GetMeta().GetTag())

	rsp := response.To(req, response.DefaultTTL)

	oxr, err := request.GetObservedCompositeResource(req)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get observed composite resource")
	}

	appName, err := oxr.Resource.GetString("spec.appName")
	if err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot get spec.appName"))
		return rsp, nil
	}

	environment, err := oxr.Resource.GetString("spec.environment")
	if err != nil {
		environment = "production"
	}

	owner, err := oxr.Resource.GetString("spec.owner")
	if err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot get spec.owner"))
		return rsp, nil
	}

	namespace := oxr.Resource.GetNamespace()
	xrName := oxr.Resource.GetName()
	configMapName := fmt.Sprintf("%s-config", xrName)

	// NEW ── 2b. Fetch team metadata from the internal service ─────────────
	const metadataBaseURL = "http://team-metadata-service.default.svc.cluster.local"
	meta, err := fetchTeamMetadata(ctx, metadataBaseURL, owner)
	if err != nil {
		f.log.Info("Could not fetch team metadata, using defaults", "error", err)
		meta = &TeamMetadata{}
	}

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":        appName,
				"env":        environment,
				"owner":      owner,
				"managed-by": "crossplane",
			},
		},
		Data: map[string]string{
			"APP_NAME":      appName,
			"ENVIRONMENT":   environment,
			"OWNER":         owner,
			"APP_FULL_NAME": fmt.Sprintf("%s-%s", appName, environment),
			"REPLICA_HINT":  fmt.Sprintf("%d", replicaHint(environment)),
			"COST_CENTER":   meta.CostCenter,   // NEW: from HTTP call
			"SLACK_CHANNEL": meta.SlackChannel, // NEW: from HTTP call
			"TIER":          meta.Tier,         // NEW: from HTTP call
		},
	}

	cmObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm)
	if err != nil {
		return nil, errors.Wrap(err, "cannot convert ConfigMap to unstructured")
	}

	cd := composed.New()
	cd.Object = cmObj

	desired := map[resource.Name]*resource.DesiredComposed{
		"app-configmap": {Resource: cd},
	}

	if err := response.SetDesiredComposedResources(rsp, desired); err != nil {
		return nil, errors.Wrap(err, "cannot set desired resources")
	}

	f.log.Info("Desired ConfigMap set", "name", configMapName)
	return rsp, nil
}
```

Confirm it compiles:

```bash
cd practice/ch11/function-app-config
go build ./...
```

---

## Step 3: Unit Test the HTTP Call With httptest

`httptest.NewServer` starts a real local HTTP server in the test process - no cluster needed.

Replace `practice/ch11/function-app-config/fn_test.go` with:

```go
package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/crossplane/function-sdk-go/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/resource"
)

func TestRunFunction(t *testing.T) {
	f := &Function{log: logging.NewNopLogger()}

	req := &fnv1.RunFunctionRequest{
		Meta: &fnv1.RequestMeta{Tag: "test"},
		Observed: &fnv1.State{
			Composite: &fnv1.Resource{
				Resource: resource.MustStructJSON(`{
					"apiVersion": "platform.example.io/v1alpha1",
					"kind": "AppConfig",
					"metadata": {"name": "payments", "namespace": "team-alpha"},
					"spec": {
						"appName": "payments-service",
						"environment": "production",
						"owner": "team-alpha"
					}
				}`),
			},
		},
	}

	rsp, err := f.RunFunction(context.Background(), req)
	if err != nil {
		t.Fatalf("RunFunction error: %v", err)
	}
	if len(rsp.GetDesired().GetResources()) != 1 {
		t.Errorf("expected 1 desired resource, got %d", len(rsp.GetDesired().GetResources()))
	}
}

func TestFetchTeamMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"costCenter":"CC-4242","slackChannel":"#payments","tier":"gold"}`)
	}))
	defer srv.Close()

	meta, err := fetchTeamMetadata(context.Background(), srv.URL, "team-alpha")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.CostCenter != "CC-4242" {
		t.Errorf("expected CC-4242, got %s", meta.CostCenter)
	}
}
```

Run both tests:

```bash
go test ./... -v
```

---

## Step 4: Build, Push, and Verify End-to-End

```bash
cd practice/ch11/function-app-config
docker build --no-cache -t runtime .
crossplane xpkg build \
  --package-root=package \
  --embed-runtime-image=runtime \
  -o function-app-config.xpkg
IMAGE_ID=$(docker load -i function-app-config.xpkg | grep -oE 'sha256:[a-f0-9]+')
docker tag "$IMAGE_ID" ttl.sh/function-app-config:1h
docker push ttl.sh/function-app-config:1h
```

Force the function pod to restart with the new image:

```bash
cd "$(git rev-parse --show-toplevel)"
kubectl delete pod -n crossplane-system \
  -l pkg.crossplane.io/function=function-app-config
kubectl get pods -n crossplane-system --watch
# Wait for the new pod to become Running, then Ctrl+C
```

Re-apply the XR to trigger a fresh reconcile:

```bash
kubectl delete -f practice/ch10/test-appconfig.yaml
kubectl apply  -f practice/ch10/test-appconfig.yaml
kubectl get configmap payments-config -n default -o yaml
```

Expected `data`:

```yaml
data:
  APP_FULL_NAME: payments-service-production
  APP_NAME: payments-service
  COST_CENTER: CC-1234
  ENVIRONMENT: production
  OWNER: team-alpha
  REPLICA_HINT: "3"
  SLACK_CHANNEL: '#platform'
  TIER: gold
```

`COST_CENTER`, `SLACK_CHANNEL`, and `TIER` came from the HTTP call to `team-metadata-service` - logic no YAML template can express.

---

## Clean Up

```bash
kubectl delete -f practice/ch11/team-metadata-mock.yaml
```

---

## What You Built

- Outbound HTTP call from within `RunFunction` using `net/http` with a context-scoped timeout
- Graceful degradation when the metadata service is unavailable
- Unit testing of HTTP client code with `httptest.NewServer` - entirely in-process, no cluster required

---

## Where to Go Next

| Topic | What to Explore |
|-------|----------------|
| **Caching** | Cache the HTTP response in a struct field to avoid a network round-trip on every reconcile |
| **mTLS** | Use `tls.Config` in `http.Transport` to call services that require mutual TLS |
| **Vault** | Replace the mock with a real Vault `kv` read using the Vault HTTP API - same pattern |
| **Cloud SDKs** | The AWS, GCP, and Azure Go SDKs can be imported and called directly inside `RunFunction` |

---

➡️ [KRO](../kro/01-intro-and-setup.md)
