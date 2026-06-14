package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSchemaValidateReportsCoverage(t *testing.T) {
	out, errOut, code := runCLI(t, "schema", "--validate", "--format", "json")
	if code != 0 {
		t.Fatalf("expected success, got %d stderr=%s", code, errOut)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out)
	}
	data := got["data"].(map[string]any)
	if data["status"] != "valid" {
		t.Fatalf("expected valid status, got %#v", data["status"])
	}
	if !strings.Contains(out, "nle export") {
		t.Fatalf("schema output missing nle export:\n%s", out)
	}
	if !strings.Contains(out, "provenance.schema.json") {
		t.Fatalf("schema output missing provenance artifact schema:\n%s", out)
	}
}

func TestAgentContextMentionsLocalIndexArtifacts(t *testing.T) {
	out, errOut, code := runCLI(t, "agent-context", "--format", "json")
	if code != 0 {
		t.Fatalf("expected success, got %d stderr=%s", code, errOut)
	}
	for _, want := range []string{"reports/provenance.json", "~/.vflow/index.sqlite"} {
		if !strings.Contains(out, want) {
			t.Fatalf("agent context missing %q in:\n%s", want, out)
		}
	}
}
