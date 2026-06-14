package transcript

type BakeoffPlan struct {
	Providers []string       `json:"providers"`
	Checks    []CheckResult  `json:"checks"`
	Scores    map[string]int `json:"scores"`
}
