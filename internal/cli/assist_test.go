package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorReportsFinishingAdapters(t *testing.T) {
	out, errOut, code := runCLI(t, "doctor", "--format", "json", "--format-error", "json")
	if code != 0 {
		t.Fatalf("expected success, got %d stderr=%s", code, errOut)
	}
	for _, want := range []string{`"finishing_adapters"`, `"task": "captions"`, `"contract_schema_ids"`, `"verification": true`} {
		if !strings.Contains(out, want) {
			t.Fatalf("doctor output missing %q in:\n%s", want, out)
		}
	}
}

func TestSuggestCaptionsReturnsRankedRunnableRecommendation(t *testing.T) {
	project := t.TempDir()
	writeAssistFixture(t, project)

	out, errOut, code := runCLI(t, "suggest", "captions", "--project", project, "--format", "json", "--format-error", "json")
	if code != 0 {
		t.Fatalf("expected success, got %d stderr=%s", code, errOut)
	}
	for _, want := range []string{
		`"schema_version": "vflow-suggestion/v1"`,
		`"task": "captions"`,
		`"contract_schema": "caption-cues.schema.json"`,
		`"invocation"`,
		`"detected_capabilities"`,
		`"missing_tool_hints"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("suggest output missing %q in:\n%s", want, out)
		}
	}
}

func TestVerifyCaptionsRoutesDriftToReviewQueueOnCommit(t *testing.T) {
	project := t.TempDir()
	writeAssistFixture(t, project)
	specPath := filepath.Join(project, "decisions", "caption-cues.json")
	writeTestJSONFile(t, specPath, map[string]any{
		"version":          "vflow-caption-cues/v1",
		"words_ref":        "transcript/words.json",
		"style_id":         "caption.default",
		"filler_clean":     true,
		"max_drift_frames": 1,
		"cues": []map[string]any{{
			"id": "cap_001", "text": "hello world", "word_ids": []string{"w1"}, "start_frame": 4, "end_frame": 20,
		}},
	})

	out, errOut, code := runCLI(t, "verify", "captions", "--project", project, "--spec", specPath, "--commit", "--format", "json", "--format-error", "json")
	if code != 0 {
		t.Fatalf("expected reviewable success, got %d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, `"status": "failed"`) || !strings.Contains(out, `"review_queue_path"`) {
		t.Fatalf("verify output did not report routed failure:\n%s", out)
	}
	raw, err := os.ReadFile(filepath.Join(project, "review", "review-queue.json"))
	if err != nil {
		t.Fatalf("expected review queue to be written: %v", err)
	}
	if !strings.Contains(string(raw), "caption_timing_drift") {
		t.Fatalf("review queue missing caption drift item:\n%s", string(raw))
	}
}

func TestVerifyCaptionsHappyPath(t *testing.T) {
	project := t.TempDir()
	writeAssistFixture(t, project)
	specPath := filepath.Join(project, "decisions", "caption-cues.json")
	writeTestJSONFile(t, specPath, map[string]any{
		"version":          "vflow-caption-cues/v1",
		"words_ref":        "transcript/words.json",
		"style_id":         "caption.default",
		"filler_clean":     true,
		"max_drift_frames": 1,
		"cues": []map[string]any{{
			"id": "cap_001", "text": "hello", "word_ids": []string{"w1"}, "start_frame": 0, "end_frame": 15,
		}},
	})

	out, errOut, code := runCLI(t, "verify", "captions", "--project", project, "--spec", specPath, "--format", "json", "--format-error", "json")
	if code != 0 {
		t.Fatalf("expected success, got %d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, `"status": "passed"`) || strings.Contains(out, "review_queue_path") {
		t.Fatalf("caption happy path should pass without review queue:\n%s", out)
	}
}

func TestVerifyAudioHappyPath(t *testing.T) {
	project := t.TempDir()
	writeAssistFixture(t, project)
	specPath := filepath.Join(project, "decisions", "audio-intent.json")
	writeTestJSONFile(t, specPath, map[string]any{
		"version":              "vflow-audio-intent/v1",
		"bed_ref":              "media/music.wav",
		"duck_target_db":       -18,
		"loudness_target_lufs": -16,
		"speech_segments":      []map[string]any{{"id": "seg_001", "start_frame": 0, "end_frame": 30}},
	})
	reportPath := filepath.Join(project, "reports", "audio-report.json")
	writeTestJSONFile(t, reportPath, map[string]any{
		"integrated_lufs":  -16.4,
		"true_peak_db":     -1.5,
		"clipping":         false,
		"timing_preserved": true,
	})

	out, errOut, code := runCLI(t, "verify", "audio", "--project", project, "--spec", specPath, "--output", reportPath, "--format", "json", "--format-error", "json")
	if code != 0 {
		t.Fatalf("expected success, got %d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, `"status": "passed"`) {
		t.Fatalf("verify output did not pass:\n%s", out)
	}
}

func TestVerifyAudioFailureRoutesReviewQueue(t *testing.T) {
	project := t.TempDir()
	writeAssistFixture(t, project)
	specPath := filepath.Join(project, "decisions", "audio-intent.json")
	writeTestJSONFile(t, specPath, map[string]any{
		"version":              "vflow-audio-intent/v1",
		"bed_ref":              "media/music.wav",
		"duck_target_db":       -18,
		"loudness_target_lufs": -16,
		"speech_segments":      []map[string]any{{"id": "seg_001", "start_frame": 0, "end_frame": 30}},
	})
	reportPath := filepath.Join(project, "reports", "audio-report.json")
	writeTestJSONFile(t, reportPath, map[string]any{
		"integrated_lufs":  -9.0,
		"true_peak_db":     1.2,
		"clipping":         true,
		"timing_preserved": false,
	})

	out, errOut, code := runCLI(t, "verify", "audio", "--project", project, "--spec", specPath, "--output", reportPath, "--commit", "--format", "json", "--format-error", "json")
	if code != 0 {
		t.Fatalf("expected reviewable success, got %d stderr=%s", code, errOut)
	}
	for _, want := range []string{`"status": "failed"`, "audio_loudness_range", "audio_clipping", "audio_timing_preservation"} {
		if !strings.Contains(out, want) {
			t.Fatalf("audio failure output missing %q in:\n%s", want, out)
		}
	}
	raw, err := os.ReadFile(filepath.Join(project, "review", "review-queue.json"))
	if err != nil {
		t.Fatalf("expected review queue to be written: %v", err)
	}
	if !strings.Contains(string(raw), "audio_loudness_range") {
		t.Fatalf("review queue missing audio item:\n%s", string(raw))
	}
}

func TestVerifySupersChecksBrandSpeakerAndSafeMargins(t *testing.T) {
	project := t.TempDir()
	writeAssistFixture(t, project)
	writeTestJSONFile(t, filepath.Join(project, "calibration", "speaker-map.json"), map[string]any{
		"version": "vflow-speaker-map/v1",
		"status":  "mapped",
		"map":     map[string]string{"Speaker 1": "speaker_ali_medium"},
		"people":  map[string]any{"Speaker 1": map[string]any{"display_name": "Imam Ali", "title": "Executive Director"}},
	})
	specPath := filepath.Join(project, "decisions", "super-cards.json")
	writeTestJSONFile(t, specPath, map[string]any{
		"version":         "vflow-super-cards/v1",
		"brand_ref":       "brand.json",
		"speaker_map_ref": "calibration/speaker-map.json",
		"items": []map[string]any{{
			"id": "super_001", "layout_id": "lower.third.default", "text": "Imam Ally", "speaker_label": "Speaker 1", "safe_margin_token": "unsafe", "start_frame": 0, "end_frame": 30,
		}},
	})

	out, errOut, code := runCLI(t, "verify", "supers", "--project", project, "--spec", specPath, "--format", "json", "--format-error", "json")
	if code != 0 {
		t.Fatalf("expected reviewable success, got %d stderr=%s", code, errOut)
	}
	for _, want := range []string{`"status": "failed"`, "super-card_spelling", "super-card_safe_margin"} {
		if !strings.Contains(out, want) {
			t.Fatalf("supers verification missing %q in:\n%s", want, out)
		}
	}
}

func TestVerifyMotionChecksApprovedPresetsAndFrameDiffReport(t *testing.T) {
	project := t.TempDir()
	writeAssistFixture(t, project)
	writeTestJSONFile(t, filepath.Join(project, "calibration", "framing-presets.json"), map[string]any{
		"version":       "vflow-framing-presets/v1",
		"source_width":  3840,
		"source_height": 2160,
		"target_aspect": "16:9",
		"presets": []map[string]any{
			{"id": "wide", "label": "Wide", "type": "wide", "crop_px": map[string]any{"x": 0, "y": 0, "w": 3840, "h": 2160}},
			{"id": "speaker_ali_medium", "label": "Ali medium", "type": "speaker", "crop_px": map[string]any{"x": 960, "y": 0, "w": 1920, "h": 1080}},
		},
	})
	specPath := filepath.Join(project, "decisions", "motion-ramp.json")
	writeTestJSONFile(t, specPath, map[string]any{
		"version": "vflow-motion-ramp/v1", "from_preset_id": "wide", "to_preset_id": "missing", "start_frame": 0, "end_frame": 30, "ease": "linear",
	})
	reportPath := filepath.Join(project, "reports", "motion-diff.json")
	writeTestJSONFile(t, reportPath, map[string]any{"frame_diff_confirmed": false, "changed_pixel_ratio": 0.0})

	out, errOut, code := runCLI(t, "verify", "motion", "--project", project, "--spec", specPath, "--output", reportPath, "--format", "json", "--format-error", "json")
	if code != 0 {
		t.Fatalf("expected reviewable success, got %d stderr=%s", code, errOut)
	}
	for _, want := range []string{`"status": "failed"`, "motion_preset_tokens", "motion_frozen_frame"} {
		if !strings.Contains(out, want) {
			t.Fatalf("motion verification missing %q in:\n%s", want, out)
		}
	}
}

func TestBrandSchemaIsPublished(t *testing.T) {
	out, errOut, code := runCLI(t, "schema", "--validate", "--format", "json", "--format-error", "json")
	if code != 0 {
		t.Fatalf("expected success, got %d stderr=%s", code, errOut)
	}
	for _, want := range []string{"brand.schema.json", "caption-cues.schema.json", "audio-intent.schema.json", "super-cards.schema.json", "motion-ramp.schema.json"} {
		if !strings.Contains(out, want) {
			t.Fatalf("schema output missing %q in:\n%s", want, out)
		}
	}
}

func writeAssistFixture(t *testing.T, project string) {
	t.Helper()
	writeTestJSONFile(t, filepath.Join(project, "brand.json"), map[string]any{
		"version":            "vflow-brand/v1",
		"colors":             map[string]string{"primary": "#00467F", "accent": "#F89828"},
		"fonts":              map[string]string{"caption": "Inter"},
		"caption_styles":     map[string]any{"caption.default": map[string]any{"font": "Inter", "color": "primary"}},
		"layout_ids":         []string{"lower.third.default", "card.quote"},
		"loudness_targets":   map[string]any{"integrated_lufs": -16, "true_peak_db": -1},
		"safe_margins":       map[string]any{"title": 0.9, "action": 0.95},
		"consistency_tokens": map[string]string{"org": "CAIR-Georgia"},
	})
	writeTestJSONFile(t, filepath.Join(project, "transcript", "words.json"), map[string]any{
		"version": "vflow-words/v1", "source_media_id": "src", "rate": "30/1",
		"words": []map[string]any{{"id": "w1", "text": "hello", "start_frame": 0, "end_frame": 15, "provider": "fixture"}},
	})
}

func writeTestJSONFile(t *testing.T, path string, value any) {
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
