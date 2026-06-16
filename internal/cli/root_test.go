package cli

import (
	"bytes"
	"strings"
	"testing"
)

func runCLI(t *testing.T, args ...string) (string, string, int) {
	t.Helper()

	var stdout, stderr bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	if err != nil {
		if exitErr, ok := err.(interface{ ExitCode() int }); ok {
			return stdout.String(), stderr.String(), exitErr.ExitCode()
		}
		return stdout.String(), stderr.String(), 1
	}
	return stdout.String(), stderr.String(), 0
}

func TestRootHelpListsCoreCommands(t *testing.T) {
	out, errOut, code := runCLI(t, "--help")
	if code != 0 {
		t.Fatalf("expected success, got %d stderr=%s", code, errOut)
	}
	for _, want := range []string{"project", "media", "transcript", "cleanup", "framing", "timeline", "render", "nle", "schema", "agent-context"} {
		if !strings.Contains(out, want) {
			t.Fatalf("help missing %q in:\n%s", want, out)
		}
	}
}

func TestSynonymCommandsResolve(t *testing.T) {
	for _, args := range [][]string{
		{"outputs", "list", "--help"},
		{"project", "new-project", "--help"},
		{"media", "inspect-media", "--help"},
		{"media", "make-proxy", "--help"},
		{"cleanup", "cleanup-plan", "--help"},
		{"framing", "speaker-map", "--help"},
		{"framing", "apply-framing", "--help"},
		{"timeline", "build-timeline", "--help"},
		{"render", "qa-render", "--help"},
		{"nle", "to-nle", "--help"},
		{"transcribe", "stt", "--help"},
		{"transcript", "load-transcript", "--help"},
		{"transcript", "word-align", "--help"},
	} {
		if _, errOut, code := runCLI(t, args...); code != 0 {
			t.Fatalf("expected alias %v to resolve, got %d stderr=%s", args, code, errOut)
		}
	}
}

func TestValidationErrorsUseStructuredJSON(t *testing.T) {
	_, errOut, code := runCLI(t, "transcript", "create", "--provider", "nope", "--format", "json")
	if code != 4 {
		t.Fatalf("expected exit code 4, got %d stderr=%s", code, errOut)
	}
	for _, want := range []string{`"ok": false`, `"schema_version": "vflow-error/v1"`, `"code": "INVALID_ENUM"`} {
		if !strings.Contains(errOut, want) {
			t.Fatalf("stderr missing %s in:\n%s", want, errOut)
		}
	}
}
