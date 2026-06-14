package render

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

type Options struct {
	Input      string `json:"input"`
	Output     string `json:"output"`
	Target     string `json:"target"`
	MaxSeconds int    `json:"max_seconds,omitempty"`
}

type Plan struct {
	Command     []string `json:"command"`
	OutputPath  string   `json:"output_path"`
	Target      string   `json:"target"`
	Description string   `json:"description"`
}

func PreviewPlan(opts Options) Plan {
	if opts.Target == "" {
		opts.Target = "youtube_16x9"
	}
	if opts.MaxSeconds <= 0 {
		opts.MaxSeconds = 3
	}
	command := []string{
		"ffmpeg", "-y",
		"-i", opts.Input,
		"-t", strconv.Itoa(opts.MaxSeconds),
		"-vf", "scale=1920:1080:force_original_aspect_ratio=decrease,pad=1920:1080:(ow-iw)/2:(oh-ih)/2",
		"-af", "afade=t=in:st=0:d=0.03,afade=t=out:st=1.97:d=0.03",
		opts.Output,
	}
	return Plan{Command: command, OutputPath: opts.Output, Target: opts.Target, Description: "ffmpeg preview render"}
}

func Run(plan Plan) error {
	if len(plan.Command) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(plan.OutputPath), 0o755); err != nil {
		return err
	}
	cmd := exec.Command(plan.Command[0], plan.Command[1:]...)
	return cmd.Run()
}

func WriteReport(path string, plan Plan, status string) error {
	raw, err := json.MarshalIndent(map[string]any{
		"version":     "vflow-render-report/v1",
		"status":      status,
		"render_path": plan.OutputPath,
		"command":     plan.Command,
		"target":      plan.Target,
	}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}
