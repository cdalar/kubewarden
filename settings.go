package main

import (
	"encoding/json"
	"fmt"

	kubewarden "github.com/kubewarden/policy-sdk-go"
	kubewarden_protocol "github.com/kubewarden/policy-sdk-go/protocol"
)

// Settings holds the user-provided configuration of the policy.
//
// The policy currently needs no configuration: every container found inside
// the Pod (regular, init and ephemeral) has its CPU limit removed.
// An empty object (`{}`) or `null` are both valid settings.
type Settings struct{}

// NewSettingsFromValidationReq extracts the settings from a validation request.
func NewSettingsFromValidationReq(validationReq *kubewarden_protocol.ValidationRequest) (Settings, error) {
	settings := Settings{}
	if len(validationReq.Settings) == 0 {
		return settings, nil
	}
	if err := json.Unmarshal(validationReq.Settings, &settings); err != nil {
		return settings, fmt.Errorf("cannot parse settings: %w", err)
	}
	return settings, nil
}

// Valid reports whether the settings are acceptable.
func (s *Settings) Valid() (bool, error) {
	return true, nil
}

// validateSettings is the waPC entry point used by Kubewarden to validate the
// settings before the policy is deployed.
func validateSettings(payload []byte) ([]byte, error) {
	settings := Settings{}
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &settings); err != nil {
			return kubewarden.RejectSettings(kubewarden.Message(
				fmt.Sprintf("cannot parse settings: %v", err)))
		}
	}

	valid, err := settings.Valid()
	if err != nil {
		return kubewarden.RejectSettings(kubewarden.Message(
			fmt.Sprintf("invalid settings: %v", err)))
	}
	if !valid {
		return kubewarden.RejectSettings(kubewarden.Message("invalid settings"))
	}

	return kubewarden.AcceptSettings()
}
