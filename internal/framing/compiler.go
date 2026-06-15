package framing

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"

	"github.com/nerveband/vflow/internal/transcript"
)

const (
	ReviewUnmappedSpeaker     = "unmapped_speaker"
	ReviewLowConfidence       = "low_confidence"
	ReviewMinDwell            = "min_dwell"
	ReviewOverlapWideFallback = "overlap_wide_fallback"
	ReviewWideReset           = "wide_reset"
)

type SpeakerMap struct {
	Version      string            `json:"version"`
	Status       string            `json:"status,omitempty"`
	SpeakerCount int               `json:"speaker_count,omitempty"`
	Map          map[string]string `json:"map"`
}

type CompileInput struct {
	Presets    Presets
	SpeakerMap SpeakerMap
	Policy     Policy
	Words      transcript.Words
}

type Compiled struct {
	Lane        Lane        `json:"lane"`
	ReviewQueue ReviewQueue `json:"review_queue"`
}

type Lane struct {
	Version    string  `json:"version"`
	Compiled   bool    `json:"compiled"`
	SourceRate string  `json:"source_rate,omitempty"`
	Events     []Event `json:"events"`
}

type Event struct {
	ID             string          `json:"id"`
	StartFrame     int64           `json:"start_frame"`
	EndFrame       int64           `json:"end_frame"`
	StartSeconds   float64         `json:"start_seconds,omitempty"`
	EndSeconds     float64         `json:"end_seconds,omitempty"`
	PresetID       string          `json:"preset_id"`
	SpeakerLabel   string          `json:"speaker_label,omitempty"`
	Reason         string          `json:"reason"`
	ReviewRequired bool            `json:"review_required,omitempty"`
	ReviewReasons  []string        `json:"review_reasons,omitempty"`
	Provenance     EventProvenance `json:"provenance"`
}

type EventProvenance struct {
	SourceMediaID  string   `json:"source_media_id,omitempty"`
	SourceWordIDs  []string `json:"source_word_ids,omitempty"`
	SourceFrameIn  int64    `json:"source_frame_in"`
	SourceFrameOut int64    `json:"source_frame_out"`
}

type ReviewQueue struct {
	Version string       `json:"version"`
	Items   []ReviewItem `json:"items"`
}

type ReviewItem struct {
	ID           string `json:"id"`
	Code         string `json:"code"`
	Severity     string `json:"severity"`
	Message      string `json:"message"`
	EventID      string `json:"event_id"`
	SpeakerLabel string `json:"speaker_label,omitempty"`
	StartFrame   int64  `json:"start_frame"`
	EndFrame     int64  `json:"end_frame"`
	PresetID     string `json:"preset_id"`
}

func CompileLane(input CompileInput) (Compiled, error) {
	if err := input.Presets.Validate(); err != nil {
		return Compiled{}, err
	}
	policy := input.Policy.withDefaults()
	presetIDs := input.Presets.IDSet()
	if !presetIDs[policy.WidePresetID] {
		return Compiled{}, fmt.Errorf("wide preset %s is not defined", policy.WidePresetID)
	}
	if err := input.SpeakerMap.Validate(presetIDs); err != nil {
		return Compiled{}, err
	}

	words := append([]transcript.Word(nil), input.Words.Words...)
	sort.SliceStable(words, func(i, j int) bool {
		if words[i].StartFrame == words[j].StartFrame {
			return words[i].EndFrame < words[j].EndFrame
		}
		return words[i].StartFrame < words[j].StartFrame
	})

	rate := parseRate(input.Words.Rate)
	compiled := Compiled{
		Lane: Lane{
			Version:    "vflow-framing-lane/v1",
			Compiled:   true,
			SourceRate: input.Words.Rate,
			Events:     []Event{},
		},
		ReviewQueue: ReviewQueue{Version: "vflow-review-queue/v1", Items: []ReviewItem{}},
	}

	for i := 0; i < len(words); i++ {
		word := words[i]
		if word.EndFrame <= word.StartFrame {
			return Compiled{}, fmt.Errorf("word %s has invalid frame range", word.ID)
		}
		if len(compiled.Lane.Events) > 0 {
			last := compiled.Lane.Events[len(compiled.Lane.Events)-1]
			if word.StartFrame >= last.EndFrame && word.StartFrame-last.EndFrame >= policy.WideResetFrames {
				event := newEvent(len(compiled.Lane.Events)+1, last.EndFrame, word.StartFrame, policy.WidePresetID, "", "wide reset between speaker decisions", []string{ReviewWideReset}, input.Words.SourceMediaID, nil, rate)
				compiled.Lane.Events = append(compiled.Lane.Events, event)
				compiled.addReview(event, ReviewWideReset, reviewMessage(ReviewWideReset))
			}
		}

		if i+1 < len(words) && words[i+1].StartFrame < word.EndFrame && words[i+1].SpeakerLabel != word.SpeakerLabel {
			next := words[i+1]
			start := min64(word.StartFrame, next.StartFrame)
			end := max64(word.EndFrame, next.EndFrame)
			wordIDs := compactWordIDs(word.ID, next.ID)
			event := newEvent(len(compiled.Lane.Events)+1, start, end, policy.WidePresetID, "", "overlapping speakers require wide fallback", []string{ReviewOverlapWideFallback}, input.Words.SourceMediaID, wordIDs, rate)
			compiled.Lane.Events = append(compiled.Lane.Events, event)
			compiled.addReview(event, ReviewOverlapWideFallback, "Overlapping speaker turns require human review before using a speaker crop.")
			i++
			continue
		}

		presetID, ok := input.SpeakerMap.Map[word.SpeakerLabel]
		reasons := []string{}
		reason := "speaker map decision"
		if !ok || presetID == "" {
			presetID = policy.WidePresetID
			reasons = append(reasons, ReviewUnmappedSpeaker)
			reason = "unmapped speaker wide fallback"
		}
		if word.Confidence > 0 && word.Confidence < policy.LowConfidenceThreshold {
			presetID = policy.WidePresetID
			reasons = appendReason(reasons, ReviewLowConfidence)
			reason = "low confidence wide fallback"
		}
		if word.EndFrame-word.StartFrame < policy.MinDwellFrames {
			presetID = policy.WidePresetID
			reasons = appendReason(reasons, ReviewMinDwell)
			if reason == "speaker map decision" {
				reason = "minimum dwell wide fallback"
			}
		}
		event := newEvent(len(compiled.Lane.Events)+1, word.StartFrame, word.EndFrame, presetID, word.SpeakerLabel, reason, reasons, input.Words.SourceMediaID, compactWordIDs(word.ID), rate)
		compiled.Lane.Events = append(compiled.Lane.Events, event)
		for _, reviewReason := range reviewCodes(reasons) {
			compiled.addReview(event, reviewReason, reviewMessage(reviewReason))
		}
	}
	return compiled, nil
}

func (s SpeakerMap) Validate(presetIDs map[string]bool) error {
	for speaker, presetID := range s.Map {
		if speaker == "" {
			return fmt.Errorf("speaker-map contains empty speaker label")
		}
		if presetID == "" {
			return fmt.Errorf("speaker-map entry %s has empty preset id", speaker)
		}
		if !presetIDs[presetID] {
			return fmt.Errorf("speaker-map entry %s references unknown preset %s", speaker, presetID)
		}
	}
	return nil
}

func ReadSpeakerMap(projectPath string) (SpeakerMap, error) {
	raw, err := os.ReadFile(filepath.Join(projectPath, "calibration", "speaker-map.json"))
	if err != nil {
		return SpeakerMap{}, err
	}
	var speakerMap SpeakerMap
	if err := json.Unmarshal(raw, &speakerMap); err != nil {
		return SpeakerMap{}, err
	}
	if speakerMap.Map == nil {
		speakerMap.Map = map[string]string{}
	}
	return speakerMap, nil
}

func ReadPolicy(projectPath string) (Policy, error) {
	raw, err := os.ReadFile(filepath.Join(projectPath, "policy", "framing-policy.json"))
	if os.IsNotExist(err) {
		return DefaultPolicy(), nil
	}
	if err != nil {
		return Policy{}, err
	}
	var policy Policy
	if err := json.Unmarshal(raw, &policy); err != nil {
		return Policy{}, err
	}
	return policy.withDefaults(), nil
}

func WriteCompiled(projectPath string, compiled Compiled) error {
	lanePath := filepath.Join(projectPath, "decisions", "framing-lane.json")
	reviewPath := filepath.Join(projectPath, "review", "review-queue.json")
	if err := os.MkdirAll(filepath.Dir(lanePath), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(reviewPath), 0o755); err != nil {
		return err
	}
	laneRaw, err := json.MarshalIndent(compiled.Lane, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(lanePath, append(laneRaw, '\n'), 0o644); err != nil {
		return err
	}
	reviewRaw, err := json.MarshalIndent(compiled.ReviewQueue, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(reviewPath, append(reviewRaw, '\n'), 0o644)
}

func (c *Compiled) addReview(event Event, code, message string) {
	c.ReviewQueue.Items = append(c.ReviewQueue.Items, ReviewItem{
		ID:           fmt.Sprintf("rev_%06d", len(c.ReviewQueue.Items)+1),
		Code:         code,
		Severity:     "needs_human_review",
		Message:      message,
		EventID:      event.ID,
		SpeakerLabel: event.SpeakerLabel,
		StartFrame:   event.StartFrame,
		EndFrame:     event.EndFrame,
		PresetID:     event.PresetID,
	})
}

func newEvent(n int, start, end int64, presetID, speakerLabel, reason string, reviewReasons []string, sourceMediaID string, wordIDs []string, rate float64) Event {
	return Event{
		ID:             fmt.Sprintf("fr_%06d", n),
		StartFrame:     start,
		EndFrame:       end,
		StartSeconds:   frameSeconds(start, rate),
		EndSeconds:     frameSeconds(end, rate),
		PresetID:       presetID,
		SpeakerLabel:   speakerLabel,
		Reason:         reason,
		ReviewRequired: len(reviewReasons) > 0,
		ReviewReasons:  reviewReasons,
		Provenance: EventProvenance{
			SourceMediaID:  sourceMediaID,
			SourceWordIDs:  wordIDs,
			SourceFrameIn:  start,
			SourceFrameOut: end,
		},
	}
}

func reviewMessage(code string) string {
	switch code {
	case ReviewUnmappedSpeaker:
		return "Speaker label is not mapped to a stable approved preset."
	case ReviewLowConfidence:
		return "Transcript confidence is below framing policy threshold."
	case ReviewMinDwell:
		return "Speaker decision is shorter than the minimum dwell policy."
	case ReviewWideReset:
		return "Policy inserted a wide reset; review the reset boundary before final delivery."
	default:
		return "Framing decision requires human review."
	}
}

func reviewCodes(reasons []string) []string {
	out := []string{}
	for _, reason := range reasons {
		if reason == ReviewLowConfidence || reason == ReviewUnmappedSpeaker || reason == ReviewOverlapWideFallback || reason == ReviewMinDwell || reason == ReviewWideReset {
			out = append(out, reason)
		}
	}
	return out
}

func appendReason(reasons []string, reason string) []string {
	for _, existing := range reasons {
		if existing == reason {
			return reasons
		}
	}
	return append(reasons, reason)
}

func compactWordIDs(ids ...string) []string {
	out := []string{}
	for _, id := range ids {
		if id != "" {
			out = append(out, id)
		}
	}
	return out
}

func parseRate(rate string) float64 {
	var numerator, denominator float64
	if _, err := fmt.Sscanf(rate, "%f/%f", &numerator, &denominator); err == nil && denominator != 0 {
		return numerator / denominator
	}
	if _, err := fmt.Sscanf(rate, "%f", &numerator); err == nil && numerator > 0 {
		return numerator
	}
	return 30
}

func frameSeconds(frame int64, rate float64) float64 {
	if rate <= 0 {
		return 0
	}
	return math.Round((float64(frame)/rate)*1_000_000) / 1_000_000
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
