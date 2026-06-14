package cli

import (
	"encoding/json"
	"testing"
)

func TestVersionEmitsJSONEnvelope(t *testing.T) {
	out, errOut, code := runCLI(t, "version", "--format", "json")
	if code != 0 {
		t.Fatalf("expected success, got %d stderr=%s", code, errOut)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("expected JSON output, got %q: %v", out, err)
	}
	if got["ok"] != true {
		t.Fatalf("expected ok=true, got %#v", got["ok"])
	}
	if got["schema_version"] != "vflow-response/v1" {
		t.Fatalf("unexpected schema_version: %#v", got["schema_version"])
	}
	if got["command"] != "version" {
		t.Fatalf("unexpected command: %#v", got["command"])
	}
}
