package transcript

import "context"

type Capabilities struct {
	WordTimestamps bool `json:"word_timestamps"`
	Diarization    bool `json:"diarization"`
	Keyterms       bool `json:"keyterms"`
	LiveCalls      bool `json:"live_calls"`
}

type ProviderConfig struct {
	APIKeyEnv string `json:"api_key_env,omitempty"`
}

type CheckResult struct {
	Provider string       `json:"provider"`
	OK       bool         `json:"ok"`
	Missing  []string     `json:"missing,omitempty"`
	Cap      Capabilities `json:"capabilities"`
}

type TranscribeJob struct {
	AudioPath string `json:"audio_path"`
	Live      bool   `json:"live"`
}

type ProviderRun struct {
	Provider string `json:"provider"`
	Status   string `json:"status"`
}

type Provider interface {
	Name() string
	Capabilities() Capabilities
	ValidateConfig(ctx context.Context, cfg ProviderConfig) CheckResult
	Transcribe(ctx context.Context, job TranscribeJob) (ProviderRun, error)
}
