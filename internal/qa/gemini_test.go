package qa

import "testing"

func TestNormalizeGeminiModel(t *testing.T) {
	for input, want := range map[string]string{
		"3.1 pro":                "gemini-3.1-pro-preview",
		"gemini-3.1-pro-preview": "gemini-3.1-pro-preview",
		"gemini-3.5-flash":       "gemini-3.5-flash",
		"":                       "gemini-3.5-flash",
	} {
		got, err := NormalizeModel(input)
		if err != nil {
			t.Fatalf("%s: %v", input, err)
		}
		if got != want {
			t.Fatalf("%s: got %s want %s", input, got, want)
		}
	}
	if _, err := NormalizeModel("bad-model"); err == nil {
		t.Fatalf("expected bad model to fail")
	}
}

func TestDoctorMissingKeyIsCapabilityWarning(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	got, err := Doctor("gemini-3.5-flash", false)
	if err != nil {
		t.Fatal(err)
	}
	if got.OK {
		t.Fatalf("missing key should not be ok")
	}
	if got.ErrorCode != "MISSING_API_KEY" {
		t.Fatalf("unexpected error code: %s", got.ErrorCode)
	}
}
