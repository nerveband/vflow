package framing

import (
	"slices"
	"testing"

	"github.com/nerveband/vflow/internal/transcript"
)

func TestCompileLaneUsesSpeakerMapAndQueuesContractExceptions(t *testing.T) {
	presets := Presets{
		Version:      "vflow-framing-presets/v1",
		SourceWidth:  3840,
		SourceHeight: 2160,
		TargetAspect: "16:9",
		Presets: []Preset{
			{ID: "wide", Label: "Wide", Type: "wide", CropPX: Rect{X: 0, Y: 0, W: 3840, H: 2160}},
			{ID: "ali_medium", Label: "Ali medium", Type: "speaker", CropPX: Rect{X: 960, Y: 0, W: 1920, H: 1080}},
		},
	}
	words := transcript.Words{
		Version:       "vflow-words/v1",
		SourceMediaID: "source_cam_a",
		Rate:          "30/1",
		Words: []transcript.Word{
			{ID: "w_000001", Text: "mapped", SpeakerLabel: "SPEAKER_00", StartFrame: 10, EndFrame: 50, Confidence: 0.98},
			{ID: "w_000002", Text: "low", SpeakerLabel: "SPEAKER_00", StartFrame: 80, EndFrame: 90, Confidence: 0.42},
			{ID: "w_000003", Text: "unmapped", SpeakerLabel: "SPEAKER_01", StartFrame: 120, EndFrame: 170, Confidence: 0.91},
		},
	}

	compiled, err := CompileLane(CompileInput{
		Presets: presets,
		SpeakerMap: SpeakerMap{
			Version: "vflow-speaker-map/v1",
			Map:     map[string]string{"SPEAKER_00": "ali_medium"},
		},
		Policy: Policy{
			Version:                "vflow-framing-policy/v1",
			MinDwellFrames:         20,
			LowConfidenceThreshold: 0.70,
			WidePresetID:           "wide",
			WideResetFrames:        20,
		},
		Words: words,
	})
	if err != nil {
		t.Fatalf("CompileLane returned error: %v", err)
	}

	if len(compiled.Lane.Events) != 5 {
		t.Fatalf("expected mapped event, two wide resets, and two review fallbacks, got %#v", compiled.Lane.Events)
	}
	first := compiled.Lane.Events[0]
	if first.PresetID != "ali_medium" || first.StartFrame != 10 || first.EndFrame != 50 {
		t.Fatalf("unexpected mapped event: %#v", first)
	}
	if first.StartSeconds != 0.333333 || first.EndSeconds != 1.666667 {
		t.Fatalf("expected readable seconds derived from frames, got %#v", first)
	}
	if !slices.Equal(first.Provenance.SourceWordIDs, []string{"w_000001"}) {
		t.Fatalf("missing source word provenance: %#v", first.Provenance)
	}

	lowConfidence := compiled.Lane.Events[2]
	if lowConfidence.PresetID != "wide" || !slices.Contains(lowConfidence.ReviewReasons, ReviewLowConfidence) || !slices.Contains(lowConfidence.ReviewReasons, ReviewMinDwell) {
		t.Fatalf("expected low-confidence/min-dwell wide fallback, got %#v", lowConfidence)
	}
	unmapped := compiled.Lane.Events[4]
	if unmapped.PresetID != "wide" || !slices.Contains(unmapped.ReviewReasons, ReviewUnmappedSpeaker) {
		t.Fatalf("expected unmapped speaker wide fallback, got %#v", unmapped)
	}
	if len(compiled.ReviewQueue.Items) != 5 {
		t.Fatalf("expected five review items, got %#v", compiled.ReviewQueue.Items)
	}
	if !slices.ContainsFunc(compiled.ReviewQueue.Items, func(item ReviewItem) bool { return item.Code == ReviewMinDwell }) {
		t.Fatalf("expected min-dwell review item, got %#v", compiled.ReviewQueue.Items)
	}
	resetCount := 0
	for _, item := range compiled.ReviewQueue.Items {
		if item.Code == ReviewWideReset {
			resetCount++
		}
	}
	if resetCount != 2 {
		t.Fatalf("expected two wide-reset review items, got %#v", compiled.ReviewQueue.Items)
	}
}

func TestCompileLaneFallsBackToWideForOverlappingSpeakers(t *testing.T) {
	presets := Presets{
		Version:      "vflow-framing-presets/v1",
		SourceWidth:  3840,
		SourceHeight: 2160,
		TargetAspect: "16:9",
		Presets: []Preset{
			{ID: "wide", Label: "Wide", Type: "wide", CropPX: Rect{X: 0, Y: 0, W: 3840, H: 2160}},
			{ID: "ali_medium", Label: "Ali medium", Type: "speaker", CropPX: Rect{X: 960, Y: 0, W: 1920, H: 1080}},
			{ID: "fatima_medium", Label: "Fatima medium", Type: "speaker", CropPX: Rect{X: 0, Y: 0, W: 1920, H: 1080}},
		},
	}
	words := transcript.Words{
		Version:       "vflow-words/v1",
		SourceMediaID: "source",
		Rate:          "30/1",
		Words: []transcript.Word{
			{ID: "w_000001", Text: "one", SpeakerLabel: "SPEAKER_00", StartFrame: 10, EndFrame: 40, Confidence: 0.99},
			{ID: "w_000002", Text: "two", SpeakerLabel: "SPEAKER_01", StartFrame: 30, EndFrame: 60, Confidence: 0.99},
		},
	}

	compiled, err := CompileLane(CompileInput{
		Presets:    presets,
		SpeakerMap: SpeakerMap{Map: map[string]string{"SPEAKER_00": "ali_medium", "SPEAKER_01": "fatima_medium"}},
		Policy:     DefaultPolicy(),
		Words:      words,
	})
	if err != nil {
		t.Fatalf("CompileLane returned error: %v", err)
	}
	if len(compiled.Lane.Events) != 1 {
		t.Fatalf("expected one overlap fallback event, got %#v", compiled.Lane.Events)
	}
	got := compiled.Lane.Events[0]
	if got.PresetID != "wide" || got.StartFrame != 10 || got.EndFrame != 60 || !slices.Contains(got.ReviewReasons, ReviewOverlapWideFallback) {
		t.Fatalf("unexpected overlap fallback: %#v", got)
	}
	if len(compiled.ReviewQueue.Items) != 1 || compiled.ReviewQueue.Items[0].Code != ReviewOverlapWideFallback {
		t.Fatalf("expected overlap review item, got %#v", compiled.ReviewQueue.Items)
	}
}
