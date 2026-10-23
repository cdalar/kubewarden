package main

import (
	"bytes"
	"encoding/json"
	"fmt"

	kubewarden "github.com/kubewarden/policy-sdk-go"
	kubewarden_protocol "github.com/kubewarden/policy-sdk-go/protocol"
)

// containerListFields are the PodSpec fields that hold containers whose CPU
// limits must be dropped.
var containerListFields = []string{"containers", "initContainers", "ephemeralContainers"}

// validate is the waPC entry point used by Kubewarden to evaluate an
// admission request.
//
// The Pod is handled as a generic JSON document instead of a typed struct so
// that fields unknown to this policy (e.g. ones introduced by newer Kubernetes
// releases) survive the mutation untouched.
func validate(payload []byte) ([]byte, error) {
	validationRequest := kubewarden_protocol.ValidationRequest{}
	if err := json.Unmarshal(payload, &validationRequest); err != nil {
		return kubewarden.RejectRequest(
			kubewarden.Message(err.Error()),
			kubewarden.Code(400))
	}

	if _, err := NewSettingsFromValidationReq(&validationRequest); err != nil {
		return kubewarden.RejectRequest(
			kubewarden.Message(err.Error()),
			kubewarden.Code(400))
	}

	// Only Pods are mutated; any other kind is accepted as-is.
	if kind := validationRequest.Request.Kind.Kind; kind != "" && kind != "Pod" {
		return kubewarden.AcceptRequest()
	}

	pod := map[string]interface{}{}
	decoder := json.NewDecoder(bytes.NewReader(validationRequest.Request.Object))
	decoder.UseNumber() // keep integers such as terminationGracePeriodSeconds intact
	if err := decoder.Decode(&pod); err != nil {
		return kubewarden.RejectRequest(
			kubewarden.Message(fmt.Sprintf("cannot decode Pod: %v", err)),
			kubewarden.Code(400))
	}

	if !removeCPULimits(pod) {
		return kubewarden.AcceptRequest()
	}

	return kubewarden.MutateRequest(pod)
}

// removeCPULimits deletes `resources.limits.cpu` from every container of the
// Pod. Empty `limits` / `resources` objects left behind are removed too, so the
// resulting manifest stays tidy. It returns true when at least one change was
// made.
func removeCPULimits(pod map[string]interface{}) bool {
	spec, ok := pod["spec"].(map[string]interface{})
	if !ok {
		return false
	}

	mutated := false
	for _, field := range containerListFields {
		containers, ok := spec[field].([]interface{})
		if !ok {
			continue
		}
		for _, c := range containers {
			container, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			if removeContainerCPULimit(container) {
				mutated = true
			}
		}
	}

	return mutated
}

// removeContainerCPULimit strips the CPU limit from a single container and
// reports whether anything changed.
func removeContainerCPULimit(container map[string]interface{}) bool {
	resources, ok := container["resources"].(map[string]interface{})
	if !ok {
		return false
	}
	limits, ok := resources["limits"].(map[string]interface{})
	if !ok {
		return false
	}
	if _, ok := limits["cpu"]; !ok {
		return false
	}

	delete(limits, "cpu")
	if len(limits) == 0 {
		delete(resources, "limits")
	}
	if len(resources) == 0 {
		delete(container, "resources")
	}

	return true
}
