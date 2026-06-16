package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestProjectInitRejectsInvalidIDWithStructuredError(t *testing.T) {
	dir := t.TempDir()
	out, errOut, code := runCLI(t, "project", "init", "--path", dir, "--id", "bad id", "--format", "json", "--format-error", "json")
	if code == 0 {
		t.Fatalf("expected project init failure, stdout=%s stderr=%s", out, errOut)
	}
	for _, want := range []string{`"code": "PROJECT_INVALID"`, "stable project id"} {
		if !strings.Contains(errOut, want) {
			t.Fatalf("expected %q in stderr:\n%s", want, errOut)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "project.json")); !os.IsNotExist(err) {
		t.Fatalf("expected no project.json to be written, stat err=%v", err)
	}
}

func TestProjectIndexWritesSQLiteAndProvenance(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VFLOW_INDEX_PATH", filepath.Join(t.TempDir(), "index.sqlite"))
	if _, _, code := runCLI(t, "project", "init", "--path", dir, "--id", "index_cli", "--commit", "--format", "json"); code != 0 {
		t.Fatalf("project init failed")
	}
	if err := os.WriteFile(filepath.Join(dir, "transcript", "input.txt"), []byte("Ali: sadaqa zakat waqf\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, errOut, code := runCLI(t, "transcript", "import", "--project", dir, "--provider", "plain-text", "--input", filepath.Join(dir, "transcript", "input.txt"), "--commit", "--format", "json")
	if code != 0 {
		t.Fatalf("transcript import failed: %s", errOut)
	}

	out, errOut, code := runCLI(t, "project", "index", "--path", dir, "--commit", "--format", "json")
	if code != 0 {
		t.Fatalf("project index failed: code=%d stdout=%s stderr=%s", code, out, errOut)
	}
	if !strings.Contains(out, `"database_path":`) || !strings.Contains(out, `"provenance_path":`) {
		t.Fatalf("index output missing sqlite/provenance paths: %s", out)
	}
	if _, err := os.Stat(os.Getenv("VFLOW_INDEX_PATH")); err != nil {
		t.Fatalf("expected sqlite index: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "reports", "provenance.json")); err != nil {
		t.Fatalf("expected provenance artifact: %v", err)
	}
}
