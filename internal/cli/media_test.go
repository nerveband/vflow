package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestDiscoverMediaSourcesHonorsExistingRelativePath(t *testing.T) {
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})
	if err := os.MkdirAll("tmp", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("tmp", "source.mp4"), []byte("not a real movie"), 0o644); err != nil {
		t.Fatal(err)
	}

	sources, err := discoverMediaSources(filepath.Join(dir, "project"), filepath.Join("tmp", "source.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := sources[0], filepath.Join("tmp", "source.mp4"); got != want {
		t.Fatalf("source = %q, want %q", got, want)
	}
}

func TestMediaProbeFFProbeFailureUsesStructuredJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "source.mp4"), []byte("not a real movie"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, errOut, code := runCLI(t, "media", "probe", "--project", dir, "--source", filepath.Join(dir, "source.mp4"), "--format", "json", "--format-error", "json")
	if code != 8 {
		t.Fatalf("expected exit code 8, got %d stderr=%s", code, errOut)
	}
	for _, want := range []string{`"ok": false`, `"schema_version": "vflow-error/v1"`, `"code": "FFPROBE_FAILED"`} {
		if !strings.Contains(errOut, want) {
			t.Fatalf("stderr missing %s in:\n%s", want, errOut)
		}
	}
}

func TestMediaProxyCommitRunsConfiguredFFmpeg(t *testing.T) {
	dir := t.TempDir()
	ffmpeg := fakeCLIFFmpeg(t, dir)
	source := filepath.Join(dir, "media", "source.mp4")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, errOut, code := runCLI(t, "media", "proxy", "--project", dir, "--source", source, "--ffmpeg-path", ffmpeg, "--commit", "--overwrite", "--format", "json", "--format-error", "json")
	if code != 0 {
		t.Fatalf("expected success, got %d stdout=%s stderr=%s", code, out, errOut)
	}
	if !strings.Contains(out, `"status": "written"`) {
		t.Fatalf("expected written status: %s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "media", "proxy.mp4")); err != nil {
		t.Fatalf("expected proxy output: %v", err)
	}
}

func TestMediaSamplesCommitRunsConfiguredFFmpeg(t *testing.T) {
	dir := t.TempDir()
	ffmpeg := fakeCLIFFmpeg(t, dir)
	source := filepath.Join(dir, "media", "source.mp4")
	output := filepath.Join(dir, "reports", "contact.jpg")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, errOut, code := runCLI(t, "media", "samples", "--project", dir, "--source", source, "--ffmpeg-path", ffmpeg, "--count", "6", "--deliver", "file:"+output, "--commit", "--overwrite", "--format", "json", "--format-error", "json")
	if code != 0 {
		t.Fatalf("expected success, got %d stdout=%s stderr=%s", code, out, errOut)
	}
	if !strings.Contains(out, `"status": "written"`) {
		t.Fatalf("expected written status: %s", out)
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatalf("expected contact sheet: %v", err)
	}
}

func fakeCLIFFmpeg(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "ffmpeg")
	script := `#!/bin/sh
out=""
for arg do
  out="$arg"
done
mkdir -p "$(dirname "$out")"
printf fake > "$out"
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
