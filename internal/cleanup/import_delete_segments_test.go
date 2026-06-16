package cleanup

import (
	"strings"
	"testing"
)

func TestImportDeleteSegmentsUsesHalfOpenFrames(t *testing.T) {
	raw := []byte(`[{"start":1.0,"end":2.0,"reason":"retake","confidence":0.91}]`)
	edl, err := ImportDeleteSegments(raw, "30/1")
	if err != nil {
		t.Fatal(err)
	}
	if len(edl.DeleteSegments) != 1 {
		t.Fatalf("expected one delete, got %d", len(edl.DeleteSegments))
	}
	got := edl.DeleteSegments[0]
	if got.StartFrame != 30 || got.EndFrame != 60 {
		t.Fatalf("expected [30,60), got [%d,%d)", got.StartFrame, got.EndFrame)
	}
}

func TestImportDeleteSegmentsRejectsInvalidFrameRange(t *testing.T) {
	raw := []byte(`[{"start":2.0,"end":1.0,"reason":"bad","confidence":0.91}]`)
	_, err := ImportDeleteSegments(raw, "30/1")
	if err == nil || !strings.Contains(err.Error(), "end_frame must be greater than start_frame") {
		t.Fatalf("expected frame range validation error, got %v", err)
	}
}

func TestValidateContentEDLRejectsOverlappingDeletes(t *testing.T) {
	err := ValidateContentEDL(ContentEDL{
		Version: "vflow-content-edl/v1",
		Rate:    "30/1",
		DeleteSegments: []DeleteSegment{
			{ID: "del_000001", StartFrame: 30, EndFrame: 60, Confidence: 0.9},
			{ID: "del_000002", StartFrame: 50, EndFrame: 75, Confidence: 0.9},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "overlaps previous delete range") {
		t.Fatalf("expected overlap validation error, got %v", err)
	}
}
