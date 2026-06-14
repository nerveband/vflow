package render

type Verification struct {
	Status string `json:"status"`
}

type VerifyResult struct {
	Status string `json:"status"`
	Render string `json:"render"`
}
