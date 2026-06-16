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
	if !strings.Contains(out, "nle-sidecar.schema.json") {
		t.Fatalf("schema output missing NLE sidecar artifact schema:\n%s", out)
	}
	if !strings.Contains(out, "provider-bakeoff.schema.json") {
		t.Fatalf("schema output missing provider bakeoff artifact schema:\n%s", out)
	}
	if !strings.Contains(out, "audit-report.schema.json") {
		t.Fatalf("schema output missing audit report artifact schema:\n%s", out)
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

func TestColorGradeReportSchemaDocumentsReviewContract(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "schemas", "color-grade-report.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("invalid color grade report schema json: %v", err)
	}
	properties := schema["properties"].(map[string]any)
	if got := properties["version"].(map[string]any)["const"]; got != "vflow-color-grade-report/v1" {
		t.Fatalf("version const = %#v", got)
	}
	statusEnum := properties["status"].(map[string]any)["enum"].([]any)
	for _, want := range []string{"planned", "written", "analyzed"} {
		found := false
		for _, got := range statusEnum {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("status enum missing %q in %#v", want, statusEnum)
		}
	}
	providerEnum := properties["provider"].(map[string]any)["enum"].([]any)
	if len(providerEnum) != 1 || providerEnum[0] != "gemini" {
		t.Fatalf("unexpected provider enum: %#v", providerEnum)
	}
	observations := properties["observations"].(map[string]any)
	itemProperties := observations["items"].(map[string]any)["properties"].(map[string]any)
	if got := itemProperties["confidence"].(map[string]any)["maximum"]; got != float64(1) {
		t.Fatalf("observation confidence maximum = %#v", got)
	}
}

func TestTranscriptSchemaDocumentsCanonicalWordsContract(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "schemas", "transcript.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("invalid transcript schema json: %v", err)
	}
	properties := schema["properties"].(map[string]any)
	if got := properties["version"].(map[string]any)["const"]; got != "vflow-words/v1" {
		t.Fatalf("version const = %#v", got)
	}
	words := properties["words"].(map[string]any)
	wordProperties := words["items"].(map[string]any)["properties"].(map[string]any)
	if got := wordProperties["start_frame"].(map[string]any)["minimum"]; got != float64(0) {
		t.Fatalf("start_frame minimum = %#v", got)
	}
	if got := wordProperties["confidence"].(map[string]any)["maximum"]; got != float64(1) {
		t.Fatalf("confidence maximum = %#v", got)
	}
	required := words["items"].(map[string]any)["required"].([]any)
	for _, want := range []string{"id", "text", "start_frame", "end_frame", "provider"} {
		found := false
		for _, got := range required {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("word schema missing required field %q in %#v", want, required)
		}
	}
}

func TestContentEDLAndTimeMapSchemasDocumentFrameContracts(t *testing.T) {
	contentRaw, err := os.ReadFile(filepath.Join("..", "..", "schemas", "content-edl.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var contentSchema map[string]any
	if err := json.Unmarshal(contentRaw, &contentSchema); err != nil {
		t.Fatalf("invalid content EDL schema json: %v", err)
	}
	contentProperties := contentSchema["properties"].(map[string]any)
	if got := contentProperties["version"].(map[string]any)["const"]; got != "vflow-content-edl/v1" {
		t.Fatalf("content EDL version const = %#v", got)
	}
	deleteItems := contentProperties["delete_segments"].(map[string]any)["items"].(map[string]any)
	deleteProperties := deleteItems["properties"].(map[string]any)
	if got := deleteProperties["start_frame"].(map[string]any)["minimum"]; got != float64(0) {
		t.Fatalf("delete start_frame minimum = %#v", got)
	}
	if got := deleteProperties["confidence"].(map[string]any)["maximum"]; got != float64(1) {
		t.Fatalf("delete confidence maximum = %#v", got)
	}

	timeMapRaw, err := os.ReadFile(filepath.Join("..", "..", "schemas", "time-map.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var timeMapSchema map[string]any
	if err := json.Unmarshal(timeMapRaw, &timeMapSchema); err != nil {
		t.Fatalf("invalid time map schema json: %v", err)
	}
	timeMapProperties := timeMapSchema["properties"].(map[string]any)
	if got := timeMapProperties["duration_frames"].(map[string]any)["minimum"]; got != float64(0) {
		t.Fatalf("duration_frames minimum = %#v", got)
	}
	deleteRangeRequired := timeMapProperties["deletes"].(map[string]any)["items"].(map[string]any)["required"].([]any)
	for _, want := range []string{"start_frame", "end_frame"} {
		found := false
		for _, got := range deleteRangeRequired {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("time map delete range missing required field %q in %#v", want, deleteRangeRequired)
		}
	}
}

func TestSourceMediaReviewSchemaDocumentsProbeContract(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "schemas", "source-media-review.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("invalid source media review schema json: %v", err)
	}
	properties := schema["properties"].(map[string]any)
	if got := properties["version"].(map[string]any)["const"]; got != "vflow-source-media-review/v1" {
		t.Fatalf("version const = %#v", got)
	}
	sources := properties["sources"].(map[string]any)
	if got := sources["minItems"]; got != float64(1) {
		t.Fatalf("sources minItems = %#v", got)
	}
	sourceProperties := sources["items"].(map[string]any)["properties"].(map[string]any)
	for _, field := range []string{"width", "height"} {
		if got := sourceProperties[field].(map[string]any)["minimum"]; got != float64(1) {
			t.Fatalf("%s minimum = %#v", field, got)
		}
	}
	vfrEnum := sourceProperties["variable_frame_rate_status"].(map[string]any)["enum"].([]any)
	for _, want := range []string{"unknown", "likely_cfr", "possible_vfr"} {
		found := false
		for _, got := range vfrEnum {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("vfr enum missing %q in %#v", want, vfrEnum)
		}
	}
}

func TestProjectSchemaDocumentsRootContract(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "schemas", "project.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("invalid project schema json: %v", err)
	}
	properties := schema["properties"].(map[string]any)
	if got := properties["version"].(map[string]any)["const"]; got != "vflow-project/v1" {
		t.Fatalf("version const = %#v", got)
	}
	idPattern := properties["id"].(map[string]any)["pattern"]
	if idPattern != "^[A-Za-z0-9][A-Za-z0-9_.-]*$" {
		t.Fatalf("unexpected project id pattern: %#v", idPattern)
	}
	for _, field := range []string{"root", "created_at", "updated_at"} {
		if _, ok := properties[field]; !ok {
			t.Fatalf("project schema missing %s", field)
		}
	}
	required := schema["required"].([]any)
	for _, want := range []string{"version", "id", "root", "created_at", "updated_at"} {
		found := false
		for _, got := range required {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("project schema missing required field %q in %#v", want, required)
		}
	}
}

func TestGeminiVideoQASchemaDocumentsProviderWrapper(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "schemas", "gemini-video-qa.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("invalid gemini video qa schema json: %v", err)
	}
	properties := schema["properties"].(map[string]any)
	if got := properties["version"].(map[string]any)["const"]; got != "vflow-gemini-video-qa/v1" {
		t.Fatalf("version const = %#v", got)
	}
	uploadEnum := properties["upload"].(map[string]any)["enum"].([]any)
	for _, want := range []string{"files", "inline"} {
		found := false
		for _, got := range uploadEnum {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("upload enum missing %q in %#v", want, uploadEnum)
		}
	}
	if _, ok := properties["provider_response"]; !ok {
		t.Fatalf("schema missing provider_response")
	}
	required := schema["required"].([]any)
	for _, want := range []string{"version", "status", "provider", "model", "render", "upload", "report_path", "prompt", "provider_response"} {
		found := false
		for _, got := range required {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("gemini video qa schema missing required field %q in %#v", want, required)
		}
	}
}

func TestProviderBakeoffSchemaDocumentsProviderRunContract(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "schemas", "provider-bakeoff.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("invalid provider bakeoff schema json: %v", err)
	}
	properties := schema["properties"].(map[string]any)
	if got := properties["version"].(map[string]any)["const"]; got != "vflow-provider-bakeoff/v1" {
		t.Fatalf("version const = %#v", got)
	}
	providers := properties["providers"].(map[string]any)
	itemProperties := providers["items"].(map[string]any)["properties"].(map[string]any)
	statusEnum := itemProperties["status"].(map[string]any)["enum"].([]any)
	for _, want := range []string{"ready", "completed", "failed", "skipped_missing_key", "local_import_only", "invalid_provider"} {
		found := false
		for _, got := range statusEnum {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("provider bakeoff status enum missing %q in %#v", want, statusEnum)
		}
	}
	required := schema["required"].([]any)
	for _, want := range []string{"version", "status", "live", "source", "providers"} {
		found := false
		for _, got := range required {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("provider bakeoff schema missing required field %q in %#v", want, required)
		}
	}
}

func TestAuditReportSchemaDocumentsScorecardContract(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "schemas", "audit-report.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("invalid audit report schema json: %v", err)
	}
	properties := schema["properties"].(map[string]any)
	if got := properties["version"].(map[string]any)["const"]; got != "vflow-cli-audit/v1" {
		t.Fatalf("version const = %#v", got)
	}
	statusEnum := properties["status"].(map[string]any)["enum"].([]any)
	for _, want := range []string{"pass", "fail"} {
		found := false
		for _, got := range statusEnum {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("audit status enum missing %q in %#v", want, statusEnum)
		}
	}
	summaryRequired := properties["summary"].(map[string]any)["required"].([]any)
	for _, want := range []string{"commands", "mutating_commands", "mutating_commands_gated", "schema_count", "provider_live_adapters", "threshold_policy", "secrets_written_to_repo", "private_work_published"} {
		found := false
		for _, got := range summaryRequired {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("audit summary missing required field %q in %#v", want, summaryRequired)
		}
	}
}

func TestNLESidecarSchemaDocumentsRoundtripMapping(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "schemas", "nle-sidecar.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("invalid nle sidecar schema json: %v", err)
	}
	properties := schema["properties"].(map[string]any)
	if got := properties["version"].(map[string]any)["const"]; got != "vflow-nle-sidecar/v1" {
		t.Fatalf("version const = %#v", got)
	}
	targetEnum := properties["target"].(map[string]any)["enum"].([]any)
	for _, want := range []string{"edl", "fcpxml", "resolve", "premiere", "mlt", "otio", "sidecar"} {
		found := false
		for _, got := range targetEnum {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("target enum missing %q in %#v", want, targetEnum)
		}
	}
	segments := properties["segments"].(map[string]any)
	segmentRequired := segments["items"].(map[string]any)["required"].([]any)
	for _, want := range []string{"id", "vflow_segment_id", "source_media_id", "source_frame_in", "source_frame_out", "timeline_frame_in", "timeline_frame_out", "marker_ids", "export_target", "export_version"} {
		found := false
		for _, got := range segmentRequired {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("sidecar segment missing required field %q in %#v", want, segmentRequired)
		}
	}
}
