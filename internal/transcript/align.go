package transcript

type AlignmentPlan struct {
	Method  string   `json:"method"`
	Command []string `json:"command"`
	Output  string   `json:"output"`
}
