package media

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nerveband/vflow/internal/syncmap"
)

type TranscriptRange struct {
	ID       string  `json:"id"`
	SourceID string  `json:"source_id"`
	Start    float64 `json:"transcript_start_seconds"`
	End      float64 `json:"transcript_end_seconds"`
	Text     string  `json:"text,omitempty"`
	Output   string  `json:"output,omitempty"`
}

type SourceRange struct {
	ID              string   `json:"id"`
	SourceID        string   `json:"source_id"`
	Source          string   `json:"source"`
	Output          string   `json:"output"`
	TranscriptStart float64  `json:"transcript_start_seconds"`
	TranscriptEnd   float64  `json:"transcript_end_seconds"`
	SourceStart     float64  `json:"source_start_seconds"`
	SourceEnd       float64  `json:"source_end_seconds"`
	Duration        float64  `json:"duration_seconds"`
	Command         []string `json:"command"`
}

type SourceRangeManifest struct {
	Version        string        `json:"version"`
	SyncMap        string        `json:"sync_map,omitempty"`
	EstimatedBytes int64         `json:"estimated_bytes"`
	Ranges         []SourceRange `json:"ranges"`
	Warnings       []string      `json:"warnings,omitempty"`
}

func PlanSourceRanges(m syncmap.SyncMap, ranges []TranscriptRange, outputDir, ffmpegPath string) (SourceRangeManifest, error) {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	if outputDir == "" {
		outputDir = "media/ranges"
	}
	manifest := SourceRangeManifest{Version: "vflow-source-range-manifest/v1"}
	for i, tr := range ranges {
		if tr.ID == "" {
			tr.ID = fmt.Sprintf("range_%06d", i+1)
		}
		source, ok := m.Source(tr.SourceID)
		if !ok {
			return manifest, fmt.Errorf("source %q not found in sync map", tr.SourceID)
		}
		start, end, err := m.ResolveRange(tr.SourceID, tr.Start, tr.End)
		if err != nil {
			return manifest, err
		}
		out := tr.Output
		if out == "" {
			out = filepath.Join(outputDir, tr.ID+"-"+tr.SourceID+".mp4")
		}
		duration := end - start
		cmd := []string{
			ffmpegPath, "-y",
			"-ss", fmtFloat(start),
			"-i", source.Path,
			"-t", fmtFloat(duration),
			"-map", "0:v:0",
			"-map", "0:a?",
			"-dn",
			"-map_metadata", "-1",
			"-map_chapters", "-1",
			"-c:v", "libx264",
			"-pix_fmt", "yuv420p",
			"-c:a", "aac",
			"-movflags", "+faststart",
			out,
		}
		manifest.Ranges = append(manifest.Ranges, SourceRange{
			ID: tr.ID, SourceID: tr.SourceID, Source: source.Path, Output: out,
			TranscriptStart: tr.Start, TranscriptEnd: tr.End, SourceStart: start, SourceEnd: end,
			Duration: duration, Command: cmd,
		})
		manifest.EstimatedBytes += int64(duration * 25_000_000 / 8)
	}
	return manifest, nil
}

func RunSourceRanges(ctx context.Context, manifest SourceRangeManifest) error {
	for _, r := range manifest.Ranges {
		if len(r.Command) == 0 {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(r.Output), 0o755); err != nil {
			return err
		}
		cmd := exec.CommandContext(ctx, r.Command[0], r.Command[1:]...)
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}

func ReadTranscriptRanges(path string) ([]TranscriptRange, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return nil, fmt.Errorf("empty ranges file")
	}
	var ranges []TranscriptRange
	if strings.HasPrefix(text, "[") {
		if err := jsonUnmarshal(raw, &ranges); err != nil {
			return nil, err
		}
		return ranges, nil
	}
	var wrapper struct {
		Ranges []TranscriptRange `json:"ranges"`
	}
	if err := jsonUnmarshal(raw, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Ranges, nil
}

var jsonUnmarshal = func(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func fmtFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', 3, 64)
}
