package timeline

import "testing"

func TestSourceFrameAnchoringAfterDeletes(t *testing.T) {
	m := BuildTimeMap([]DeleteRange{{StartFrame: 100, EndFrame: 200}, {StartFrame: 400, EndFrame: 450}}, 600)
	got, ok := m.SourceToTimeline(250)
	if !ok {
		t.Fatalf("source frame 250 should remain")
	}
	if got != 150 {
		t.Fatalf("expected source 250 -> timeline 150, got %d", got)
	}
	if _, ok := m.SourceToTimeline(150); ok {
		t.Fatalf("deleted source frame 150 should not map")
	}
}
