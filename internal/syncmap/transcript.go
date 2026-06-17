package syncmap

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"

	vtranscript "github.com/nerveband/vflow/internal/transcript"
)

const MethodTranscriptAnchor = "transcript_anchor"

type TranscriptAlignment struct {
	OffsetSeconds      float64  `json:"offset_seconds"`
	Confidence         float64  `json:"confidence"`
	MatchCount         int      `json:"match_count"`
	MaxResidualSeconds float64  `json:"max_residual_seconds"`
	MatchedWords       []string `json:"matched_words,omitempty"`
	Warnings           []string `json:"warnings,omitempty"`
}

func AlignTranscriptWords(reference, source vtranscript.Words) (TranscriptAlignment, error) {
	refRate := wordsFrameRate(reference.Rate)
	sourceRate := wordsFrameRate(source.Rate)
	refUnique := uniqueWordFrames(reference)
	sourceUnique := uniqueWordFrames(source)
	deltas := []float64{}
	matched := []string{}
	for word, refFrame := range refUnique {
		sourceFrame, ok := sourceUnique[word]
		if !ok {
			continue
		}
		deltas = append(deltas, float64(sourceFrame)/sourceRate-float64(refFrame)/refRate)
		matched = append(matched, word)
	}
	if len(deltas) == 0 {
		return TranscriptAlignment{}, fmt.Errorf("no unique word matches between transcripts")
	}
	sort.Float64s(deltas)
	sort.Strings(matched)
	offset := median(deltas)
	maxResidual := 0.0
	for _, delta := range deltas {
		residual := math.Abs(delta - offset)
		if residual > maxResidual {
			maxResidual = residual
		}
	}
	confidence := math.Min(1, float64(len(deltas))/3)
	if maxResidual > 0.08 {
		confidence *= math.Max(0, 1-(maxResidual-0.08))
	}
	result := TranscriptAlignment{
		OffsetSeconds:      roundMillis(offset),
		Confidence:         roundMillis(confidence),
		MatchCount:         len(deltas),
		MaxResidualSeconds: roundMillis(maxResidual),
		MatchedWords:       matched,
	}
	if len(deltas) < 3 {
		result.Warnings = append(result.Warnings, "fewer than 3 unique word matches")
	}
	if maxResidual > 0.08 {
		result.Warnings = append(result.Warnings, "transcript alignment residual exceeds 0.08s")
	}
	return result, nil
}

func uniqueWordFrames(words vtranscript.Words) map[string]int64 {
	type occurrence struct {
		frame int64
		count int
	}
	seen := map[string]occurrence{}
	for _, word := range words.Words {
		key := normalizeSyncWord(word.Text)
		if key == "" {
			continue
		}
		entry := seen[key]
		if entry.count == 0 {
			entry.frame = word.StartFrame
		}
		entry.count++
		seen[key] = entry
	}
	out := map[string]int64{}
	for key, entry := range seen {
		if entry.count == 1 {
			out[key] = entry.frame
		}
	}
	return out
}

func normalizeSyncWord(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.TrimFunc(value, func(r rune) bool {
		return unicode.IsPunct(r) || unicode.IsSpace(r)
	})
}

func wordsFrameRate(rate string) float64 {
	rate = strings.TrimSpace(rate)
	if rate == "" {
		return 30000.0 / 1001.0
	}
	if before, after, ok := strings.Cut(rate, "/"); ok {
		num, nerr := parsePositiveFloat(before)
		den, derr := parsePositiveFloat(after)
		if nerr == nil && derr == nil && den > 0 {
			return num / den
		}
	}
	if val, err := parsePositiveFloat(rate); err == nil {
		return val
	}
	return 30000.0 / 1001.0
}

func parsePositiveFloat(value string) (float64, error) {
	value = strings.TrimSpace(value)
	var out float64
	_, err := fmt.Sscanf(value, "%f", &out)
	if err != nil || out <= 0 {
		return 0, fmt.Errorf("invalid positive float %q", value)
	}
	return out, nil
}

func median(values []float64) float64 {
	mid := len(values) / 2
	if len(values)%2 == 1 {
		return values[mid]
	}
	return (values[mid-1] + values[mid]) / 2
}

func roundMillis(value float64) float64 {
	return math.Round(value*1000) / 1000
}
