package media

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
)

type SamplePlan struct {
	Frames  []string `json:"frames"`
	Output  string   `json:"output"`
	Command []string `json:"command,omitempty"`
	Status  string   `json:"status,omitempty"`
}

type SampleOptions struct {
	FFmpegPath string
	SourcePath string
	OutputPath string
	Count      int
	Overwrite  bool
}

func BuildSamplePlan(opts SampleOptions) SamplePlan {
	if opts.Count <= 0 {
		opts.Count = 12
	}
	frames := make([]string, 0, opts.Count)
	for i := 0; i < opts.Count; i++ {
		frames = append(frames, fmt.Sprintf("sample_%03d", i+1))
	}
	ffmpeg := firstNonEmpty(opts.FFmpegPath, "ffmpeg")
	source := firstNonEmpty(opts.SourcePath, "media/source.mp4")
	output := firstNonEmpty(opts.OutputPath, "reports/contact-sheet.jpg")
	cols := int(math.Ceil(math.Sqrt(float64(opts.Count))))
	if cols < 1 {
		cols = 1
	}
	rows := int(math.Ceil(float64(opts.Count) / float64(cols)))
	overwrite := "-n"
	if opts.Overwrite {
		overwrite = "-y"
	}
	filter := fmt.Sprintf("thumbnail,scale=320:-1,tile=%dx%d", cols, rows)
	return SamplePlan{
		Frames:  frames,
		Output:  output,
		Command: []string{ffmpeg, overwrite, "-i", source, "-vf", filter, "-frames:v", "1", output},
		Status:  "planned",
	}
}

func RunSamples(ctx context.Context, opts SampleOptions) (SamplePlan, error) {
	plan := BuildSamplePlan(opts)
	if err := os.MkdirAll(filepath.Dir(plan.Output), 0o755); err != nil {
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
