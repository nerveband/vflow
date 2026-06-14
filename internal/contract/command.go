package contract

type Input struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description,omitempty"`
}

type Output struct {
	Name        string `json:"name"`
	Schema      string `json:"schema"`
	Description string `json:"description,omitempty"`
}

type Command struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Canonical      bool     `json:"canonical"`
	ReadOnly       bool     `json:"read_only"`
	Destructive    bool     `json:"destructive"`
	Idempotent     bool     `json:"idempotent"`
	SupportsDryRun bool     `json:"supports_dry_run"`
	RequiresCommit bool     `json:"requires_commit"`
	Scope          string   `json:"scope"`
	Inputs         []Input  `json:"inputs,omitempty"`
	Outputs        []Output `json:"outputs,omitempty"`
	Examples       []string `json:"examples,omitempty"`
}
