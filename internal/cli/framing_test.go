package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFramingPresetImportAndValidate(t *testing.T) {
	dir := t.TempDir()
	if _, _, code := runCLI(t, "project", "init", "--path", dir, "--id", "framing_test", "--commit", "--format", "json"); code != 0 {
		t.Fatalf("project init failed")
	}
	input := filepath.Join(dir, "framing-presets.json")
	raw, err := os.ReadFile("../../fixtures/project/basic/calibration/framing-presets.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(input, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, errOut, code := runCLI(t, "framing", "preset", "import", "--project", dir, "--input", input, "--commit", "--format", "json"); code != 0 {
		t.Fatalf("preset import failed: %d %s", code, errOut)
	}
	if _, errOut, code := runCLI(t, "framing", "preset", "validate", "--project", dir, "--format", "json"); code != 0 {
		t.Fatalf("preset validate failed: %d %s", code, errOut)
	}
}

func TestFramingCalibratePrintsManagedSessionJSON(t *testing.T) {
	dir := t.TempDir()
	if _, errOut, code := runCLI(t, "project", "init", "--path", dir, "--id", "framing_calibrate_test", "--commit", "--format", "json"); code != 0 {
		t.Fatalf("project init failed: %d %s", code, errOut)
	}
	out, errOut, code := runCLI(t, "framing", "calibrate", "--project", dir, "--listen", "127.0.0.1:0", "--open=false", "--wait=false", "--session-timeout", "2s", "--shutdown-token", "test-token", "--format", "json", "--format-error", "json")
	if code != 0 {
		t.Fatalf("calibrate failed: %d %s", code, errOut)
	}
	for _, want := range []string{`"command": "framing calibrate"`, `"session_id":`, `"url": "http://127.0.0.1:`, `"health_url":`, `"status_url":`, `"shutdown_url":`, `"shutdown_token_present": true`, `"framing_presets":`} {
		if !strings.Contains(out, want) {
			t.Fatalf("calibrate output missing %s in:\n%s", want, out)
		}
	}
}

func TestFramingCalibrateCropZoomAliases(t *testing.T) {
	for _, alias := range []string{"crop", "zoom", "reframe", "frame", "crop-calibrate", "zoom-calibrate", "preset-calibrate"} {
		dir := t.TempDir()
		if _, errOut, code := runCLI(t, "project", "init", "--path", dir, "--id", "framing_"+alias+"_test", "--commit", "--format", "json"); code != 0 {
			t.Fatalf("project init failed for %s: %d %s", alias, code, errOut)
		}
		out, errOut, code := runCLI(t, "framing", alias, "--project", dir, "--listen", "127.0.0.1:0", "--open=false", "--wait=false", "--session-timeout", "2s", "--format", "json", "--format-error", "json")
		if code != 0 {
			t.Fatalf("framing %s failed: %d %s", alias, code, errOut)
		}
		if !strings.Contains(out, `"command": "framing calibrate"`) || !strings.Contains(out, `"session_id":`) {
			t.Fatalf("alias %s did not return calibrate session JSON:\n%s", alias, out)
		}
	}
}

func TestFramingCalibrateAcceptsAbsoluteExternalSource(t *testing.T) {
	dir := t.TempDir()
	external := filepath.Join(t.TempDir(), "proxy-source.mp4")
	if err := os.WriteFile(external, []byte("fake-media"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, errOut, code := runCLI(t, "project", "init", "--path", dir, "--id", "framing_external_source_test", "--commit", "--format", "json"); code != 0 {
		t.Fatalf("project init failed: %d %s", code, errOut)
	}
	out, errOut, code := runCLI(t, "framing", "calibrate", "--project", dir, "--source", external, "--listen", "127.0.0.1:0", "--open=false", "--wait=false", "--session-timeout", "2s", "--format", "json", "--format-error", "json")
	if code != 0 {
		t.Fatalf("calibrate with external source failed: %d %s", code, errOut)
	}
	if !strings.Contains(out, `"command": "framing calibrate"`) || !strings.Contains(out, `"session_id":`) {
		t.Fatalf("unexpected calibrate output:\n%s", out)
	}
}

func TestFramingCompileBuildsLaneAndReviewQueueFromProjectArtifacts(t *testing.T) {
	dir := t.TempDir()
	if _, errOut, code := runCLI(t, "project", "init", "--path", dir, "--id", "framing_compile_test", "--commit", "--format", "json"); code != 0 {
		t.Fatalf("project init failed: %d %s", code, errOut)
	}
	raw, err := os.ReadFile("../../fixtures/project/basic/calibration/framing-presets.json")
	if err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(dir, "framing-presets.json")
	if err := os.WriteFile(input, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, errOut, code := runCLI(t, "framing", "preset", "import", "--project", dir, "--input", input, "--commit", "--format", "json"); code != 0 {
		t.Fatalf("preset import failed: %d %s", code, errOut)
	}
	writeJSONForTest(t, filepath.Join(dir, "calibration", "speaker-map.json"), map[string]any{
		"version": "vflow-speaker-map/v1",
		"map":     map[string]string{"SPEAKER_00": "speaker_ali_medium"},
	})
	writeJSONForTest(t, filepath.Join(dir, "policy", "framing-policy.json"), map[string]any{
		"version":                  "vflow-framing-policy/v1",
		"min_dwell_frames":         5,
		"low_confidence_threshold": 0.70,
		"wide_preset_id":           "wide",
		"wide_reset_frames":        100,
	})
	writeJSONForTest(t, filepath.Join(dir, "transcript", "words.json"), map[string]any{
		"version":         "vflow-words/v1",
		"source_media_id": "source",
		"rate":            "30/1",
		"words": []map[string]any{{
			"id":            "w_000001",
			"text":          "Bismillah",
			"speaker_label": "SPEAKER_00",
			"start_frame":   0,
			"end_frame":     60,
			"confidence":    0.98,
			"provider":      "test",
		}},
	})

	out, errOut, code := runCLI(t, "framing", "compile", "--project", dir, "--format", "json")
	if code != 0 {
		t.Fatalf("dry-run compile failed: %d %s", code, errOut)
	}
	if !strings.Contains(out, `"status": "planned"`) || !strings.Contains(out, `"preset_id": "speaker_ali_medium"`) {
		t.Fatalf("dry-run output missing compiled lane: %s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "decisions", "framing-lane.json")); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not write framing-lane.json")
	}

	if _, errOut, code := runCLI(t, "framing", "compile", "--project", dir, "--commit", "--format", "json"); code != 0 {
		t.Fatalf("commit compile failed: %d %s", code, errOut)
	}
	for _, path := range []string{
		filepath.Join(dir, "decisions", "framing-lane.json"),
		filepath.Join(dir, "review", "review-queue.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected artifact %s: %v", path, err)
		}
	}
}

func TestFramingPresetImportRejectsAlphaSpeakerLabel(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "bad-framing-presets.json")
	raw := []byte(`{
  "version": "vflow-framing-presets/v1",
  "source_width": 3840,
  "source_height": 2160,
  "target_aspect": "16:9",
  "presets": [
    {"id": "SPEAKER_A", "label": "Speaker A", "type": "speaker", "crop_px": {"x": 0, "y": 0, "w": 1920, "h": 1080}}
  ]
}`)
	if err := os.WriteFile(input, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	_, errOut, code := runCLI(t, "framing", "preset", "import", "--project", dir, "--input", input, "--format", "json", "--format-error", "json")
	if code != 4 {
		t.Fatalf("expected validation failure, got %d stderr=%s", code, errOut)
	}
	if !strings.Contains(errOut, `"code": "FRAMING_PRESET_INVALID"`) {
		t.Fatalf("expected FRAMING_PRESET_INVALID, got %s", errOut)
	}
}

func writeJSONForTest(t *testing.T, path string, value any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
