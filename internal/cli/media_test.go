package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMediaProbeFixtureJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, code := runCLI(t, "project", "init", "--path", dir, "--id", "media_test", "--commit", "--format", "json"); code != 0 {
		t.Fatalf("project init failed")
	}
	probePath := filepath.Join(dir, "probe.json")
	raw, err := os.ReadFile("../../fixtures/media/tiny/ffprobe.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(probePath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	out, errOut, code := runCLI(t, "media", "probe", "--project", dir, "--ffprobe-json", probePath, "--format", "json", "--commit")
	if code != 0 {
		t.Fatalf("expected success, got %d stderr=%s", code, errOut)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "source-media-review.json")); err != nil {
		t.Fatalf("expected source-media-review.json: %v", err)
	}
}
