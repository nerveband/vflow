package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanupApplyAndTimelineCompile(t *testing.T) {
	dir := t.TempDir()
	if _, _, code := runCLI(t, "project", "init", "--path", dir, "--id", "timeline_test", "--commit", "--format", "json"); code != 0 {
		t.Fatalf("project init failed")
	}
	deletePath := filepath.Join(dir, "delete_segments.json")
	if err := os.WriteFile(deletePath, []byte(`[{"start":1.0,"end":2.0,"reason":"retake","confidence":0.91}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, errOut, code := runCLI(t, "cleanup", "apply", "--project", dir, "--input", deletePath, "--rate", "30/1", "--commit", "--format", "json"); code != 0 {
		t.Fatalf("cleanup apply failed: %d %s", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, "decisions", "content-edl.json")); err != nil {
		t.Fatalf("expected content-edl: %v", err)
	}
	if _, errOut, code := runCLI(t, "timeline", "compile", "--project", dir, "--duration-frames", "120", "--commit", "--format", "json"); code != 0 {
		t.Fatalf("timeline compile failed: %d %s", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, "timeline", "compiled-timeline.json")); err != nil {
		t.Fatalf("expected compiled timeline: %v", err)
	}
}

func TestTimelineCompileRejectsMalformedContentEDL(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "decisions"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "decisions", "content-edl.json"), []byte(`{"version":`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, errOut, code := runCLI(t, "timeline", "compile", "--project", dir, "--format", "json", "--format-error", "json")
	if code != 4 {
		t.Fatalf("expected validation failure, got %d stderr=%s", code, errOut)
	}
	if !strings.Contains(errOut, `"code": "CONTENT_EDL_INVALID"`) {
		t.Fatalf("expected CONTENT_EDL_INVALID, got %s", errOut)
	}
}
