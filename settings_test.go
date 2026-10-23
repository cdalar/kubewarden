package main

import (
	"encoding/json"
	"testing"

	kubewarden_protocol "github.com/kubewarden/policy-sdk-go/protocol"
)

func runValidateSettings(t *testing.T, payload string) kubewarden_protocol.SettingsValidationResponse {
	t.Helper()
	responsePayload, err := validateSettings([]byte(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	response := kubewarden_protocol.SettingsValidationResponse{}
	if err = json.Unmarshal(responsePayload, &response); err != nil {
		t.Fatalf("cannot unmarshal response: %v", err)
	}
	return response
}

func TestValidateSettingsAcceptsEmptyAndNull(t *testing.T) {
	for _, payload := range []string{"", "{}", "null"} {
		if response := runValidateSettings(t, payload); !response.Valid {
			t.Errorf("settings %q should be valid, got %v", payload, response.Message)
		}
	}
}

func TestValidateSettingsRejectsGarbage(t *testing.T) {
	if response := runValidateSettings(t, `{"not": closed`); response.Valid {
		t.Errorf("malformed settings should be rejected")
	}
}
