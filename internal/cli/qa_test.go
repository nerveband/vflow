package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQADoctorDryRunWithoutKeyReturnsWarning(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	out, errOut, code := runCLI(t, "qa", "doctor", "--provider", "gemini", "--model", "3.1 pro", "--format", "json")
	if code != 0 {
		t.Fatalf("expected success capability warning, got %d stderr=%s", code, errOut)
	}
	for _, want := range []string{"MISSING_API_KEY", "gemini-3.1-pro-preview"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q: %s", want, out)
		}
	}
}

func TestQADoctorUsesNamedKeyEnv(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GEMINI_CANDIDATE_1", "test-key")
	out, errOut, code := runCLI(t, "qa", "doctor", "--provider", "gemini", "--key-env", "GEMINI_CANDIDATE_1", "--format", "json")
	if code != 0 {
		t.Fatalf("expected success, got %d stderr=%s", code, errOut)
	}
	for _, want := range []string{`"key_source": "env:GEMINI_CANDIDATE_1"`, `"key_present": true`} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q: %s", want, out)
		}
	}
	if strings.Contains(out, "test-key") {
		t.Fatalf("output leaked key: %s", out)
	}
}

func TestQADoctorRejectsInvalidKeyEnv(t *testing.T) {
	_, errOut, code := runCLI(t, "qa", "doctor", "--provider", "gemini", "--key-env", "BAD-NAME", "--format-error", "json")
	if code != 4 {
		t.Fatalf("expected validation failure, got %d stderr=%s", code, errOut)
	}
	if !strings.Contains(errOut, "INVALID_KEY_ENV") {
		t.Fatalf("expected invalid key env error: %s", errOut)
	}
}

func TestQAAnalyzeDryRunDefaultsToFilesUpload(t *testing.T) {
	out, errOut, code := runCLI(t, "qa", "analyze", "--project", t.TempDir(), "--render", "../../fixtures/media/tiny/source.mp4", "--provider", "gemini", "--model", "gemini-3.5-flash", "--format", "json")
	if code != 0 {
		t.Fatalf("expected success, got %d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, `"upload": "files"`) {
		t.Fatalf("expected files upload mode: %s", out)
	}
}

func TestQAAnalyzeAppendReviewQueueDryRunDoesNotWrite(t *testing.T) {
	project := t.TempDir()
	out, errOut, code := runCLI(t, "qa", "analyze", "--project", project, "--render", "../../fixtures/media/tiny/source.mp4", "--provider", "gemini", "--model", "gemini-3.5-flash", "--append-review-queue", "--format", "json", "--format-error", "json")
	if code != 0 {
		t.Fatalf("expected success, got %d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, `"proposed_review_items": []`) || !strings.Contains(out, "review-queue.json") {
		t.Fatalf("dry-run output missing review queue plan: %s", out)
	}
	if _, err := os.Stat(filepath.Join(project, "review", "review-queue.json")); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not write review queue, stat err=%v", err)
	}
}

func TestQAAnalyzeRejectsInvalidUploadMode(t *testing.T) {
	_, errOut, code := runCLI(t, "qa", "analyze", "--upload", "bad", "--format", "json", "--format-error", "json")
	if code != 4 {
		t.Fatalf("expected validation error, got %d stderr=%s", code, errOut)
	}
	if !strings.Contains(errOut, "INVALID_ENUM") {
		t.Fatalf("expected invalid enum error: %s", errOut)
	}
}

func TestReviewItemsFromQAResponseFiltersHighConfidenceObservations(t *testing.T) {
	raw := json.RawMessage(`{
	  "candidates": [{"content": {"parts": [{"text": "{\"issues\":[{\"code\":\"caption occlusion\",\"message\":\"Caption covers lower-third speaker name\",\"confidence\":0.91,\"start_frame\":120,\"end_frame\":150},{\"code\":\"maybe\",\"message\":\"Low confidence note\",\"confidence\":0.5}]}"}]}}]
	}`)
	items := reviewItemsFromQAResponse(raw)
	if len(items) != 1 {
		t.Fatalf("expected one high-confidence item, got %#v", items)
	}
	item := items[0]
	if item.Code != "qa_caption_occlusion" || item.StartFrame != 120 || item.EndFrame != 150 || item.PresetID != "qa_video_output" {
		t.Fatalf("unexpected item: %#v", item)
	}
}

func TestAppendReviewQueueItemsPreservesExistingItems(t *testing.T) {
	project := t.TempDir()
	path := filepath.Join(project, "review", "review-queue.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version":"vflow-review-queue/v1","items":[{"id":"rev_000001","code":"existing","severity":"needs_human_review","message":"Existing item","event_id":"fr_000001","start_frame":0,"end_frame":10,"preset_id":"wide"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	items := reviewItemsFromQAResponse(json.RawMessage(`{"observations":[{"code":"wrong_speaker","message":"Wrong speaker on screen","confidence":0.86,"time_seconds":2.0}]}`))
	if err := appendReviewQueueItems(project, items); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"id": "rev_000001"`, `"id": "rev_000002"`, `"code": "qa_wrong_speaker"`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("review queue missing %s in:\n%s", want, raw)
		}
	}
}
