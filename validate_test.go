package main

import (
	"encoding/json"
	"testing"

	kubewarden_protocol "github.com/kubewarden/policy-sdk-go/protocol"
	kubewarden_testing "github.com/kubewarden/policy-sdk-go/testing"
)

func runValidate(t *testing.T, payload []byte) kubewarden_protocol.ValidationResponse {
	t.Helper()
	responsePayload, err := validate(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	response := kubewarden_protocol.ValidationResponse{}
	if err := json.Unmarshal(responsePayload, &response); err != nil {
		t.Fatalf("cannot unmarshal response: %v", err)
	}
	return response
}

func containersOf(t *testing.T, mutated interface{}, field string) []interface{} {
	t.Helper()
	pod, ok := mutated.(map[string]interface{})
	if !ok {
		t.Fatalf("mutated object is not an object: %T", mutated)
	}
	spec, ok := pod["spec"].(map[string]interface{})
	if !ok {
		t.Fatalf("mutated object has no spec")
	}
	containers, ok := spec[field].([]interface{})
	if !ok {
		t.Fatalf("mutated object has no %s", field)
	}
	return containers
}

func TestValidateRemovesCPULimitFromAllContainerKinds(t *testing.T) {
	payload, err := kubewarden_testing.BuildValidationRequestFromFixture(
		"test_data/pod_with_cpu_limits.json", Settings{})
	if err != nil {
		t.Fatalf("cannot build request: %v", err)
	}

	response := runValidate(t, payload)
	if !response.Accepted {
		t.Fatalf("request should be accepted, got message %v", response.Message)
	}
	if response.MutatedObject == nil {
		t.Fatalf("request should be mutated")
	}

	// Regular container: cpu removed, memory kept, requests untouched.
	containers := containersOf(t, response.MutatedObject, "containers")
	first := containers[0].(map[string]interface{})
	resources := first["resources"].(map[string]interface{})
	limits := resources["limits"].(map[string]interface{})
	if _, found := limits["cpu"]; found {
		t.Errorf("cpu limit should have been removed from container %q", first["name"])
	}
	if limits["memory"] != "128Mi" {
		t.Errorf("memory limit should be kept, got %v", limits["memory"])
	}
	requests := resources["requests"].(map[string]interface{})
	if requests["cpu"] != "100m" {
		t.Errorf("cpu request should be kept, got %v", requests["cpu"])
	}

	// Second container only had a cpu limit: the now-empty `limits` map is dropped.
	second := containers[1].(map[string]interface{})
	secondResources := second["resources"].(map[string]interface{})
	if _, found := secondResources["limits"]; found {
		t.Errorf("empty limits should have been dropped, got %v", secondResources["limits"])
	}
	if _, found := secondResources["requests"]; !found {
		t.Errorf("requests should be kept on second container")
	}

	// Init container only had `resources.limits.cpu`: the whole `resources` is dropped.
	initContainers := containersOf(t, response.MutatedObject, "initContainers")
	init := initContainers[0].(map[string]interface{})
	if _, found := init["resources"]; found {
		t.Errorf("empty resources should have been dropped from init container, got %v", init["resources"])
	}

	// Ephemeral containers are handled too.
	ephemeral := containersOf(t, response.MutatedObject, "ephemeralContainers")[0].(map[string]interface{})
	ephemeralLimits := ephemeral["resources"].(map[string]interface{})["limits"].(map[string]interface{})
	if _, found := ephemeralLimits["cpu"]; found {
		t.Errorf("cpu limit should have been removed from ephemeral container")
	}

	// Unrelated fields survive the round-trip, including large integers.
	spec := response.MutatedObject.(map[string]interface{})["spec"].(map[string]interface{})
	if got := spec["terminationGracePeriodSeconds"]; got != json.Number("30") && got != float64(30) {
		t.Errorf("terminationGracePeriodSeconds changed: %v (%T)", got, got)
	}
	if spec["someUnknownFutureField"] != "kept" {
		t.Errorf("unknown fields must be preserved, got %v", spec["someUnknownFutureField"])
	}
}

func TestValidateAcceptsPodWithoutCPULimits(t *testing.T) {
	pod := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   map[string]interface{}{"name": "no-limits"},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "app",
					"image": "nginx",
					"resources": map[string]interface{}{
						"limits":   map[string]interface{}{"memory": "64Mi"},
						"requests": map[string]interface{}{"cpu": "50m"},
					},
				},
				map[string]interface{}{"name": "sidecar", "image": "busybox"},
			},
		},
	}
	payload, err := kubewarden_testing.BuildValidationRequest(pod, Settings{})
	if err != nil {
		t.Fatalf("cannot build request: %v", err)
	}

	response := runValidate(t, payload)
	if !response.Accepted {
		t.Fatalf("request should be accepted")
	}
	if response.MutatedObject != nil {
		t.Fatalf("request should not be mutated, got %v", response.MutatedObject)
	}
}

func TestValidateIgnoresNonPodKinds(t *testing.T) {
	request := kubewarden_protocol.ValidationRequest{
		Request: kubewarden_protocol.KubernetesAdmissionRequest{
			Kind:   kubewarden_protocol.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			Object: json.RawMessage(`{"spec":{"containers":[{"name":"x","resources":{"limits":{"cpu":"1"}}}]}}`),
		},
		Settings: json.RawMessage(`{}`),
	}
	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("cannot marshal request: %v", err)
	}

	response := runValidate(t, payload)
	if !response.Accepted || response.MutatedObject != nil {
		t.Fatalf("non-Pod kinds must be accepted without mutation")
	}
}

func TestValidateRejectsMalformedObject(t *testing.T) {
	request := kubewarden_protocol.ValidationRequest{
		Request: kubewarden_protocol.KubernetesAdmissionRequest{
			Kind:   kubewarden_protocol.GroupVersionKind{Version: "v1", Kind: "Pod"},
			Object: json.RawMessage(`"not an object"`),
		},
		Settings: json.RawMessage(`{}`),
	}
	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("cannot marshal request: %v", err)
	}

	response := runValidate(t, payload)
	if response.Accepted {
		t.Fatalf("malformed objects must be rejected")
	}
	if response.Code == nil || *response.Code != 400 {
		t.Fatalf("expected code 400, got %v", response.Code)
	}
}
