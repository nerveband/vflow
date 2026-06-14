package transcript

import (
	"os"
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
