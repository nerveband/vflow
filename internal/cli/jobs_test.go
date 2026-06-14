package cli

import (
	"strings"
	"testing"

	vjobs "github.com/nerveband/vflow/internal/jobs"
)

func TestJobsListGetAndResumeUseProjectLedger(t *testing.T) {
	project := t.TempDir()
	record := vjobs.NewRecord(project, "render preview", "succeeded", true)
	record.JobID = "job_test"
	if err := vjobs.Write(project, record); err != nil {
		t.Fatal(err)
	}

	out, errOut, code := runCLI(t, "jobs", "list", "--project", project, "--format", "json")
	if code != 0 {
		t.Fatalf("jobs list failed: code=%d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, "job_test") {
		t.Fatalf("jobs list missing job: %s", out)
	}

	out, errOut, code = runCLI(t, "jobs", "get", "job_test", "--project", project, "--format", "json")
	if code != 0 {
		t.Fatalf("jobs get failed: code=%d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, `"command": "render preview"`) {
		t.Fatalf("jobs get missing command: %s", out)
	}

	out, errOut, code = runCLI(t, "jobs", "resume", "job_test", "--project", project, "--format", "json")
	if code != 0 {
		t.Fatalf("jobs resume failed: code=%d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, `"status": "succeeded"`) {
		t.Fatalf("jobs resume should report existing status: %s", out)
	}
}
