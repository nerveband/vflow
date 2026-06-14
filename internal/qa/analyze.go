package qa

type AnalyzePlan struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Render   string `json:"render"`
	PromptID string `json:"prompt_id"`
}
