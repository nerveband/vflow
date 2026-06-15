package render

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nerveband/vflow/internal/syncmap"
)

type TranscriptCut struct {
	Version  string                 `json:"version"`
	Summary  string                 `json:"summary,omitempty"`
	Segments []TranscriptCutSegment `json:"segments"`
}

type TranscriptCutSegment struct {
	ID                     string  `json:"id"`
	Source                 string  `json:"source"`
	SourceID               string  `json:"source_id,omitempty"`
	StartSeconds           float64 `json:"start_seconds"`
	EndSeconds             float64 `json:"end_seconds"`
	TranscriptStartSeconds float64 `json:"transcript_start_seconds,omitempty"`
	TranscriptEndSeconds   float64 `json:"transcript_end_seconds,omitempty"`
	TranscriptStart        string  `json:"transcript_start,omitempty"`
	TranscriptEnd          string  `json:"transcript_end,omitempty"`
	Speaker                string  `json:"speaker,omitempty"`
	Text                   string  `json:"text,omitempty"`
	Reason                 string  `json:"reason,omitempty"`
	SourceTimelineOffset   float64 `json:"source_timeline_offset,omitempty"`
	ReferenceStartSeconds  float64 `json:"reference_start_seconds,omitempty"`
	ReferenceEndSeconds    float64 `json:"reference_end_seconds,omitempty"`
}

type TranscriptCutPlanResult struct {
	Plan
	Segments        []TranscriptCutSegment `json:"segments"`
	DurationSeconds float64                `json:"duration_seconds"`
}

func ReadTranscriptCut(path string) (TranscriptCut, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return TranscriptCut{}, err
	}
	var cut TranscriptCut
	if err := json.Unmarshal(raw, &cut); err != nil {
		return TranscriptCut{}, err
	}
	return cut, nil
}

func WriteTranscriptCutReport(path string, result TranscriptCutPlanResult, status string) error {
	raw, err := json.MarshalIndent(map[string]any{
		"version":          "vflow-transcript-cut-render-report/v1",
		"status":           status,
		"render_path":      result.OutputPath,
		"duration_seconds": result.DurationSeconds,
		"command":          result.Command,
		"segments":         result.Segments,
		"target":           result.Target,
	}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func ResolveTranscriptCutWithSyncMap(edit TranscriptCut, m syncmap.SyncMap) (TranscriptCut, error) {
	for i := range edit.Segments {
		segment := &edit.Segments[i]
		if segment.SourceID == "" || segment.TranscriptStartSeconds == 0 && segment.TranscriptEndSeconds == 0 {
			continue
		}
		start, end, err := m.ResolveRange(segment.SourceID, segment.TranscriptStartSeconds, segment.TranscriptEndSeconds)
		if err != nil {
			return edit, err
		}
		source, _ := m.Source(segment.SourceID)
		if segment.Source == "" {
			segment.Source = source.Path
		}
		segment.StartSeconds = start
		segment.EndSeconds = end
		segment.SourceTimelineOffset = start - segment.TranscriptStartSeconds
		segment.ReferenceStartSeconds = m.ReferenceSeconds(segment.TranscriptStartSeconds)
		segment.ReferenceEndSeconds = m.ReferenceSeconds(segment.TranscriptEndSeconds)
	}
	return edit, nil
}

func TranscriptCutPlan(edit TranscriptCut, output, target string) (TranscriptCutPlanResult, error) {
	if target == "" {
		target = "youtube_16x9"
	}
	if output == "" {
		return TranscriptCutPlanResult{}, fmt.Errorf("missing output path")
	}
	if len(edit.Segments) == 0 {
		return TranscriptCutPlanResult{}, fmt.Errorf("transcript cut has no segments")
	}
	command := []string{"ffmpeg", "-y"}
	total := 0.0
	for i, segment := range edit.Segments {
		if strings.TrimSpace(segment.Source) == "" {
			return TranscriptCutPlanResult{}, fmt.Errorf("segment %d missing source", i)
		}
		duration := segment.EndSeconds - segment.StartSeconds
		if duration <= 0 {
			return TranscriptCutPlanResult{}, fmt.Errorf("segment %d has non-positive duration", i)
		}
		total += duration
		command = append(command,
			"-ss", strconv.FormatFloat(segment.StartSeconds, 'f', 3, 64),
			"-t", strconv.FormatFloat(duration, 'f', 3, 64),
			"-i", segment.Source,
		)
	}
	filter := buildTranscriptCutFilter(len(edit.Segments))
	command = append(command,
		"-filter_complex", filter,
		"-map", "[v]",
		"-map", "[a]",
		"-dn",
		"-map_metadata", "-1",
		"-map_chapters", "-1",
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-movflags", "+faststart",
		output,
	)
	return TranscriptCutPlanResult{
		Plan: Plan{
			Command:     command,
			OutputPath:  output,
			Target:      target,
			Description: "ffmpeg transcript-selected social cut",
		},
		Segments:        edit.Segments,
		DurationSeconds: total,
	}, nil
}

func buildTranscriptCutFilter(count int) string {
	parts := make([]string, 0, count*3+1)
	concatInputs := strings.Builder{}
	for i := 0; i < count; i++ {
		parts = append(parts, fmt.Sprintf("[%d:v]scale=1920:1080:force_original_aspect_ratio=decrease,pad=1920:1080:(ow-iw)/2:(oh-ih)/2,setsar=1,setpts=PTS-STARTPTS[v%d]", i, i))
		parts = append(parts, fmt.Sprintf("[%d:a]aresample=async=1:first_pts=0,asetpts=PTS-STARTPTS[a%d]", i, i))
		concatInputs.WriteString(fmt.Sprintf("[v%d][a%d]", i, i))
	}
	parts = append(parts, fmt.Sprintf("%sconcat=n=%d:v=1:a=1[v][a]", concatInputs.String(), count))
	return strings.Join(parts, ";")
}
