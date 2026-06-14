package color

import "os/exec"

type ApplyPlan struct {
	Command []string `json:"command"`
	LUT     string   `json:"lut"`
	Output  string   `json:"output"`
}

func LUTApplyPlan(input, lut, output string) ApplyPlan {
	return ApplyPlan{
		Command: []string{"ffmpeg", "-y", "-i", input, "-vf", "lut3d=file=" + lut + ":interp=tetrahedral", output},
		LUT:     lut,
		Output:  output,
	}
}

func Run(plan ApplyPlan) error {
	if len(plan.Command) == 0 {
		return nil
	}
	return exec.Command(plan.Command[0], plan.Command[1:]...).Run()
}
