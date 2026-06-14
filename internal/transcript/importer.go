package transcript

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

func Import(provider string, raw []byte, opts ImportOptions) (Words, error) {
	if opts.SourceMediaID == "" {
		opts.SourceMediaID = "source"
	}
	if opts.Rate == "" {
		opts.Rate = "30000/1001"
	}
	if opts.FramesPerWord <= 0 {
		opts.FramesPerWord = 15
	}
	switch provider {
	case "generic-words":
		var words Words
		if err := json.Unmarshal(raw, &words); err != nil {
			return Words{}, err
		}
		if words.Version == "" {
			words.Version = "vflow-words/v1"
		}
		if words.SourceMediaID == "" {
			words.SourceMediaID = opts.SourceMediaID
		}
		if words.Rate == "" {
			words.Rate = opts.Rate
		}
		return words, nil
	case "plain-text", "local":
		return importPlainText(provider, string(raw), opts), nil
	default:
		return Words{}, fmt.Errorf("unsupported transcript importer %q", provider)
	}
}

func importPlainText(provider, text string, opts ImportOptions) Words {
	out := Words{Version: "vflow-words/v1", SourceMediaID: opts.SourceMediaID, Rate: opts.Rate}
	var frame int64
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		speaker := ""
		if before, after, ok := strings.Cut(line, ":"); ok && len(before) <= 64 {
			speaker = strings.TrimSpace(before)
			line = strings.TrimSpace(after)
		}
		for _, token := range strings.Fields(line) {
			cleaned := strings.TrimFunc(token, func(r rune) bool {
				return unicode.IsPunct(r) && r != '\'' && r != '-'
			})
			if cleaned == "" {
				continue
			}
			start := frame
			end := frame + opts.FramesPerWord
			out.Words = append(out.Words, Word{
				ID:           fmt.Sprintf("w_%06d", len(out.Words)+1),
				Text:         cleaned,
				SpeakerLabel: speaker,
				StartFrame:   start,
				EndFrame:     end,
				Confidence:   1,
				Provider:     provider,
			})
			frame = end
		}
	}
	return out
}

func WriteWords(projectPath string, words Words) error {
	raw, err := json.MarshalIndent(words, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(projectPath, "transcript", "words.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}
