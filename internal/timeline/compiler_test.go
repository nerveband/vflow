package timeline

import "testing"

func TestCompileSegmentsCarryFrameProvenance(t *testing.T) {
	tl := Compile(contentEDLForTest(30, 45), 90)
	if len(tl.Segments) == 0 {
		t.Fatalf("expected compiled segments")
	}
	got := tl.Segments[0].Provenance
	if got.SourceFrameIn != 0 || got.SourceFrameOut != 30 || got.TimelineFrameIn != 0 || got.TimelineFrameOut != 30 {
		t.Fatalf("unexpected provenance: %#v", got)
	}
}
