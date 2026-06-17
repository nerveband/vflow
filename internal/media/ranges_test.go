package media

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nerveband/vflow/internal/syncmap"
)

func TestPlanSourceRangesResolvesThroughSyncMap(t *testing.T) {
	m := syncmap.SyncMap{
		Version:                            syncmap.Version,
		ReferenceSourceID:                  "12mm",
		TranscriptToReferenceOffsetSeconds: 356,
		Sources:                            []syncmap.SourceSync{{ID: "7mm", Path: "7.mp4", OffsetFromReferenceSeconds: 17, Confidence: 0.9}},
	}
	manifest, err := PlanSourceRanges(m, []TranscriptRange{{ID: "a", SourceID: "7mm", Start: 2059, End: 2064}}, "ranges", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := manifest.Ranges[0].SourceStart; got != 2432 {
		t.Fatalf("source start = %.3f, want 2432", got)
	}
	if manifest.Ranges[0].Command[0] != "ffmpeg" {
		t.Fatalf("command = %#v", manifest.Ranges[0].Command)
	}
}

func TestReadTranscriptRangesAcceptsStartEndAliases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ranges.json")
	if err := os.WriteFile(path, []byte(`{"ranges":[{"id":"hook","source_id":"9mm","start":152.9,"end":168.04,"speaker_id":"s1","reason":"opening hook"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ranges, err := ReadTranscriptRanges(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(ranges) != 1 || ranges[0].Start != 152.9 || ranges[0].End != 168.04 || ranges[0].SpeakerID != "s1" || ranges[0].Reason != "opening hook" {
		t.Fatalf("unexpected ranges: %+v", ranges)
	}
}
