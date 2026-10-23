package main

import (
	"bytes"
	"encoding/json"
	"fmt"

	kubewarden "github.com/kubewarden/policy-sdk-go"
	kubewarden_protocol "github.com/kubewarden/policy-sdk-go/protocol"
)

// codeBadRequest is the status code returned when the request cannot be parsed.
const codeBadRequest kubewarden.Code = 400

// validate is the waPC entry point used by Kubewarden to evaluate an
// admission request.
//
// The Pod is handled as a generic JSON document instead of a typed struct so
// that fields unknown to this policy (e.g. ones introduced by newer Kubernetes
// releases) survive the mutation untouched.
func validate(payload []byte) ([]byte, error) {
	validationRequest := kubewarden_protocol.ValidationRequest{}
	if err := json.Unmarshal(payload, &validationRequest); err != nil {
		return kubewarden.RejectRequest(kubewarden.Message(err.Error()), codeBadRequest)
	}

	if _, err := NewSettingsFromValidationReq(&validationRequest); err != nil {
		return kubewarden.RejectRequest(kubewarden.Message(err.Error()), codeBadRequest)
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
			codeBadRequest)
	}

	if !removeCPULimits(pod) {
		return kubewarden.AcceptRequest()
	}

	return kubewarden.MutateRequest(pod)
}

// removeCPULimits deletes `resources.limits.cpu` from every container of the
// Pod (regular, init and ephemeral). Empty `limits` / `resources` objects left
// behind are removed too, so the resulting manifest stays tidy. It returns
// true when at least one change was made.
func removeCPULimits(pod map[string]interface{}) bool {
	spec, isObject := pod["spec"].(map[string]interface{})
	if !isObject {
		return false
	}

	mutated := false
	for _, field := range []string{"containers", "initContainers", "ephemeralContainers"} {
		containers, isList := spec[field].([]interface{})
		if !isList {
			continue
		}
		for _, item := range containers {
			container, isContainer := item.(map[string]interface{})
			if !isContainer {
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
	resources, hasResources := container["resources"].(map[string]interface{})
	if !hasResources {
		return false
	}
	limits, hasLimits := resources["limits"].(map[string]interface{})
	if !hasLimits {
		return false
	}
	if _, hasCPU := limits["cpu"]; !hasCPU {
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
