package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestProjectInitCreatesExpectedLayout(t *testing.T) {
	dir := t.TempDir()
	out, errOut, code := runCLI(t, "project", "init", "--path", dir, "--id", "panel_test", "--format", "json", "--commit")
	if code != 0 {
		t.Fatalf("expected success, got %d stderr=%s", code, errOut)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out)
	}
	for _, rel := range []string{"project.json", "media", "transcript", "calibration", "policy", "decisions", "timeline", "review", "renders", "exports", "imports", "reports", "jobs"} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Fatalf("expected %s: %v", rel, err)
		}
	}
}
