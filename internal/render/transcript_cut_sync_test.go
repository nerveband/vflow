package render

import (
	"testing"

	"github.com/nerveband/vflow/internal/syncmap"
)

func TestResolveTranscriptCutWithSyncMap(t *testing.T) {
	cut := TranscriptCut{Version: "vflow-transcript-cut/v1", Segments: []TranscriptCutSegment{{
		ID: "seg1", SourceID: "7mm", TranscriptStartSeconds: 2059, TranscriptEndSeconds: 2064,
	}}}
	m := syncmap.SyncMap{
		Version: syncmap.Version, ReferenceSourceID: "12mm", TranscriptToReferenceOffsetSeconds: 356,
		Sources: []syncmap.SourceSync{{ID: "7mm", Path: "7.mp4", OffsetFromReferenceSeconds: 17, Confidence: 0.9}},
	}
	got, err := ResolveTranscriptCutWithSyncMap(cut, m)
	if err != nil {
		t.Fatal(err)
	}
	seg := got.Segments[0]
	if seg.Source != "7.mp4" || seg.StartSeconds != 2432 || seg.EndSeconds != 2437 {
		t.Fatalf("resolved segment = %+v", seg)
	}
	if seg.ReferenceStartSeconds != 2415 {
		t.Fatalf("reference start = %.3f, want 2415", seg.ReferenceStartSeconds)
	}
}
