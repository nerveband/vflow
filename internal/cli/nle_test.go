package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNLEImportWritesArtifactAndDiffDeliversReviewHTML(t *testing.T) {
	project := t.TempDir()
	input := filepath.Join(project, "timeline.fcpxml")
	if err := os.WriteFile(input, []byte(`<?xml version="1.0"?><fcpxml version="1.11"></fcpxml>`), 0o644); err != nil {
		t.Fatal(err)
	}

	out, errOut, code := runCLI(t, "nle", "import", "--project", project, "--input", input, "--commit", "--format", "json")
	if code != 0 {
		t.Fatalf("nle import failed: code=%d stdout=%s stderr=%s", code, out, errOut)
	}
	importPath := filepath.Join(project, "imports", "nle-import.json")
	if _, err := os.Stat(importPath); err != nil {
		t.Fatalf("expected import artifact: %v", err)
	}

	reviewPath := filepath.Join(project, "review", "roundtrip-review.html")
	out, errOut, code = runCLI(t, "nle", "diff", "--project", project, "--import", importPath, "--deliver", "file:"+reviewPath, "--format", "json")
	if code != 0 {
		t.Fatalf("nle diff failed: code=%d stdout=%s stderr=%s", code, out, errOut)
	}
	if !strings.Contains(out, "roundtrip-review.html") {
		t.Fatalf("diff output missing review artifact: %s", out)
	}
	raw, err := os.ReadFile(reviewPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "Roundtrip Review") || !strings.Contains(string(raw), "blocked") {
		t.Fatalf("review HTML missing expected sections: %s", raw)
	}
}
