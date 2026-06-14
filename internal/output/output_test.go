package output

import (
	"encoding/json"
	"testing"

	verrors "github.com/nerveband/vflow/internal/errors"
)

func TestStructuredErrorShape(t *testing.T) {
	err := verrors.Validation("INVALID_ENUM", "unsupported provider", "Use one of: local, elevenlabs, soniox", false)
	got := ErrorEnvelope(err)

	raw, marshalErr := json.Marshal(got)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	errorObj := decoded["error"].(map[string]any)
	if errorObj["code"] != "INVALID_ENUM" {
		t.Fatalf("unexpected code: %#v", errorObj["code"])
	}
	if errorObj["hint"] != "Use one of: local, elevenlabs, soniox" {
		t.Fatalf("unexpected hint: %#v", errorObj["hint"])
	}
	if errorObj["retryable"] != false {
		t.Fatalf("unexpected retryable: %#v", errorObj["retryable"])
	}
	if errorObj["exit_code"] != float64(4) {
		t.Fatalf("unexpected exit_code: %#v", errorObj["exit_code"])
	}
}
