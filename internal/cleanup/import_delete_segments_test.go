package cleanup

import "testing"

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
