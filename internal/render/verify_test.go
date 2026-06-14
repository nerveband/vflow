package render

import (
	"os"
	"strings"
	"testing"
)

func TestVerifyProbeJSONReportsDurationResolutionAudioAndFrames(t *testing.T) {
	raw, err := os.ReadFile("../../fixtures/media/tiny/ffprobe.json")
	if err != nil {
		t.Fatal(err)
	}
	got, err := VerifyProbe(raw, VerifyOptions{
		Render:                  "rough-preview.mp4",
		ExpectedWidth:           1920,
		ExpectedHeight:          1080,
		ExpectedDurationSeconds: 12.345,
		DurationTolerance:       0.01,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "valid" || got.Width != 1920 || got.Height != 1080 || got.FrameCount != 370 || got.AudioStreams != 1 {
		t.Fatalf("unexpected verification: %+v", got)
	}
}

func TestVerifyProbeJSONFlagsMismatchAndMissingAudio(t *testing.T) {
	raw := []byte(`{"streams":[{"codec_type":"video","codec_name":"h264","width":640,"height":360,"duration":"1.000000","nb_frames":"30"}],"format":{"duration":"1.000000"}}`)
	got, err := VerifyProbe(raw, VerifyOptions{
		Render:                  "rough-preview.mp4",
		ExpectedWidth:           1920,
		ExpectedHeight:          1080,
		ExpectedDurationSeconds: 2,
		DurationTolerance:       0.01,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "invalid" {
		t.Fatalf("expected invalid status: %+v", got)
	}
	joined := strings.Join(got.Issues, ",")
	for _, want := range []string{"resolution_mismatch", "duration_mismatch", "missing_audio"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing issue %q in %+v", want, got.Issues)
		}
	}
}
