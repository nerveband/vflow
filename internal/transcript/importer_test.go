package transcript

import (
	"os"
	"strings"
	"testing"
)

func TestImportGenericWords(t *testing.T) {
	raw, err := os.ReadFile("../../fixtures/project/basic/transcript/words.json")
	if err != nil {
		t.Fatal(err)
	}
	words, err := Import("generic-words", raw, ImportOptions{Rate: "30000/1001"})
	if err != nil {
		t.Fatal(err)
	}
	if len(words.Words) != 2 {
		t.Fatalf("expected 2 words, got %d", len(words.Words))
	}
	if words.Words[0].StartFrame != 120 {
		t.Fatalf("unexpected start frame: %d", words.Words[0].StartFrame)
	}
}

func TestImportGenericWordsRejectsInvalidFrameRange(t *testing.T) {
	raw := []byte(`{"version":"vflow-words/v1","source_media_id":"source","rate":"30000/1001","words":[{"id":"w_000001","text":"bad","start_frame":20,"end_frame":20,"confidence":0.9,"provider":"generic-words"}]}`)
	_, err := Import("generic-words", raw, ImportOptions{Rate: "30000/1001"})
	if err == nil || !strings.Contains(err.Error(), "end_frame must be greater than start_frame") {
		t.Fatalf("expected frame range validation error, got %v", err)
	}
}

func TestValidateWordsRejectsOutOfRangeConfidence(t *testing.T) {
	err := ValidateWords(Words{
		Version:       "vflow-words/v1",
		SourceMediaID: "source",
		Rate:          "30000/1001",
		Words: []Word{{
			ID:         "w_000001",
			Text:       "bad",
			StartFrame: 0,
			EndFrame:   15,
			Confidence: 1.2,
			Provider:   "test",
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "confidence must be between 0 and 1") {
		t.Fatalf("expected confidence validation error, got %v", err)
	}
}

func TestImportPlainTextProducesStableWordIDs(t *testing.T) {
	words, err := Import("plain-text", []byte("Ali: Bismillah welcome\nFatima: salaam"), ImportOptions{Rate: "30000/1001", FramesPerWord: 12})
	if err != nil {
		t.Fatal(err)
	}
	if len(words.Words) != 3 {
		t.Fatalf("expected 3 words, got %d", len(words.Words))
	}
	if words.Words[0].ID != "w_000001" || words.Words[0].SpeakerLabel != "Ali" {
		t.Fatalf("unexpected first word: %#v", words.Words[0])
	}
}
