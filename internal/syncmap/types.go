package syncmap

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	Version                  = "vflow-media-sync-map/v1"
	MethodAudioXcorrEnvelope = "audio_xcorr_envelope"
)

type SyncMap struct {
	Version                            string       `json:"version"`
	ProjectID                          string       `json:"project_id,omitempty"`
	CreatedAt                          string       `json:"created_at,omitempty"`
	Method                             string       `json:"method"`
	ReferenceSourceID                  string       `json:"reference_source_id"`
	TranscriptToReferenceOffsetSeconds float64      `json:"transcript_to_reference_offset_seconds"`
	FrameRate                          string       `json:"frame_rate,omitempty"`
	SampleRate                         int          `json:"sample_rate,omitempty"`
	Sources                            []SourceSync `json:"sources"`
	Anchors                            []Anchor     `json:"anchors,omitempty"`
	Warnings                           []string     `json:"warnings,omitempty"`
}

type SourceSync struct {
	ID                         string   `json:"id"`
	Path                       string   `json:"path"`
	AudioStreamIndex           int      `json:"audio_stream_index,omitempty"`
	OffsetFromReferenceSeconds float64  `json:"offset_from_reference_seconds"`
	Confidence                 float64  `json:"confidence"`
	DriftPPM                   float64  `json:"drift_ppm,omitempty"`
	Warnings                   []string `json:"warnings,omitempty"`
}

type Anchor struct {
	ID                string  `json:"id"`
	TranscriptSeconds float64 `json:"transcript_seconds"`
	ReferenceSeconds  float64 `json:"reference_seconds"`
	Method            string  `json:"method"`
	MatchedText       string  `json:"matched_text,omitempty"`
	Confidence        float64 `json:"confidence"`
}

type ValidationOptions struct {
	AllowLowConfidence bool
}

func New(projectID, referenceSourceID string, sources []SourceSync) SyncMap {
	return SyncMap{
		Version:           Version,
		ProjectID:         projectID,
		CreatedAt:         time.Now().UTC().Format(time.RFC3339),
		Method:            MethodAudioXcorrEnvelope,
		ReferenceSourceID: referenceSourceID,
		SampleRate:        16000,
		Sources:           sources,
	}
}

func (m SyncMap) Source(sourceID string) (SourceSync, bool) {
	for _, source := range m.Sources {
		if source.ID == sourceID {
			return source, true
		}
	}
	return SourceSync{}, false
}

func (m SyncMap) ReferenceSeconds(transcriptSeconds float64) float64 {
	return transcriptSeconds + m.TranscriptToReferenceOffsetSeconds
}

func (m SyncMap) SourceSeconds(sourceID string, transcriptSeconds float64) (float64, error) {
	source, ok := m.Source(sourceID)
	if !ok {
		return 0, fmt.Errorf("source %q not found in sync map", sourceID)
	}
	return m.ReferenceSeconds(transcriptSeconds) + source.OffsetFromReferenceSeconds, nil
}

func (m SyncMap) ResolveRange(sourceID string, transcriptStart, transcriptEnd float64) (float64, float64, error) {
	if transcriptEnd <= transcriptStart {
		return 0, 0, fmt.Errorf("range end %.3f must be after start %.3f", transcriptEnd, transcriptStart)
	}
	start, err := m.SourceSeconds(sourceID, transcriptStart)
	if err != nil {
		return 0, 0, err
	}
	end, err := m.SourceSeconds(sourceID, transcriptEnd)
	if err != nil {
		return 0, 0, err
	}
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	return start, end, nil
}

func (m SyncMap) Validate(opts ValidationOptions) []string {
	var errs []string
	if m.Version != Version {
		errs = append(errs, "version must be "+Version)
	}
	if strings.TrimSpace(m.ReferenceSourceID) == "" {
		errs = append(errs, "reference_source_id is required")
	}
	seen := map[string]bool{}
	for i, source := range m.Sources {
		prefix := fmt.Sprintf("sources[%d]", i)
		if strings.TrimSpace(source.ID) == "" {
			errs = append(errs, prefix+".id is required")
		}
		if strings.TrimSpace(source.Path) == "" {
			errs = append(errs, prefix+".path is required")
		}
		if source.Confidence < 0 || source.Confidence > 1 || math.IsNaN(source.Confidence) {
			errs = append(errs, prefix+".confidence must be between 0 and 1")
		}
		if source.Confidence < 0.3 && !opts.AllowLowConfidence {
			errs = append(errs, prefix+".confidence below 0.30 requires allow-low-confidence")
		}
		seen[source.ID] = true
	}
	if len(m.Sources) == 0 {
		errs = append(errs, "sources must not be empty")
	}
	if m.ReferenceSourceID != "" && !seen[m.ReferenceSourceID] {
		errs = append(errs, "reference_source_id must exist in sources")
	}
	return errs
}

func (m SyncMap) ConfidenceWarnings() []string {
	warnings := append([]string{}, m.Warnings...)
	for _, source := range m.Sources {
		if source.Confidence < 0.65 {
			warnings = append(warnings, fmt.Sprintf("source %s confidence %.2f below 0.65", source.ID, source.Confidence))
		}
		warnings = append(warnings, source.Warnings...)
	}
	return warnings
}

func Read(path string) (SyncMap, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return SyncMap{}, err
	}
	var m SyncMap
	if err := json.Unmarshal(raw, &m); err != nil {
		return SyncMap{}, err
	}
	return m, nil
}

func Write(path string, m SyncMap) error {
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}
