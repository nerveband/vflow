package cli

import (
	"os"
	"path/filepath"
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
