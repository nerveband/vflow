package media

import (
	"os"
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
