package nle

import "testing"

func TestExportSidecarMapsSourceAndTimelineFrames(t *testing.T) {
	result := Export(Options{Target: "edl", Output: "timeline.edl"}, []Segment{{ID: "seg_A", SourceFrameIn: 0, SourceFrameOut: 90, TimelineFrameIn: 0, TimelineFrameOut: 90}})
	if len(result.Sidecar.Segments) != 1 {
		t.Fatalf("expected one sidecar segment")
	}
	if result.Sidecar.Segments[0].SourceFrameOut != 90 {
		t.Fatalf("unexpected source frame out: %d", result.Sidecar.Segments[0].SourceFrameOut)
	}
	if result.Target != "edl" {
		t.Fatalf("unexpected target: %s", result.Target)
	}
}
