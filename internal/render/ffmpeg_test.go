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

func TestPreviewPlanStripsSourceTimecodeAndDataStreams(t *testing.T) {
	plan := PreviewPlan(Options{Input: "source.mp4", Output: "rough.mp4", Target: "youtube_16x9", MaxSeconds: 30})
	joined := strings.Join(plan.Command, " ")
	for _, want := range []string{"-map 0:v:0", "-map 0:a?", "-dn", "-map_metadata -1", "-map_chapters -1", "-movflags +faststart"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("plan missing %q in %s", want, joined)
		}
	}
}

func TestTranscriptCutPlanConcatsTranscriptSelectedSegments(t *testing.T) {
	edit := TranscriptCut{
		Version: "vflow-transcript-cut/v1",
		Segments: []TranscriptCutSegment{
			{ID: "hook", Source: "wide.mp4", StartSeconds: 120, EndSeconds: 128, Text: "10-year video"},
			{ID: "legacy", Source: "wide.mp4", StartSeconds: 173, EndSeconds: 185, Text: "legacy"},
		},
	}
	plan, err := TranscriptCutPlan(edit, "social.mp4", "youtube_16x9")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(plan.Command, " ")
	for _, want := range []string{
		"-ss 120.000 -t 8.000 -i wide.mp4",
		"-ss 173.000 -t 12.000 -i wide.mp4",
		"concat=n=2:v=1:a=1",
		"-map [v]",
		"-map [a]",
		"-map_metadata -1",
		"-dn",
		"social.mp4",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("plan missing %q in %s", want, joined)
		}
	}
	if plan.DurationSeconds != 20 {
		t.Fatalf("duration = %v, want 20", plan.DurationSeconds)
	}
}
