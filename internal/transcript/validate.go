package transcript

import (
	"fmt"
	"strings"
)

func ValidateWords(words Words) error {
	if words.Version != "vflow-words/v1" {
		return fmt.Errorf("transcript version must be vflow-words/v1")
	}
	if strings.TrimSpace(words.SourceMediaID) == "" {
		return fmt.Errorf("transcript source_media_id is required")
	}
	if strings.TrimSpace(words.Rate) == "" {
		return fmt.Errorf("transcript rate is required")
	}
	for i, word := range words.Words {
		if strings.TrimSpace(word.ID) == "" {
			return fmt.Errorf("word %d id is required", i)
		}
		if strings.TrimSpace(word.Text) == "" {
			return fmt.Errorf("word %s text is required", word.ID)
		}
		if strings.TrimSpace(word.Provider) == "" {
			return fmt.Errorf("word %s provider is required", word.ID)
		}
		if word.StartFrame < 0 {
			return fmt.Errorf("word %s start_frame must be >= 0", word.ID)
		}
		if word.EndFrame <= word.StartFrame {
			return fmt.Errorf("word %s end_frame must be greater than start_frame", word.ID)
		}
		if word.Confidence < 0 || word.Confidence > 1 {
			return fmt.Errorf("word %s confidence must be between 0 and 1", word.ID)
		}
	}
	return nil
}
