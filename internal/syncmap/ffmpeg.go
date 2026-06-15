package syncmap

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

type AudioExtractPlan struct {
	Command    []string `json:"command"`
	Source     string   `json:"source"`
	Output     string   `json:"output"`
	Start      float64  `json:"start_seconds"`
	Duration   float64  `json:"duration_seconds"`
	SampleRate int      `json:"sample_rate"`
	Stream     int      `json:"audio_stream_index"`
}

func BuildAudioExtractPlan(ffmpegPath, source, output string, stream int, start, duration float64) AudioExtractPlan {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	if duration <= 0 {
		duration = 45
	}
	sampleRate := 16000
	cmd := []string{ffmpegPath, "-y"}
	if start > 0 {
		cmd = append(cmd, "-ss", fmtSeconds(start))
	}
	cmd = append(cmd,
		"-i", source,
		"-t", fmtSeconds(duration),
		"-map", fmt.Sprintf("0:a:%d?", stream),
		"-vn", "-dn",
		"-ac", "1",
		"-ar", strconv.Itoa(sampleRate),
		"-f", "s16le",
		"-map_metadata", "-1",
		"-map_chapters", "-1",
		output,
	)
	return AudioExtractPlan{Command: cmd, Source: source, Output: output, Start: start, Duration: duration, SampleRate: sampleRate, Stream: stream}
}

func BuildWaveformPlan(ffmpegPath, pcmPath, output string, sampleRate int) AudioExtractPlan {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	if sampleRate <= 0 {
		sampleRate = 16000
	}
	cmd := []string{
		ffmpegPath, "-y",
		"-f", "s16le",
		"-ar", strconv.Itoa(sampleRate),
		"-ac", "1",
		"-i", pcmPath,
		"-filter_complex", "showwavespic=s=1280x240:colors=0x2f6f73",
		"-frames:v", "1",
		"-map_metadata", "-1",
		"-map_chapters", "-1",
		output,
	}
	return AudioExtractPlan{Command: cmd, Source: pcmPath, Output: output, SampleRate: sampleRate}
}

func RunPlan(ctx context.Context, plan AudioExtractPlan) error {
	if len(plan.Command) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(plan.Output), 0o755); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, plan.Command[0], plan.Command[1:]...)
	return cmd.Run()
}

func ReadPCMEnvelope(path string, sampleRate int) ([]float64, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return PCM16LEToRMSEnvelope(raw, sampleRate)
}

func fmtSeconds(v float64) string {
	return strconv.FormatFloat(v, 'f', 3, 64)
}
