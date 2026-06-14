package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestColorReviewCommitWritesReportWithoutLiveProvider(t *testing.T) {
	project := t.TempDir()
	input := filepath.Join(project, "renders", "rough-preview.mp4")
	if err := os.MkdirAll(filepath.Dir(input), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(input, []byte("placeholder"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, errOut, code := runCLI(t, "color", "review", "--project", project, "--input", input, "--provider", "gemini", "--model", "gemini-3.5-flash", "--commit", "--format", "json")
	if code != 0 {
		t.Fatalf("color review failed: code=%d stdout=%s stderr=%s", code, out, errOut)
	}
	reportPath := filepath.Join(project, "reports", "color-grade-report.json")
	if _, err := os.Stat(reportPath); err != nil {
		t.Fatalf("expected report: %v", err)
	}
	if !strings.Contains(out, `"status": "written"`) || !strings.Contains(out, "color-grade-report.json") {
		t.Fatalf("unexpected output: %s", out)
	}
}
