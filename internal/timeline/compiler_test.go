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

func TestCompileProducesCanonicalTimelineV1(t *testing.T) {
	tl := Compile(contentEDLForTest(30, 45), 90)

	if tl.Canonical.Version != "vflow-timeline/v1" {
		t.Fatalf("unexpected canonical version: %+v", tl.Canonical)
	}
	if tl.Canonical.FPS != "30/1" {
		t.Fatalf("expected default fps, got %q", tl.Canonical.FPS)
	}
	if len(tl.Canonical.Tracks) != 2 {
		t.Fatalf("expected linked video/audio tracks, got %+v", tl.Canonical.Tracks)
	}
	clip := tl.Canonical.Tracks[0].Clips[0]
	if clip.ID == "" || clip.StableClipID == "" || clip.LinkedClipID == "" {
		t.Fatalf("canonical clip should carry stable and linked IDs: %+v", clip)
	}
	if clip.SourceRange.StartFrame != 0 || clip.TimelineRange.EndFrame != 30 {
		t.Fatalf("canonical clip should preserve source/timeline ranges: %+v", clip)
	}
	if len(tl.Canonical.MulticamGroups) != 0 || len(tl.Canonical.ActiveAngleSpans) != 0 {
		t.Fatalf("plain compile should not invent multicam metadata: %+v", tl.Canonical)
	}
}
