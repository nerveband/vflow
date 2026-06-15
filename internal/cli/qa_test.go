package cli

import (
	"strings"
	"testing"
)

func TestQADoctorDryRunWithoutKeyReturnsWarning(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	out, errOut, code := runCLI(t, "qa", "doctor", "--provider", "gemini", "--model", "3.1 pro", "--format", "json")
	if code != 0 {
		t.Fatalf("expected success capability warning, got %d stderr=%s", code, errOut)
	}
	for _, want := range []string{"MISSING_API_KEY", "gemini-3.1-pro-preview"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q: %s", want, out)
		}
	}
}

func TestQADoctorUsesNamedKeyEnv(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GEMINI_CANDIDATE_1", "test-key")
	out, errOut, code := runCLI(t, "qa", "doctor", "--provider", "gemini", "--key-env", "GEMINI_CANDIDATE_1", "--format", "json")
	if code != 0 {
		t.Fatalf("expected success, got %d stderr=%s", code, errOut)
	}
	for _, want := range []string{`"key_source": "env:GEMINI_CANDIDATE_1"`, `"key_present": true`} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q: %s", want, out)
		}
	}
	if strings.Contains(out, "test-key") {
		t.Fatalf("output leaked key: %s", out)
	}
}

func TestQADoctorRejectsInvalidKeyEnv(t *testing.T) {
	_, errOut, code := runCLI(t, "qa", "doctor", "--provider", "gemini", "--key-env", "BAD-NAME", "--format-error", "json")
	if code != 4 {
		t.Fatalf("expected validation failure, got %d stderr=%s", code, errOut)
	}
	if !strings.Contains(errOut, "INVALID_KEY_ENV") {
		t.Fatalf("expected invalid key env error: %s", errOut)
	}
}

func TestQAAnalyzeDryRunDefaultsToFilesUpload(t *testing.T) {
	out, errOut, code := runCLI(t, "qa", "analyze", "--project", t.TempDir(), "--render", "../../fixtures/media/tiny/source.mp4", "--provider", "gemini", "--model", "gemini-3.5-flash", "--format", "json")
	if code != 0 {
		t.Fatalf("expected success, got %d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, `"upload": "files"`) {
		t.Fatalf("expected files upload mode: %s", out)
	}
}

func TestQAAnalyzeRejectsInvalidUploadMode(t *testing.T) {
	_, errOut, code := runCLI(t, "qa", "analyze", "--upload", "bad", "--format", "json", "--format-error", "json")
	if code != 4 {
		t.Fatalf("expected validation error, got %d stderr=%s", code, errOut)
	}
	if !strings.Contains(errOut, "INVALID_ENUM") {
		t.Fatalf("expected invalid enum error: %s", errOut)
	}
}
