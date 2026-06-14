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
