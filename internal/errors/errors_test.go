package errors

import "testing"

func TestValidationErrorCarriesStructuredFields(t *testing.T) {
	err := Validation("INVALID_ENUM", "unsupported provider", "Use one of: local, elevenlabs, soniox", false)
	if err.Code != "INVALID_ENUM" {
		t.Fatalf("unexpected code: %s", err.Code)
	}
	if err.ExitCode != 4 {
		t.Fatalf("unexpected exit code: %d", err.ExitCode)
	}
	if err.Retryable {
		t.Fatalf("validation error should not be retryable")
	}
}
