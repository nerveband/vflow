package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	artifactValidation := data["artifact_schema_validation"].(map[string]any)
	if artifactValidation["status"] != "valid" || artifactValidation["checked"].(float64) == 0 {
		t.Fatalf("expected artifact schemas to be validated, got %#v", artifactValidation)
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

func TestRenderReportSchemaDocumentsColorMetadata(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "schemas", "render-report.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("invalid render report schema json: %v", err)
	}
	properties := schema["properties"].(map[string]any)
	color := properties["color"].(map[string]any)
	colorProperties := color["properties"].(map[string]any)
	if got := colorProperties["lut_sha256"].(map[string]any)["pattern"]; got != "^[a-f0-9]{64}$" {
		t.Fatalf("lut_sha256 pattern = %#v", got)
	}
	intentEnum := colorProperties["intent"].(map[string]any)["enum"].([]any)
	if len(intentEnum) != 2 || intentEnum[0] != "preview" || intentEnum[1] != "final" {
		t.Fatalf("unexpected intent enum: %#v", intentEnum)
	}
	required := color["required"].([]any)
	for _, want := range []string{"ungraded_render_path", "graded_render_path", "lut_path", "lut_sha256", "ffmpeg_filtergraph", "warnings", "intent", "qa_report_refs"} {
		found := false
		for _, got := range required {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("color schema missing required field %q in %#v", want, required)
		}
	}
}
