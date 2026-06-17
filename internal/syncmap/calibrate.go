package syncmap

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
)

type SourceInput struct {
	ID               string  `json:"id"`
	Path             string  `json:"path"`
	AudioStreamIndex int     `json:"audio_stream_index,omitempty"`
	WindowStart      float64 `json:"window_start_seconds,omitempty"`
	WindowDuration   float64 `json:"window_duration_seconds,omitempty"`
}

type CalibrationOptions struct {
	ProjectID         string        `json:"project_id,omitempty"`
	ReferenceSourceID string        `json:"reference_source_id"`
	Sources           []SourceInput `json:"sources"`
	OutputPath        string        `json:"output_path,omitempty"`
	ProofDir          string        `json:"proof_dir,omitempty"`
	FFmpegPath        string        `json:"ffmpeg_path,omitempty"`
	MaxLagSeconds     float64       `json:"max_lag_seconds,omitempty"`
	Windows           int           `json:"windows,omitempty"`
	FrameRate         string        `json:"frame_rate,omitempty"`
	Commit            bool          `json:"commit,omitempty"`
}

type CalibrationReport struct {
	Version       string             `json:"version"`
	Status        string             `json:"status"`
	SyncMapPath   string             `json:"sync_map_path,omitempty"`
	SyncMap       SyncMap            `json:"sync_map"`
	ExtractPlans  []AudioExtractPlan `json:"extract_plans,omitempty"`
	WaveformPlans []AudioExtractPlan `json:"waveform_plans,omitempty"`
	Warnings      []string           `json:"warnings,omitempty"`
	Validation    []string           `json:"validation_errors,omitempty"`
}

func Calibrate(ctx context.Context, opts CalibrationOptions) (CalibrationReport, error) {
	if opts.ReferenceSourceID == "" {
		return CalibrationReport{}, fmt.Errorf("reference source id is required")
	}
	if len(opts.Sources) == 0 {
		return CalibrationReport{}, fmt.Errorf("at least one source is required")
	}
	if opts.MaxLagSeconds <= 0 {
		opts.MaxLagSeconds = 90
	}
	if opts.Windows <= 0 {
		opts.Windows = 1
	}
	if opts.ProofDir == "" {
		opts.ProofDir = filepath.Join(filepath.Dir(opts.OutputPath), "sync-proof")
	}
	sources := make([]SourceSync, 0, len(opts.Sources))
	extractPlans := []AudioExtractPlan{}
	waveformPlans := []AudioExtractPlan{}
	envelopes := map[string][][]float64{}
	referenceFound := false
	for _, src := range opts.Sources {
		if src.ID == opts.ReferenceSourceID {
			referenceFound = true
		}
		duration := src.WindowDuration
		if duration <= 0 {
			duration = 45
		}
		for window := 0; window < opts.Windows; window++ {
			suffix := ""
			if opts.Windows > 1 {
				suffix = fmt.Sprintf("-w%02d", window+1)
			}
			pcm := filepath.Join(opts.ProofDir, sanitizeID(src.ID)+suffix+".s16le")
			start := src.WindowStart + float64(window)*duration*2
			extract := BuildAudioExtractPlan(opts.FFmpegPath, src.Path, pcm, src.AudioStreamIndex, start, duration)
			waveform := BuildWaveformPlan(opts.FFmpegPath, pcm, filepath.Join(opts.ProofDir, sanitizeID(src.ID)+suffix+".waveform.png"), extract.SampleRate)
			extractPlans = append(extractPlans, extract)
			waveformPlans = append(waveformPlans, waveform)
			if opts.Commit {
				if err := RunPlan(ctx, extract); err != nil {
					return CalibrationReport{}, err
				}
				env, err := ReadPCMEnvelope(pcm, extract.SampleRate)
				if err != nil {
					return CalibrationReport{}, err
				}
				envelopes[src.ID] = append(envelopes[src.ID], env)
				_ = RunPlan(ctx, waveform)
			}
		}
		sync := SourceSync{ID: src.ID, Path: src.Path, AudioStreamIndex: src.AudioStreamIndex, Confidence: 1}
		if src.ID != opts.ReferenceSourceID && opts.Windows > 1 {
			sync.Warnings = append(sync.Warnings, fmt.Sprintf("audio sync voted across %d windows", opts.Windows))
		}
		sources = append(sources, sync)
	}
	if !referenceFound {
		return CalibrationReport{}, fmt.Errorf("reference source %q not found", opts.ReferenceSourceID)
	}
	if opts.Commit {
		ref := envelopes[opts.ReferenceSourceID]
		for i := range sources {
			if sources[i].ID == opts.ReferenceSourceID {
				continue
			}
			results := []CorrelationResult{}
			sourceEnvelopes := envelopes[sources[i].ID]
			for window := 0; window < min(len(ref), len(sourceEnvelopes)); window++ {
				result, err := Correlate(ref[window], sourceEnvelopes[window], EnvelopeRate, opts.MaxLagSeconds)
				if err != nil {
					return CalibrationReport{}, err
				}
				results = append(results, result)
			}
			result := voteCorrelations(results)
			sources[i].OffsetFromReferenceSeconds = result.LagSeconds
			sources[i].Confidence = result.Confidence
		}
	}
	m := New(opts.ProjectID, opts.ReferenceSourceID, sources)
	m.FrameRate = opts.FrameRate
	report := CalibrationReport{Version: "vflow-media-sync-report/v1", Status: "planned", SyncMapPath: filepath.ToSlash(opts.OutputPath), SyncMap: m, ExtractPlans: extractPlans, WaveformPlans: waveformPlans}
	if opts.Commit {
		report.Status = "written"
		report.Validation = m.Validate(ValidationOptions{MinConfidence: 0.65})
		report.Warnings = m.ConfidenceWarnings()
		if len(report.Validation) == 0 && opts.OutputPath != "" {
			if err := Write(opts.OutputPath, m); err != nil {
				return CalibrationReport{}, err
			}
		}
	}
	return report, nil
}

func voteCorrelations(results []CorrelationResult) CorrelationResult {
	if len(results) == 0 {
		return CorrelationResult{}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].LagSeconds < results[j].LagSeconds
	})
	mid := len(results) / 2
	voted := results[mid]
	if len(results)%2 == 0 {
		voted.LagSeconds = (results[mid-1].LagSeconds + results[mid].LagSeconds) / 2
		voted.LagSamples = int(math.Round(voted.LagSeconds * EnvelopeRate))
	}
	confidence := 0.0
	peak := 0.0
	for _, result := range results {
		confidence += result.Confidence
		peak += result.Peak
	}
	voted.Confidence = confidence / float64(len(results))
	voted.Peak = peak / float64(len(results))
	return voted
}

func ApplyTranscriptOffset(m SyncMap, transcriptSeconds, referenceSeconds float64, anchorID, method, text string, confidence float64) SyncMap {
	m.TranscriptToReferenceOffsetSeconds = referenceSeconds - transcriptSeconds
	m.Anchors = append(m.Anchors, Anchor{ID: anchorID, TranscriptSeconds: transcriptSeconds, ReferenceSeconds: referenceSeconds, Method: method, MatchedText: text, Confidence: confidence})
	return m
}

func sanitizeID(id string) string {
	id = strings.TrimSpace(strings.ToLower(id))
	repl := strings.NewReplacer("/", "-", "\\", "-", " ", "-", ":", "-", "..", "-")
	id = repl.Replace(id)
	if id == "" {
		return "source"
	}
	return id
}
