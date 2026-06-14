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
	out, errOut, code := runCLI(t, "render", "preview", "--project", dir, "--source", source, "--format", "json")
	if code != 0 {
		t.Fatalf("expected success, got %d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, "rough-preview.mp4") {
		t.Fatalf("expected render output path in json: %s", out)
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
