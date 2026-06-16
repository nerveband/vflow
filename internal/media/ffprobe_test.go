package media

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseFFProbeJSON(t *testing.T) {
	raw, err := os.ReadFile("../../fixtures/media/tiny/ffprobe.json")
	if err != nil {
		t.Fatal(err)
	}
	review, err := ParseFFProbe(raw, "media/source.mp4")
	if err != nil {
		t.Fatal(err)
	}
	if review.Width != 1920 || review.Height != 1080 {
		t.Fatalf("unexpected dimensions: %dx%d", review.Width, review.Height)
	}
	if review.FrameRate != "30000/1001" {
		t.Fatalf("unexpected frame rate: %s", review.FrameRate)
	}
	if len(review.AudioStreams) != 1 {
		t.Fatalf("expected one audio stream, got %d", len(review.AudioStreams))
	}
}

func TestValidateSourceReviewRejectsMissingDimensions(t *testing.T) {
	err := ValidateSourceReview(SourceReview{
		Version:                 "vflow-source-media-review/v1",
		Source:                  "media/source.mp4",
		DurationSeconds:         1,
		FrameRate:               "30/1",
		Timebase:                "1/30000",
		Codec:                   "h264",
		VariableFrameRateStatus: "likely_cfr",
		RepresentativeFramePlan: []string{"first_frame"},
	})
	if err == nil || err.Error() != "source dimensions must be positive" {
		t.Fatalf("expected dimension validation error, got %v", err)
	}
}

func TestWriteReviewsWritesCanonicalArtifact(t *testing.T) {
	dir := t.TempDir()
	review := SourceReview{
		Version:                 "vflow-source-media-review/v1",
		Source:                  "media/source.mp4",
		Width:                   1920,
		Height:                  1080,
		DurationSeconds:         12.5,
		FrameRate:               "30000/1001",
		Timebase:                "1/30000",
		Codec:                   "h264",
		AudioStreams:            []AudioStream{{Index: 1, Codec: "aac", Channels: 2}},
		VariableFrameRateStatus: "likely_cfr",
		RepresentativeFramePlan: []string{"first_frame"},
	}
	if err := WriteReviews(dir, []SourceReview{review}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "source-media-review.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"version": "vflow-source-media-review/v1"`, `"sources": [`, `"source": "media/source.mp4"`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("artifact missing %s in:\n%s", want, raw)
		}
	}
}
