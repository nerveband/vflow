package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileSetShowAndConfigInspectRedactSecrets(t *testing.T) {
	t.Setenv("VFLOW_CONFIG_PATH", filepath.Join(t.TempDir(), "config.yaml"))
	t.Setenv("ELEVENLABS_API_KEY", "secret-value")

	out, errOut, code := runCLI(t, "profile", "set", "--name", "test", "--provider", "elevenlabs", "--api-key-env", "ELEVENLABS_API_KEY", "--commit", "--format", "json")
	if code != 0 {
		t.Fatalf("profile set failed: code=%d stdout=%s stderr=%s", code, out, errOut)
	}
	if !strings.Contains(out, `"status": "written"`) {
		t.Fatalf("expected written profile status: %s", out)
	}

	out, errOut, code = runCLI(t, "profile", "show", "test", "--format", "json")
	if code != 0 {
		t.Fatalf("profile show failed: code=%d stdout=%s stderr=%s", code, out, errOut)
	}
	if strings.Contains(out, "secret-value") {
		t.Fatalf("profile show leaked secret: %s", out)
	}
	if !strings.Contains(out, "ELEVENLABS_API_KEY") {
		t.Fatalf("profile show missing env reference: %s", out)
	}

	out, errOut, code = runCLI(t, "config", "inspect", "--format", "json")
	if code != 0 {
		t.Fatalf("config inspect failed: code=%d stdout=%s stderr=%s", code, out, errOut)
	}
	if strings.Contains(out, "secret-value") {
		t.Fatalf("config inspect leaked secret: %s", out)
	}
}

func TestConfigSetDefaultsPersistsProjectRoot(t *testing.T) {
	t.Setenv("VFLOW_CONFIG_PATH", filepath.Join(t.TempDir(), "config.yaml"))

	_, errOut, code := runCLI(t, "config", "set-defaults", "--project-root", "./work", "--commit", "--format", "json")
	if code != 0 {
		t.Fatalf("config set-defaults failed: code=%d stderr=%s", code, errOut)
	}
	out, errOut, code := runCLI(t, "config", "defaults", "--format", "json")
	if code != 0 {
		t.Fatalf("config defaults failed: code=%d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, `"project_root": "./work"`) {
		t.Fatalf("defaults did not persist project root: %s", out)
	}
}
