package render

import (
	"strings"
	"testing"
)

func TestPreviewPlanUsesTrimConcatAndAudioFades(t *testing.T) {
	plan := PreviewPlan(Options{Input: "source.mp4", Output: "rough.mp4", Target: "youtube_16x9", MaxSeconds: 2})
	joined := strings.Join(plan.Command, " ")
	for _, want := range []string{"ffmpeg", "-t", "2", "scale=1920:1080", "afade=t=in:st=0:d=0.03", "rough.mp4"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("plan missing %q in %s", want, joined)
		}
	}
}

func TestPreviewPlanFadeOutTracksDuration(t *testing.T) {
	plan := PreviewPlan(Options{Input: "source.mp4", Output: "rough.mp4", Target: "youtube_16x9", MaxSeconds: 1})
	joined := strings.Join(plan.Command, " ")
	if !strings.Contains(joined, "afade=t=out:st=0.97:d=0.03") {
		t.Fatalf("expected one-second fade-out start in %s", joined)
	}
}

func TestPreviewPlanSupportsStartOffset(t *testing.T) {
	plan := PreviewPlan(Options{Input: "source.mp4", Output: "rough.mp4", Target: "youtube_16x9", MaxSeconds: 30, StartSeconds: 12.5})
	joined := strings.Join(plan.Command, " ")
	for _, want := range []string{"-ss 12.500", "-i source.mp4", "-t 30"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("plan missing %q in %s", want, joined)
		}
	}
}
