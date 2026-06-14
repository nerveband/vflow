package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFramingPresetImportAndValidate(t *testing.T) {
	dir := t.TempDir()
	if _, _, code := runCLI(t, "project", "init", "--path", dir, "--id", "framing_test", "--commit", "--format", "json"); code != 0 {
		t.Fatalf("project init failed")
	}
	input := filepath.Join(dir, "framing-presets.json")
	raw, err := os.ReadFile("../../fixtures/project/basic/calibration/framing-presets.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(input, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, errOut, code := runCLI(t, "framing", "preset", "import", "--project", dir, "--input", input, "--commit", "--format", "json"); code != 0 {
		t.Fatalf("preset import failed: %d %s", code, errOut)
	}
	if _, errOut, code := runCLI(t, "framing", "preset", "validate", "--project", dir, "--format", "json"); code != 0 {
		t.Fatalf("preset validate failed: %d %s", code, errOut)
	}
}
