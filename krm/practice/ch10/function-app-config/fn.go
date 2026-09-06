package main

import (
	"context"
	"fmt"

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

// Function implements the Crossplane function gRPC server.
type Function struct {
	fnv1.UnimplementedFunctionRunnerServiceServer
	log logging.Logger
}

// replicaHint returns a sensible starting replica count for an environment.
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

// RunFunction is called by Crossplane on every reconcile of an AppConfig XR.
func (f *Function) RunFunction(_ context.Context, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	f.log.Info("Running function", "tag", req.GetMeta().GetTag())

	// Build the base response object.
	rsp := response.To(req, response.DefaultTTL)

	// ── 1. Read the observed XR ────────────────────────────────────────────
	oxr, err := request.GetObservedCompositeResource(req)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get observed composite resource")
	}

	// ── 2. Extract fields from spec ────────────────────────────────────────
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

	// ── 3. Build the ConfigMap ─────────────────────────────────────────────
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
			"APP_FULL_NAME": fmt.Sprintf("%s-%s", appName, environment), // computed
			"REPLICA_HINT":  fmt.Sprintf("%d", replicaHint(environment)),
		},
	}

	// ── 4. Convert to unstructured and register as a desired resource ──────
	cmObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm)
	if err != nil {
		return nil, errors.Wrap(err, "cannot convert ConfigMap to unstructured")
	}

	cd := composed.New()
	cd.Object = cmObj

	desired := map[resource.Name]*resource.DesiredComposed{
		"app-configmap": {
			Resource: cd,
		},
	}

	if err := response.SetDesiredComposedResources(rsp, desired); err != nil {
		return nil, errors.Wrap(err, "cannot set desired resources")
	}

	f.log.Info("Desired ConfigMap set", "name", configMapName)
	return rsp, nil
}
