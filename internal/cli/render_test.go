package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderPreviewDryCommandShape(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "media"), 0o755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "media", "source.mp4")
	if err := os.WriteFile(source, []byte("placeholder"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, errOut, code := runCLI(t,
		"render", "preview",
		"--project", dir,
		"--source", source,
		"--output", "renders/sample-30s.mp4",
		"--duration-seconds", "30",
		"--start-seconds", "12.5",
		"--format", "json",
	)
	if code != 0 {
		t.Fatalf("expected success, got %d stderr=%s", code, errOut)
	}
	for _, want := range []string{"sample-30s.mp4", `"-ss"`, `"12.500"`, `"-t"`, `"30"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q in:\n%s", want, out)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "reports", "render-report.json")); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not write render report, err=%v", err)
	}
}

func TestRenderVerifyAcceptsInputAlias(t *testing.T) {
	dir := t.TempDir()
	renderPath := filepath.Join(dir, "rough-preview.mp4")
	if err := os.WriteFile(renderPath, []byte("placeholder"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, errOut, code := runCLI(t, "render", "verify", "--input", renderPath, "--format", "json")
	if code != 0 {
		t.Fatalf("expected success, got %d stderr=%s", code, errOut)
	}
	for _, want := range []string{`"status": "exists"`, renderPath} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q in:\n%s", want, out)
		}
	}
}

func TestRenderVerifyUsesFFProbeJSON(t *testing.T) {
	out, errOut, code := runCLI(t,
		"render", "verify",
		"--render", "rough-preview.mp4",
		"--ffprobe-json", "../../fixtures/media/tiny/ffprobe.json",
		"--expected-width", "1920",
		"--expected-height", "1080",
		"--expected-duration", "12.345",
		"--format", "json",
	)
	if code != 0 {
		t.Fatalf("expected success, got %d stderr=%s", code, errOut)
	}
	for _, want := range []string{`"status": "valid"`, `"width": 1920`, `"audio_streams": 1`, `"frame_count": 370`} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q in:\n%s", want, out)
		}
	}
}

func TestRenderVerifyResolvesProjectRelativeRender(t *testing.T) {
	dir := t.TempDir()
	out, errOut, code := runCLI(t,
		"render", "verify",
		"--project", dir,
		"--render", "renders/rough-preview.mp4",
		"--ffprobe-json", "../../fixtures/media/tiny/ffprobe.json",
		"--expected-width", "1920",
		"--expected-height", "1080",
		"--expected-duration", "12.345",
		"--format", "json",
	)
	if code != 0 {
		t.Fatalf("expected success, got %d stderr=%s", code, errOut)
	}
	want := filepath.ToSlash(filepath.Join(dir, "renders", "rough-preview.mp4"))
	if !strings.Contains(out, want) {
		t.Fatalf("expected project-relative render path %q in:\n%s", want, out)
	}
}

func TestRenderTranscriptCutDryRunUsesEditDecision(t *testing.T) {
	dir := t.TempDir()
	editPath := filepath.Join(dir, "social-cut.json")
	edit := `{
  "version": "vflow-transcript-cut/v1",
  "segments": [
    {"id":"hook","source":"media/source-a.mp4","start_seconds":120,"end_seconds":128,"text":"10-year video"},
    {"id":"legacy","source":"media/source-b.mp4","start_seconds":12,"end_seconds":24,"text":"legacy"}
  ]
}`
	if err := os.WriteFile(editPath, []byte(edit), 0o644); err != nil {
		t.Fatal(err)
	}
	out, errOut, code := runCLI(t,
		"render", "transcript-cut",
		"--project", dir,
		"--input", editPath,
		"--output", "renders/social-30s.mp4",
		"--format", "json",
	)
	if code != 0 {
		t.Fatalf("expected success, got %d stderr=%s", code, errOut)
	}
	for _, want := range []string{`"status": "planned"`, "social-30s.mp4", "concat=n=2:v=1:a=1", `"duration_seconds": 20`} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q in:\n%s", want, out)
		}
	}
}

func TestRenderPreviewCommitWritesJobRecord(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "media"), 0o755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "media", "source.mp4")
	if err := os.WriteFile(source, []byte("placeholder"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, errOut, code := runCLI(t, "render", "preview", "--project", dir, "--source", source, "--ffmpeg-path", "true", "--commit", "--format", "json")
	if code != 0 {
		t.Fatalf("expected success, got %d stderr=%s", code, errOut)
	}
	out, errOut, code := runCLI(t, "jobs", "list", "--project", dir, "--format", "json")
	if code != 0 {
		t.Fatalf("jobs list failed: %d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, `"command": "render preview"`) || !strings.Contains(out, `"status": "succeeded"`) {
		t.Fatalf("expected render job record: %s", out)
	}
}
