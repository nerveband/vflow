package media

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRunProxyExecutesFFmpegAndWritesOutput(t *testing.T) {
	dir := t.TempDir()
	ffmpeg := fakeFFmpeg(t, dir)
	source := filepath.Join(dir, "source.mp4")
	output := filepath.Join(dir, "proxy.mp4")
	if err := os.WriteFile(source, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	plan, err := RunProxy(context.Background(), ProxyOptions{FFmpegPath: ffmpeg, SourcePath: source, OutputPath: output, Preset: "edit-1080p", Overwrite: true})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != "written" {
		t.Fatalf("status = %q", plan.Status)
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatalf("expected output: %v", err)
	}
}

func TestRunSamplesExecutesFFmpegAndWritesContactSheet(t *testing.T) {
	dir := t.TempDir()
	ffmpeg := fakeFFmpeg(t, dir)
	source := filepath.Join(dir, "source.mp4")
	output := filepath.Join(dir, "contact.jpg")
	if err := os.WriteFile(source, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	plan, err := RunSamples(context.Background(), SampleOptions{FFmpegPath: ffmpeg, SourcePath: source, OutputPath: output, Count: 6, Overwrite: true})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != "written" || len(plan.Frames) != 6 {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatalf("expected output: %v", err)
	}
}

func fakeFFmpeg(t *testing.T, dir string) string {
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
