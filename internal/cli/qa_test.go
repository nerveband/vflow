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
