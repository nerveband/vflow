package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNLEImportWritesArtifactAndDiffDeliversReviewHTML(t *testing.T) {
	project := t.TempDir()
	input := filepath.Join(project, "timeline.fcpxml")
	if err := os.WriteFile(input, []byte(cliRoundtripFCPXML), 0o644); err != nil {
		t.Fatal(err)
	}

	out, errOut, code := runCLI(t, "nle", "import", "--project", project, "--input", input, "--commit", "--format", "json")
	if code != 0 {
		t.Fatalf("nle import failed: code=%d stdout=%s stderr=%s", code, out, errOut)
	}
	importPath := filepath.Join(project, "imports", "nle-import.json")
	if _, err := os.Stat(importPath); err != nil {
		t.Fatalf("expected import artifact: %v", err)
	}
	importRaw, err := os.ReadFile(importPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(importRaw), `"color_grade"`) || !strings.Contains(string(importRaw), `"clip_trim"`) {
		t.Fatalf("import artifact missing parsed changes: %s", importRaw)
	}

	reviewPath := filepath.Join(project, "review", "roundtrip-review.html")
	out, errOut, code = runCLI(t, "nle", "diff", "--project", project, "--import", importPath, "--deliver", "file:"+reviewPath, "--format", "json")
	if code != 0 {
		t.Fatalf("nle diff failed: code=%d stdout=%s stderr=%s", code, out, errOut)
	}
	if !strings.Contains(out, "roundtrip-review.html") {
		t.Fatalf("diff output missing review artifact: %s", out)
	}
	if !strings.Contains(out, `"safe_merge"`) || !strings.Contains(out, `"needs_review"`) || !strings.Contains(out, `"blocked"`) {
		t.Fatalf("diff output missing classification buckets: %s", out)
	}
	raw, err := os.ReadFile(reviewPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "Roundtrip Review") || !strings.Contains(string(raw), "color_grade") {
		t.Fatalf("review HTML missing expected sections: %s", raw)
	}
}

func TestNLEImportResolvesProjectRelativeInput(t *testing.T) {
	project := t.TempDir()
	input := filepath.Join(project, "exports", "timeline.fcpxml")
	if err := os.MkdirAll(filepath.Dir(input), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(input, []byte(cliRoundtripFCPXML), 0o644); err != nil {
		t.Fatal(err)
	}

	out, errOut, code := runCLI(t, "nle", "import", "--project", project, "--input", filepath.Join("exports", "timeline.fcpxml"), "--commit", "--format", "json", "--format-error", "json")
	if code != 0 {
		t.Fatalf("nle import failed: code=%d stdout=%s stderr=%s", code, out, errOut)
	}
	if !strings.Contains(out, `"format": "fcpxml"`) || !strings.Contains(out, `"status": "parsed"`) {
		t.Fatalf("unexpected nle import output: %s", out)
	}
	importRaw, err := os.ReadFile(filepath.Join(project, "imports", "nle-import.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(importRaw), filepath.ToSlash(filepath.Join(project, "exports", "timeline.fcpxml"))) {
		t.Fatalf("import artifact should record resolved input path: %s", importRaw)
	}
}

func TestNLEApplyCommitRefusesBlockedChanges(t *testing.T) {
	project := t.TempDir()
	input := filepath.Join(project, "timeline.fcpxml")
	if err := os.WriteFile(input, []byte(cliRoundtripFCPXML), 0o644); err != nil {
		t.Fatal(err)
	}
	_, errOut, code := runCLI(t, "nle", "import", "--project", project, "--input", input, "--commit", "--format", "json")
	if code != 0 {
		t.Fatalf("nle import failed: code=%d stderr=%s", code, errOut)
	}

	importPath := filepath.Join(project, "imports", "nle-import.json")
	_, errOut, code = runCLI(t, "nle", "apply", "--input", importPath, "--commit", "--format", "json", "--format-error", "json")
	if code != 5 {
		t.Fatalf("expected safety refusal, got code=%d stderr=%s", code, errOut)
	}
	if !strings.Contains(errOut, "blocked NLE changes") || !strings.Contains(errOut, `"ok": false`) {
		t.Fatalf("expected structured blocked error: %s", errOut)
	}
}

func TestNLEApplyCommitWritesAppliedChangesArtifact(t *testing.T) {
	project := t.TempDir()
	input := filepath.Join(project, "safe-import.json")
	raw := []byte(`{
  "version": "vflow-nle-import/v1",
  "status": "parsed",
  "input": "timeline.fcpxml",
  "format": "fcpxml",
  "bytes": 123,
  "changes": [
    {"id":"change_1","type":"clip_trim","segment_id":"seg_A","description":"clip timing changed","confidence":0.9}
  ]
}`)
	if err := os.WriteFile(input, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	out, errOut, code := runCLI(t, "nle", "apply", "--project", project, "--input", input, "--commit", "--format", "json")
	if code != 0 {
		t.Fatalf("nle apply failed: code=%d stdout=%s stderr=%s", code, out, errOut)
	}
	artifact := filepath.Join(project, "imports", "applied-nle-changes.json")
	if _, err := os.Stat(artifact); err != nil {
		t.Fatalf("expected applied changes artifact: %v", err)
	}
	appliedRaw, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(appliedRaw), `"vflow-nle-apply/v1"`) || !strings.Contains(out, "applied-nle-changes.json") {
		t.Fatalf("applied artifact not reported correctly: stdout=%s artifact=%s", out, appliedRaw)
	}
}

func TestNLEAcceptThenApplyMergesAcceptedNeedsReview(t *testing.T) {
	project := t.TempDir()
	input := filepath.Join(project, "review-import.json")
	raw := []byte(`{
  "version": "vflow-nle-import/v1",
  "status": "parsed",
  "input": "timeline.fcpxml",
  "format": "fcpxml",
  "bytes": 123,
  "changes": [
    {"id":"change_safe","type":"clip_trim","segment_id":"seg_A","description":"clip timing changed","confidence":0.9},
    {"id":"change_review","type":"title_card","segment_id":"seg_A","description":"title changed","confidence":0.85}
  ]
}`)
	if err := os.WriteFile(input, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	acceptedPath := filepath.Join(project, "imports", "accepted-nle-changes.json")
	out, errOut, code := runCLI(t, "nle", "accept", "--project", project, "--import", input, "--all-needs-review", "--reviewer", "operator", "--output", acceptedPath, "--commit", "--format", "json")
	if code != 0 {
		t.Fatalf("nle accept failed: code=%d stdout=%s stderr=%s", code, out, errOut)
	}
	if !strings.Contains(out, `"accepted_needs_review"`) || !strings.Contains(out, `"change_review"`) {
		t.Fatalf("accept output missing reviewed change: %s", out)
	}

	out, errOut, code = runCLI(t, "nle", "apply", "--project", project, "--input", acceptedPath, "--commit", "--format", "json", "--format-error", "json")
	if code != 0 {
		t.Fatalf("nle apply accepted failed: code=%d stdout=%s stderr=%s", code, out, errOut)
	}
	if !strings.Contains(out, `"status": "applied"`) || !strings.Contains(out, `"title_card"`) {
		t.Fatalf("apply output missing accepted needs-review change: %s", out)
	}
}

func TestNLEDiffCanReadRawTimelineInput(t *testing.T) {
	project := t.TempDir()
	input := filepath.Join(project, "timeline.fcpxml")
	if err := os.WriteFile(input, []byte(cliRoundtripFCPXML), 0o644); err != nil {
		t.Fatal(err)
	}
	out, errOut, code := runCLI(t, "nle", "diff", "--project", project, "--import", input, "--format", "json")
	if code != 0 {
		t.Fatalf("nle diff failed: code=%d stdout=%s stderr=%s", code, out, errOut)
	}
	var envelope struct {
		Data struct {
			SafeMerge   []any `json:"safe_merge"`
			NeedsReview []any `json:"needs_review"`
			Blocked     []any `json:"blocked"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("invalid diff json: %v\n%s", err, out)
	}
	if len(envelope.Data.SafeMerge) == 0 || len(envelope.Data.NeedsReview) == 0 || len(envelope.Data.Blocked) == 0 {
		t.Fatalf("expected all diff buckets from raw timeline: %+v", envelope.Data)
	}
}

func TestNLEExportRejectsUnsupportedTarget(t *testing.T) {
	project := t.TempDir()
	_, errOut, code := runCLI(t, "nle", "export", "--project", project, "--target", "fcpxm", "--format", "json", "--format-error", "json")
	if code != 4 {
		t.Fatalf("expected validation failure, got code=%d stderr=%s", code, errOut)
	}
	if !strings.Contains(errOut, `"code": "INVALID_ENUM"`) || !strings.Contains(errOut, "unsupported NLE export target") {
		t.Fatalf("unexpected target error: %s", errOut)
	}
}

const cliRoundtripFCPXML = `<?xml version="1.0" encoding="UTF-8"?>
<fcpxml version="1.11">
  <library><event name="Roundtrip"><project name="vflow"><sequence duration="120/24s"><spine>
    <asset-clip name="seg_A" offset="0/24s" start="12/24s" duration="48/24s">
      <marker start="0s" value="producer note" note="vflow:segment-id=seg_A reviewed=yes"/>
      <adjust-volume amount="-3dB"/>
      <adjust-transform position="12 4" scale="1.05 1.05"/>
      <filter-video name="Lumetri Color"/>
    </asset-clip>
    <title name="Lower Third" offset="24/24s" duration="24/24s"><text>Executive Director</text></title>
  </spine></sequence></project></event></library>
</fcpxml>`
