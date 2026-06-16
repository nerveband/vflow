package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFeedbackRequiresMessage(t *testing.T) {
	_, errOut, code := runCLI(t, "feedback", "--format", "json", "--format-error", "json")
	if code != 4 {
		t.Fatalf("expected exit code 4, got %d stderr=%s", code, errOut)
	}
	if !strings.Contains(errOut, `"code": "MISSING_FEEDBACK_MESSAGE"`) {
		t.Fatalf("expected missing message error, got:\n%s", errOut)
	}
}

func TestFeedbackDryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	out, errOut, code := runCLI(t,
		"feedback",
		"--project", dir,
		"--message", "tighten transcript cut scoring",
		"--category", "implementation",
		"--format", "json",
		"--format-error", "json",
	)
	if code != 0 {
		t.Fatalf("expected success, got %d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, `"status": "planned"`) || !strings.Contains(out, `"version": "vflow-feedback/v1"`) {
		t.Fatalf("unexpected feedback output:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "reports", "feedback.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not create feedback ledger, stat err=%v", err)
	}
}

func TestFeedbackCommitAppendsJSONL(t *testing.T) {
	dir := t.TempDir()
	out, errOut, code := runCLI(t,
		"feedback",
		"--project", dir,
		"--message", "prove Gemini files upload with query auth",
		"--category", "qa",
		"--source", "codex",
		"--commit",
		"--format", "json",
		"--format-error", "json",
	)
	if code != 0 {
		t.Fatalf("expected success, got %d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, `"status": "recorded"`) || !strings.Contains(out, `"commit": true`) {
		t.Fatalf("unexpected feedback output:\n%s", out)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "reports", "feedback.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"version":"vflow-feedback/v1"`, `"category":"qa"`, `"source":"codex"`, `"message":"prove Gemini files upload with query auth"`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("feedback ledger missing %s in:\n%s", want, raw)
		}
	}
	if lines := strings.Count(string(raw), "\n"); lines != 1 {
		t.Fatalf("expected one JSONL line, got %d in:\n%s", lines, raw)
	}
}
