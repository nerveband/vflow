package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	vcleanup "github.com/nerveband/vflow/internal/cleanup"
)

func TestCleanupReviewWritesHTMLDelivery(t *testing.T) {
	project := t.TempDir()
	edl := vcleanup.ContentEDL{
		Version: "vflow-content-edl/v1",
		Rate:    "30/1",
		DeleteSegments: []vcleanup.DeleteSegment{
			{ID: "del_000001", StartFrame: 30, EndFrame: 45, Reason: "retake", Confidence: 0.91},
		},
	}
	if err := vcleanup.WriteContentEDL(project, edl); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(project, "review", "cleanup-review.html")

	out, errOut, code := runCLI(t, "cleanup", "review", "--project", project, "--deliver", "file:"+outPath, "--commit", "--format", "json")
	if code != 0 {
		t.Fatalf("cleanup review failed: code=%d stdout=%s stderr=%s", code, out, errOut)
	}
	if !strings.Contains(out, `"status": "reviewed"`) || !strings.Contains(out, "cleanup-review.html") {
		t.Fatalf("unexpected output: %s", out)
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "retake") || !strings.Contains(string(raw), "del_000001") {
		t.Fatalf("review HTML missing decision details: %s", raw)
	}
}
