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
	artifactRaw, err := os.ReadFile(filepath.Join(dir, "source-media-review.json"))
	if err != nil {
		t.Fatal(err)
	}
	var artifact map[string]any
	if err := json.Unmarshal(artifactRaw, &artifact); err != nil {
		t.Fatalf("invalid source media review artifact json: %v\n%s", err, artifactRaw)
	}
	if artifact["version"] != "vflow-source-media-review/v1" {
		t.Fatalf("unexpected artifact version: %#v", artifact["version"])
	}
	if artifact["status"] != nil || artifact["review_path"] != nil {
		t.Fatalf("source-media-review.json should not persist CLI response envelope:\n%s", artifactRaw)
	}
	sources := artifact["sources"].([]any)
	if len(sources) != 1 {
		t.Fatalf("expected one source in artifact, got %#v", sources)
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

func TestMediaIngestReferenceWritesReviewWithoutCopy(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "external-source.mp4")
	if err := os.WriteFile(source, []byte("external media placeholder"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, errOut, code := runCLI(t,
		"media", "ingest",
		"--project", dir,
		"--source", source,
		"--reference",
		"--ffprobe-json", "../../fixtures/media/tiny/ffprobe.json",
		"--commit",
		"--format", "json",
		"--format-error", "json",
	)
	if code != 0 {
		t.Fatalf("expected success, got %d stdout=%s stderr=%s", code, out, errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, "media", filepath.Base(source))); !os.IsNotExist(err) {
		t.Fatalf("reference ingest should not copy media, stat err=%v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "source-media-review.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"status": "referenced"`, `"ingest_mode": "reference"`, filepath.ToSlash(source)} {
		if !strings.Contains(out+string(raw), want) {
			t.Fatalf("reference ingest missing %q\nstdout=%s\nreview=%s", want, out, raw)
		}
	}
}

func TestMediaIngestLinkCreatesProjectLink(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "external-source.mp4")
	if err := os.WriteFile(source, []byte("external media placeholder"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, errOut, code := runCLI(t,
		"media", "ingest",
		"--project", dir,
		"--source", source,
		"--link",
		"--ffprobe-json", "../../fixtures/media/tiny/ffprobe.json",
		"--commit",
		"--format", "json",
		"--format-error", "json",
	)
	if code != 0 {
		t.Fatalf("expected success, got %d stdout=%s stderr=%s", code, out, errOut)
	}
	dest := filepath.Join(dir, "media", filepath.Base(source))
	target, err := os.Readlink(dest)
	if err != nil {
		t.Fatalf("expected project media symlink at %s: %v", dest, err)
	}
	if target != source {
		t.Fatalf("link target = %q, want %q", target, source)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "source-media-review.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out+string(raw), `"ingest_mode": "link"`) || !strings.Contains(out, `"status": "linked"`) {
		t.Fatalf("link ingest missing mode/status\nstdout=%s\nreview=%s", out, raw)
	}
}

func TestMediaSyncTranscriptMethodWritesSyncMap(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "transcript"), 0o755); err != nil {
		t.Fatal(err)
	}
	referenceWords := `{"version":"vflow-words/v1","source_media_id":"atem","rate":"30/1","words":[
{"id":"r1","text":"cair","start_frame":300,"end_frame":315,"provider":"test"},
{"id":"r2","text":"georgia","start_frame":330,"end_frame":345,"provider":"test"},
{"id":"r3","text":"served","start_frame":360,"end_frame":375,"provider":"test"}]}`
	sourceWords := `{"version":"vflow-words/v1","source_media_id":"9mm","rate":"30/1","words":[
{"id":"s1","text":"CAIR","start_frame":4890,"end_frame":4905,"provider":"test"},
{"id":"s2","text":"Georgia","start_frame":4920,"end_frame":4935,"provider":"test"},
{"id":"s3","text":"served","start_frame":4950,"end_frame":4965,"provider":"test"}]}`
	refPath := filepath.Join(dir, "transcript", "atem.words.json")
	srcPath := filepath.Join(dir, "transcript", "9mm.words.json")
	if err := os.WriteFile(refPath, []byte(referenceWords), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcPath, []byte(sourceWords), 0o644); err != nil {
		t.Fatal(err)
	}

	out, errOut, code := runCLI(t,
		"media", "sync",
		"--project", dir,
		"--method", "transcript",
		"--reference-source-id", "atem",
		"--reference-words", refPath,
		"--source-words", "9mm="+srcPath,
		"--output", "calibration/media-sync-map.json",
		"--commit",
		"--format", "json",
		"--format-error", "json",
	)
	if code != 0 {
		t.Fatalf("expected success, got %d stdout=%s stderr=%s", code, out, errOut)
	}
	for _, want := range []string{
		`"method": "transcript_anchor"`,
		`"offset_from_reference_seconds": 153`,
		`"match_count": 3`,
		`"max_residual_seconds": 0`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %s in:\n%s", want, out)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "calibration", "media-sync-map.json")); err != nil {
		t.Fatalf("expected sync map: %v", err)
	}
}

func TestMediaSyncTranscriptMethodRejectsLowConfidenceCommit(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "transcript"), 0o755); err != nil {
		t.Fatal(err)
	}
	refPath := filepath.Join(dir, "transcript", "atem.words.json")
	srcPath := filepath.Join(dir, "transcript", "9mm.words.json")
	if err := os.WriteFile(refPath, []byte(`{"version":"vflow-words/v1","source_media_id":"atem","rate":"30/1","words":[{"id":"r1","text":"unique","start_frame":0,"end_frame":1,"provider":"test"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcPath, []byte(`{"version":"vflow-words/v1","source_media_id":"9mm","rate":"30/1","words":[{"id":"s1","text":"unique","start_frame":300,"end_frame":301,"provider":"test"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, errOut, code := runCLI(t,
		"media", "sync",
		"--project", dir,
		"--method", "transcript",
		"--reference-source-id", "atem",
		"--reference-words", refPath,
		"--source-words", "9mm="+srcPath,
		"--output", "calibration/media-sync-map.json",
		"--commit",
		"--format", "json",
		"--format-error", "json",
	)
	if code != 4 {
		t.Fatalf("expected validation failure, got %d stderr=%s", code, errOut)
	}
	if !strings.Contains(errOut, "confidence") || !strings.Contains(errOut, "below 0.65") {
		t.Fatalf("expected low-confidence error, got:\n%s", errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, "calibration", "media-sync-map.json")); !os.IsNotExist(err) {
		t.Fatalf("low-confidence commit should not write sync map, stat err=%v", err)
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

func TestMediaExtractRangesResolvesProjectRelativePaths(t *testing.T) {
	dir := t.TempDir()
	writeSyncFixture(t, dir)

	out, errOut, code := runCLI(t,
		"media", "extract-ranges",
		"--project", dir,
		"--sync-map", "calibration/media-sync-map.json",
		"--ranges", "decisions/ranges.json",
		"--output-dir", "media/sync-ranges",
		"--manifest", "calibration/range-manifest.json",
		"--format", "json",
		"--format-error", "json",
	)
	if code != 0 {
		t.Fatalf("expected success, got %d stdout=%s stderr=%s", code, out, errOut)
	}
	for _, want := range []string{
		filepath.ToSlash(filepath.Join(dir, "media", "sync-ranges", "seg1-7mm.mp4")),
		filepath.ToSlash(filepath.Join(dir, "calibration", "range-manifest.json")),
		`"source_start_seconds": 2432`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q in:\n%s", want, out)
		}
	}
}

func TestCutCreateResolvesProjectRelativePaths(t *testing.T) {
	dir := t.TempDir()
	writeSyncFixture(t, dir)

	out, errOut, code := runCLI(t,
		"cut", "create",
		"--project", dir,
		"--sync-map", "calibration/media-sync-map.json",
		"--ranges", "decisions/ranges.json",
		"--output", "decisions/cut.json",
		"--commit",
		"--format", "json",
		"--format-error", "json",
	)
	if code != 0 {
		t.Fatalf("expected success, got %d stdout=%s stderr=%s", code, out, errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, "decisions", "cut.json")); err != nil {
		t.Fatalf("expected project-relative cut output: %v", err)
	}
	if !strings.Contains(out, `"source_timeline_offset": 373`) {
		t.Fatalf("expected sync-resolved cut: %s", out)
	}
}

func TestCutCreateAcceptsAtFileRanges(t *testing.T) {
	dir := t.TempDir()
	writeSyncFixture(t, dir)

	out, errOut, code := runCLI(t,
		"cut", "create",
		"--project", dir,
		"--sync-map", "calibration/media-sync-map.json",
		"--ranges", "@"+filepath.Join(dir, "decisions", "ranges.json"),
		"--format", "json",
		"--format-error", "json",
	)
	if code != 0 {
		t.Fatalf("expected success, got %d stdout=%s stderr=%s", code, out, errOut)
	}
	if !strings.Contains(out, `"status": "planned"`) || !strings.Contains(out, `"source_timeline_offset": 373`) {
		t.Fatalf("expected sync-resolved planned cut: %s", out)
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

func writeSyncFixture(t *testing.T, dir string) {
	t.Helper()
	for _, subdir := range []string{"calibration", "decisions"} {
		if err := os.MkdirAll(filepath.Join(dir, subdir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	syncMap := `{
  "version": "vflow-media-sync-map/v1",
  "method": "audio_xcorr_envelope",
  "reference_source_id": "12mm",
  "transcript_to_reference_offset_seconds": 356,
  "sources": [
    {"id": "12mm", "path": "12.mp4", "offset_from_reference_seconds": 0, "confidence": 1},
    {"id": "7mm", "path": "7.mp4", "offset_from_reference_seconds": 17, "confidence": 0.9}
  ]
}`
	ranges := `{"ranges":[{"id":"seg1","source_id":"7mm","transcript_start_seconds":2059,"transcript_end_seconds":2064,"text":"has your back"}]}`
	if err := os.WriteFile(filepath.Join(dir, "calibration", "media-sync-map.json"), []byte(syncMap), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "decisions", "ranges.json"), []byte(ranges), 0o644); err != nil {
		t.Fatal(err)
	}
}
