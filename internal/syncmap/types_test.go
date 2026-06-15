package syncmap

import "testing"

func TestResolveTranscriptSecondsToSourceSeconds(t *testing.T) {
	m := SyncMap{
		Version:                            Version,
		ReferenceSourceID:                  "12mm",
		TranscriptToReferenceOffsetSeconds: 356,
		Sources: []SourceSync{
			{ID: "12mm", Path: "12.mp4", Confidence: 1},
			{ID: "7mm", Path: "7.mp4", OffsetFromReferenceSeconds: 17, Confidence: 0.91},
		},
	}
	got, err := m.SourceSeconds("7mm", 2059)
	if err != nil {
		t.Fatal(err)
	}
	if got != 2432 {
		t.Fatalf("source seconds = %v, want 2432", got)
	}
}

func TestValidateRejectsMissingReference(t *testing.T) {
	m := SyncMap{Version: Version, ReferenceSourceID: "missing", Sources: []SourceSync{{ID: "a", Path: "a.mp4", Confidence: 1}}}
	if got := m.Validate(ValidationOptions{}); len(got) == 0 {
		t.Fatal("expected validation error")
	}
}
