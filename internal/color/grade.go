package color

import "os/exec"

type ApplyPlan struct {
	Command []string `json:"command"`
	LUT     string   `json:"lut"`
	Output  string   `json:"output"`
}

func LUTApplyPlan(input, lut, output string) ApplyPlan {
	return ApplyPlan{
		Command: []string{
			"ffmpeg", "-y",
			"-i", input,
			"-map", "0:v:0",
			"-map", "0:a?",
			"-dn",
			"-map_metadata", "-1",
			"-map_chapters", "-1",
			"-vf", "lut3d=file=" + lut + ":interp=tetrahedral",
			"-c:v", "libx264",
			"-pix_fmt", "yuv420p",
			"-c:a", "aac",
			"-movflags", "+faststart",
			output,
		},
		LUT:    lut,
		Output: output,
	}
}

func Run(plan ApplyPlan) error {
	if len(plan.Command) == 0 {
		return nil
	}
	return exec.Command(plan.Command[0], plan.Command[1:]...).Run()
}
