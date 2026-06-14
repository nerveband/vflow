package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadAndRedactConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("VFLOW_CONFIG_PATH", path)

	cfg := Default()
	cfg.DefaultProfile = "test"
	cfg.Defaults.ProjectRoot = "./work"
	cfg.Profiles["test"] = Profile{Providers: map[string]Provider{
		"elevenlabs": {APIKeyEnv: "ELEVENLABS_API_KEY", APIKey: "secret-value"},
	}}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.DefaultProfile != "test" || loaded.Defaults.ProjectRoot != "./work" {
		t.Fatalf("unexpected loaded config: %+v", loaded)
	}
	redacted := loaded.Redacted()
	if got := redacted.Profiles["test"].Providers["elevenlabs"].APIKey; got != "REDACTED" {
		t.Fatalf("api key was not redacted: %q", got)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) == "" {
		t.Fatalf("expected config file to be written")
	}
}

func TestLoadMissingReturnsDefaults(t *testing.T) {
	t.Setenv("VFLOW_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.yaml"))
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Defaults.Format != "json" || cfg.Defaults.ProjectRoot != "." {
		t.Fatalf("unexpected defaults: %+v", cfg.Defaults)
	}
}
