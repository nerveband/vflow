package timeline

import (
	"testing"

	"github.com/nerveband/vflow/internal/cleanup"
)

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

func TestCompileMapsSegmentOutBoundaryAtDeleteStart(t *testing.T) {
	tl := Compile(contentEDLForTest(30, 45), 90)
	if len(tl.Segments) != 2 {
		t.Fatalf("expected 2 kept segments, got %d", len(tl.Segments))
	}
	first := tl.Segments[0]
	if first.SourceFrameIn != 0 || first.SourceFrameOut != 30 || first.TimelineFrameIn != 0 || first.TimelineFrameOut != 30 {
		t.Fatalf("unexpected first segment: %+v", first)
	}
	second := tl.Segments[1]
	if second.SourceFrameIn != 45 || second.SourceFrameOut != 90 || second.TimelineFrameIn != 30 || second.TimelineFrameOut != 75 {
		t.Fatalf("unexpected second segment: %+v", second)
	}
}

func contentEDLForTest(start, end int) cleanup.ContentEDL {
	return cleanup.ContentEDL{
		Version: "vflow-content-edl/v1",
		Rate:    "30/1",
		DeleteSegments: []cleanup.DeleteSegment{
			{ID: "del_000001", StartFrame: start, EndFrame: end, Reason: "test", Confidence: 1},
		},
	}
}
