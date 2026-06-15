package color

import (
	"slices"
	"testing"
)

func TestLUTApplyPlanUsesDeliverySafeCodecSettings(t *testing.T) {
	plan := LUTApplyPlan("input.mp4", "look.cube", "graded.mp4")
	want := []string{
		"-map", "0:v:0",
		"-map", "0:a?",
		"-dn",
		"-map_metadata", "-1",
		"-map_chapters", "-1",
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-movflags", "+faststart",
	}
	for _, token := range want {
		if !slices.Contains(plan.Command, token) {
			t.Fatalf("expected command to contain %q: %#v", token, plan.Command)
		}
	}
}
