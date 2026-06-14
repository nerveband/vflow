package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTranscriptImportWritesCanonicalWords(t *testing.T) {
	dir := t.TempDir()
	if _, _, code := runCLI(t, "project", "init", "--path", dir, "--id", "transcript_test", "--commit", "--format", "json"); code != 0 {
		t.Fatalf("project init failed")
	}
	input := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(input, []byte("Ali: Bismillah welcome\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, errOut, code := runCLI(t, "transcript", "import", "--project", dir, "--provider", "plain-text", "--input", input, "--commit", "--format", "json")
	if code != 0 {
		t.Fatalf("expected success, got %d stderr=%s", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, "transcript", "words.json")); err != nil {
		t.Fatalf("expected words.json: %v", err)
	}
}

func TestTranscriptSearchLocalUsesProjectIndex(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VFLOW_INDEX_PATH", filepath.Join(t.TempDir(), "index.sqlite"))
	if _, _, code := runCLI(t, "project", "init", "--path", dir, "--id", "search_cli", "--commit", "--format", "json"); code != 0 {
		t.Fatalf("project init failed")
	}
	input := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(input, []byte("Ali: bismillah zakat sadaqa\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, errOut, code := runCLI(t, "transcript", "import", "--project", dir, "--provider", "plain-text", "--input", input, "--commit", "--format", "json")
	if code != 0 {
		t.Fatalf("transcript import failed: %s", errOut)
	}
	_, errOut, code = runCLI(t, "project", "index", "--path", dir, "--commit", "--format", "json")
	if code != 0 {
		t.Fatalf("project index failed: %s", errOut)
	}

	out, errOut, code := runCLI(t, "transcript", "search", "--project", dir, "--query", "zakat", "--data-source", "local", "--format", "json")
	if code != 0 {
		t.Fatalf("transcript search failed: code=%d stdout=%s stderr=%s", code, out, errOut)
	}
	for _, want := range []string{`"data_source": "local"`, `"text": "zakat"`, `"project_id": "search_cli"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("search output missing %s in:\n%s", want, out)
		}
	}
}
