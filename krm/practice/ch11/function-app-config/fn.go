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
