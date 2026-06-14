package framing

import (
	"os"
	"testing"
)

func TestValidatePresetsRejectsDiarizationLabels(t *testing.T) {
	raw, err := os.ReadFile("../../fixtures/project/basic/calibration/framing-presets.json")
	if err != nil {
		t.Fatal(err)
	}
	presets, err := ParsePresets(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := presets.Validate(); err != nil {
		t.Fatalf("valid fixture rejected: %v", err)
	}
	presets.Presets[0].ID = "SPEAKER_00"
	if err := presets.Validate(); err == nil {
		t.Fatalf("expected diarization label rejection")
	}
}
