package main

import (
	"context"
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
