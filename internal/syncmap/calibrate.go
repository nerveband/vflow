package syncmap

import (
	"context"
	"fmt"
	"path/filepath"
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
	if opts.ProofDir == "" {
		opts.ProofDir = filepath.Join(filepath.Dir(opts.OutputPath), "sync-proof")
	}
	sources := make([]SourceSync, 0, len(opts.Sources))
	extractPlans := []AudioExtractPlan{}
	waveformPlans := []AudioExtractPlan{}
	envelopes := map[string][]float64{}
	referenceFound := false
	for _, src := range opts.Sources {
		if src.ID == opts.ReferenceSourceID {
			referenceFound = true
		}
		pcm := filepath.Join(opts.ProofDir, sanitizeID(src.ID)+".s16le")
		duration := src.WindowDuration
		if duration <= 0 {
			duration = 45
		}
		extract := BuildAudioExtractPlan(opts.FFmpegPath, src.Path, pcm, src.AudioStreamIndex, src.WindowStart, duration)
		waveform := BuildWaveformPlan(opts.FFmpegPath, pcm, filepath.Join(opts.ProofDir, sanitizeID(src.ID)+".waveform.png"), extract.SampleRate)
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
			envelopes[src.ID] = env
			_ = RunPlan(ctx, waveform)
		}
		sources = append(sources, SourceSync{ID: src.ID, Path: src.Path, AudioStreamIndex: src.AudioStreamIndex, Confidence: 1})
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
			result, err := Correlate(ref, envelopes[sources[i].ID], EnvelopeRate, opts.MaxLagSeconds)
			if err != nil {
				return CalibrationReport{}, err
			}
			sources[i].OffsetFromReferenceSeconds = result.LagSeconds
			sources[i].Confidence = result.Confidence
		}
	}
	m := New(opts.ProjectID, opts.ReferenceSourceID, sources)
	m.FrameRate = opts.FrameRate
	report := CalibrationReport{Version: "vflow-media-sync-report/v1", Status: "planned", SyncMapPath: filepath.ToSlash(opts.OutputPath), SyncMap: m, ExtractPlans: extractPlans, WaveformPlans: waveformPlans}
	if opts.Commit {
		report.Status = "written"
		report.Validation = m.Validate(ValidationOptions{})
		report.Warnings = m.ConfidenceWarnings()
		if len(report.Validation) == 0 && opts.OutputPath != "" {
			if err := Write(opts.OutputPath, m); err != nil {
				return CalibrationReport{}, err
			}
		}
	}
	return report, nil
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
