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

func TestAuthDoctorProviderSpecificOutput(t *testing.T) {
	t.Setenv("ELEVENLABS_API_KEY", "secret-value")

	out, errOut, code := runCLI(t, "auth", "doctor", "--provider", "elevenlabs", "--format", "json", "--format-error", "json")
	if code != 0 {
		t.Fatalf("auth doctor failed: code=%d stdout=%s stderr=%s", code, out, errOut)
	}
	for _, want := range []string{`"provider": "elevenlabs"`, `"key_present": true`, `"ELEVENLABS_API_KEY"`, `"secrets_redacted": true`, `"word_timestamps"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("auth doctor missing %s in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "secret-value") {
		t.Fatalf("auth doctor leaked secret: %s", out)
	}
}

func TestAuthDoctorMissingKeyDegradesWithoutFailure(t *testing.T) {
	t.Setenv("GLADIA_API_KEY", "")

	out, errOut, code := runCLI(t, "auth", "doctor", "--provider", "gladia", "--format", "json", "--format-error", "json")
	if code != 0 {
		t.Fatalf("auth doctor should not fail for missing key: code=%d stdout=%s stderr=%s", code, out, errOut)
	}
	if !strings.Contains(out, `"status": "degraded"`) || !strings.Contains(out, `"status": "missing_key"`) || !strings.Contains(out, `"GLADIA_API_KEY"`) {
		t.Fatalf("missing key did not degrade clearly:\n%s", out)
	}
}

func TestAuthDoctorLiveRequiresCommit(t *testing.T) {
	_, errOut, code := runCLI(t, "auth", "doctor", "--provider", "gemini", "--live", "--format", "json", "--format-error", "json")
	if code != 5 {
		t.Fatalf("expected safety exit 5, got %d stderr=%s", code, errOut)
	}
	if !strings.Contains(errOut, "live auth checks require --commit") || !strings.Contains(errOut, `"ok": false`) {
		t.Fatalf("expected structured safety error, got:\n%s", errOut)
	}
}

func TestDoctorReportsNLECapabilities(t *testing.T) {
	out, errOut, code := runCLI(t, "doctor", "--local", "--format", "json", "--format-error", "json")
	if code != 0 {
		t.Fatalf("doctor failed: code=%d stdout=%s stderr=%s", code, out, errOut)
	}
	for _, want := range []string{
		`"local": true`,
		`"nle":`,
		`"targets":`,
		`"resolve"`,
		`"fcpxml"`,
		`"exports_sidecars": true`,
		`"missing_sidecar"`,
		`"real_editor_fixture_gap"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("doctor output missing %s in:\n%s", want, out)
		}
	}
}
