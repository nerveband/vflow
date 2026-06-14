package config

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DefaultProfile string             `json:"default_profile" yaml:"default_profile"`
	Defaults       Defaults           `json:"defaults" yaml:"defaults"`
	Profiles       map[string]Profile `json:"profiles" yaml:"profiles"`
}

type Defaults struct {
	Format      string `json:"format" yaml:"format"`
	ProjectRoot string `json:"project_root" yaml:"project_root"`
	DataSource  string `json:"data_source" yaml:"data_source"`
}

type Profile struct {
	Providers map[string]Provider `json:"providers" yaml:"providers"`
}

type Provider struct {
	APIKeyEnv string `json:"api_key_env,omitempty" yaml:"api_key_env,omitempty"`
	APIKey    string `json:"api_key,omitempty" yaml:"api_key,omitempty"`
}

func Default() Config {
	return Config{
		DefaultProfile: "default",
		Defaults: Defaults{
			Format:      "json",
			ProjectRoot: ".",
			DataSource:  "auto",
		},
		Profiles: map[string]Profile{},
	}
}

func Path() (string, error) {
	if path := os.Getenv("VFLOW_CONFIG_PATH"); path != "" {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".vflow", "config.yaml"), nil
}

func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Default(), nil
		}
		return Config{}, err
	}
	cfg := Default()
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}
	return cfg, nil
}

func Save(cfg Config) error {
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}
	path, err := Path()
	if err != nil {
		return err
	}
	raw, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".vflow-config-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	committed = true
	return nil
}

func (c Config) Redacted() Config {
	out := c
	out.Profiles = map[string]Profile{}
	for name, profile := range c.Profiles {
		copied := Profile{Providers: map[string]Provider{}}
		for provider, cfg := range profile.Providers {
			if cfg.APIKey != "" {
				cfg.APIKey = "REDACTED"
			}
			copied.Providers[provider] = cfg
		}
		out.Profiles[name] = copied
	}
	return out
}
