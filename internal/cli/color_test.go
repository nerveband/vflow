package cli

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
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
	raw, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("expected report: %v", err)
	}
	var report map[string]any
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatalf("invalid report json: %v\n%s", err, raw)
	}
	if report["status"] != "written" {
		t.Fatalf("expected persisted report status written, got %#v in %s", report["status"], raw)
	}
	if !strings.Contains(out, `"status": "written"`) || !strings.Contains(out, "color-grade-report.json") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestColorApplyCommitRecordsColorInRenderReport(t *testing.T) {
	project := t.TempDir()
	input := filepath.Join(project, "renders", "rough-preview.mp4")
	if err := os.MkdirAll(filepath.Dir(input), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(input, []byte("placeholder"), 0o644); err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(project, "reports", "render-report.json")
	if err := os.MkdirAll(filepath.Dir(reportPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(reportPath, []byte(`{"version":"vflow-render-report/v1","status":"rendered","render_path":"renders/rough-preview.mp4","command":["ffmpeg"],"target":"youtube_16x9"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	lutPath := filepath.Join(project, "calibration", "look.cube")
	lutRaw := []byte("TITLE \"test\"\nLUT_3D_SIZE 2\n0 0 0\n0 0 1\n0 1 0\n0 1 1\n1 0 0\n1 0 1\n1 1 0\n1 1 1\n")
	if err := os.MkdirAll(filepath.Dir(lutPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lutPath, lutRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	out, errOut, code := runCLI(t,
		"color", "apply",
		"--project", project,
		"--input", input,
		"--lut", lutPath,
		"--deliver", "file:renders/rough-preview-graded.mp4",
		"--intent", "final",
		"--qa-report", "reports/gemini-video-qa.json",
		"--ffmpeg-path", "true",
		"--commit",
		"--format", "json",
	)
	if code != 0 {
		t.Fatalf("color apply failed: code=%d stdout=%s stderr=%s", code, out, errOut)
	}
	raw, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(lutRaw)
	for _, want := range []string{
		`"color"`,
		`"ungraded_render_path": "` + filepath.ToSlash(input) + `"`,
		`"graded_render_path": "` + filepath.ToSlash(filepath.Join(project, "renders", "rough-preview-graded.mp4")) + `"`,
		`"lut_sha256": "` + fmt.Sprintf("%x", sum) + `"`,
		`"ffmpeg_filtergraph": "lut3d=file=` + filepath.ToSlash(lutPath) + `:interp=tetrahedral"`,
		`"intent": "final"`,
		`"reports/gemini-video-qa.json"`,
	} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("render report missing %s in:\n%s", want, raw)
		}
	}
}

func TestColorApplyRejectsInvalidIntent(t *testing.T) {
	project := t.TempDir()
	out, errOut, code := runCLI(t,
		"color", "apply",
		"--project", project,
		"--input", "renders/rough-preview.mp4",
		"--lut", "calibration/look.cube",
		"--intent", "review",
		"--format", "json",
		"--format-error", "json",
	)
	if code == 0 {
		t.Fatalf("expected invalid intent failure, stdout=%s stderr=%s", out, errOut)
	}
	for _, want := range []string{`"code": "INVALID_ENUM"`, "Use --intent preview or --intent final"} {
		if !strings.Contains(errOut, want) {
			t.Fatalf("expected %q in stderr:\n%s", want, errOut)
		}
	}
}

func TestColorReviewRejectsUnsupportedProvider(t *testing.T) {
	out, errOut, code := runCLI(t,
		"color", "review",
		"--provider", "openai",
		"--format", "json",
		"--format-error", "json",
	)
	if code == 0 {
		t.Fatalf("expected unsupported provider failure, stdout=%s stderr=%s", out, errOut)
	}
	for _, want := range []string{`"code": "INVALID_ENUM"`, "Use provider gemini"} {
		if !strings.Contains(errOut, want) {
			t.Fatalf("expected %q in stderr:\n%s", want, errOut)
		}
	}
}
