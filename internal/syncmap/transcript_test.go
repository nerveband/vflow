package syncmap

import (
	"math"
	"testing"

	vtranscript "github.com/nerveband/vflow/internal/transcript"
)

func TestAlignTranscriptWordsEstimatesSourceOffset(t *testing.T) {
	reference := vtranscript.Words{
		Version:       "vflow-words/v1",
		SourceMediaID: "atem",
		Rate:          "30/1",
		Words: []vtranscript.Word{
			{ID: "r1", Text: "cair", StartFrame: 300, EndFrame: 315, Provider: "test"},
			{ID: "r2", Text: "georgia", StartFrame: 330, EndFrame: 345, Provider: "test"},
			{ID: "r3", Text: "served", StartFrame: 360, EndFrame: 375, Provider: "test"},
		},
	}
	source := vtranscript.Words{
		Version:       "vflow-words/v1",
		SourceMediaID: "9mm",
		Rate:          "30/1",
		Words: []vtranscript.Word{
			{ID: "s1", Text: "CAIR", StartFrame: 4890, EndFrame: 4905, Provider: "test"},
			{ID: "s2", Text: "Georgia", StartFrame: 4920, EndFrame: 4935, Provider: "test"},
			{ID: "s3", Text: "served", StartFrame: 4950, EndFrame: 4965, Provider: "test"},
		},
	}

	result, err := AlignTranscriptWords(reference, source)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(result.OffsetSeconds-153) > 0.001 {
		t.Fatalf("offset = %.3f, want 153.000", result.OffsetSeconds)
	}
	if result.MatchCount != 3 || result.MaxResidualSeconds > 0.001 || result.Confidence < 0.99 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestAlignTranscriptWordsReportsLowConfidenceForSparseMatches(t *testing.T) {
	reference := vtranscript.Words{Version: "vflow-words/v1", SourceMediaID: "atem", Rate: "30/1", Words: []vtranscript.Word{{ID: "r1", Text: "unique", StartFrame: 0, EndFrame: 1, Provider: "test"}}}
	source := vtranscript.Words{Version: "vflow-words/v1", SourceMediaID: "9mm", Rate: "30/1", Words: []vtranscript.Word{{ID: "s1", Text: "unique", StartFrame: 300, EndFrame: 301, Provider: "test"}}}

	result, err := AlignTranscriptWords(reference, source)
	if err != nil {
		t.Fatal(err)
	}
	if result.MatchCount != 1 || result.Confidence >= 0.65 {
		t.Fatalf("expected low-confidence single match, got %+v", result)
	}
}
