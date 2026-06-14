package media

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
)

type RenderPlan struct {
	Command     []string `json:"command"`
	OutputPath  string   `json:"output_path"`
	Description string   `json:"description"`
	Status      string   `json:"status,omitempty"`
}

type ProxyOptions struct {
	FFmpegPath string
	SourcePath string
	OutputPath string
	Preset     string
	Overwrite  bool
}

func BuildProxyPlan(opts ProxyOptions) RenderPlan {
	ffmpeg := firstNonEmpty(opts.FFmpegPath, "ffmpeg")
	source := firstNonEmpty(opts.SourcePath, "media/source.mp4")
	output := firstNonEmpty(opts.OutputPath, "media/proxy.mp4")
	scale := "scale=1920:-2"
	if opts.Preset == "edit-720p" {
		scale = "scale=1280:-2"
	}
	overwrite := "-n"
	if opts.Overwrite {
		overwrite = "-y"
	}
	return RenderPlan{
		Command:     []string{ffmpeg, overwrite, "-i", source, "-vf", scale, "-c:v", "libx264", "-preset", "veryfast", "-crf", "22", "-c:a", "aac", output},
		OutputPath:  output,
		Description: "proxy generation plan: " + firstNonEmpty(opts.Preset, "edit-1080p"),
		Status:      "planned",
	}
}

func RunProxy(ctx context.Context, opts ProxyOptions) (RenderPlan, error) {
	plan := BuildProxyPlan(opts)
	if err := os.MkdirAll(filepath.Dir(plan.OutputPath), 0o755); err != nil {
		return plan, err
	}
	cmd := exec.CommandContext(ctx, plan.Command[0], plan.Command[1:]...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return plan, &CommandError{Command: plan.Command, Output: string(out), Err: err}
	}
	plan.Status = "written"
	return plan, nil
}

type CommandError struct {
	Command []string
	Output  string
	Err     error
}

func (e *CommandError) Error() string {
	if e.Output != "" {
		return e.Err.Error() + ": " + e.Output
	}
	return e.Err.Error()
}
