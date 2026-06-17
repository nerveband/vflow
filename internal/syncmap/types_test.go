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

func TestConfidenceWarningsFlagIdenticalOffsetsAcrossDistinctSources(t *testing.T) {
	m := SyncMap{
		Version:           Version,
		ReferenceSourceID: "atem",
		Sources: []SourceSync{
			{ID: "atem", Path: "atem.mp4", Confidence: 1},
			{ID: "9mm", Path: "9mm.mp4", OffsetFromReferenceSeconds: -44.88, Confidence: 0.51},
			{ID: "12mm", Path: "12mm.mp4", OffsetFromReferenceSeconds: -44.88, Confidence: 0.52},
		},
	}
	warnings := m.ConfidenceWarnings()
	found := false
	for _, warning := range warnings {
		if warning == "sources 9mm and 12mm have identical offset -44.880s; review sync before cutting" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing identical-offset warning: %#v", warnings)
	}
}

func TestValidateRejectsLowConfidenceBelowAgentSafeThreshold(t *testing.T) {
	m := SyncMap{
		Version:           Version,
		ReferenceSourceID: "atem",
		Sources: []SourceSync{
			{ID: "atem", Path: "atem.mp4", Confidence: 1},
			{ID: "9mm", Path: "9mm.mp4", OffsetFromReferenceSeconds: 12, Confidence: 0.51},
		},
	}
	errs := m.Validate(ValidationOptions{MinConfidence: 0.65})
	found := false
	for _, err := range errs {
		if err == "sources[1].confidence below 0.65 requires allow-low-confidence" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing min confidence validation: %#v", errs)
	}
}
