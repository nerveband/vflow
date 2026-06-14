package media

type RenderPlan struct {
	Command     []string `json:"command"`
	OutputPath  string   `json:"output_path"`
	Description string   `json:"description"`
}
