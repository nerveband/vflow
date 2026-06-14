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
