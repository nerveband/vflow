package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMulticamCreateConsumesSyncMapAndWritesCanonicalTimeline(t *testing.T) {
	project := t.TempDir()
	syncMapPath := filepath.Join(project, "decisions", "media-sync-map.json")
	if err := os.MkdirAll(filepath.Dir(syncMapPath), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{
  "version": "vflow-media-sync-map/v1",
  "project_id": "fixture",
  "method": "audio_xcorr_envelope",
  "reference_source_id": "cam_a",
  "sources": [
    {"id":"cam_a","path":"media/cam-a.mp4","offset_from_reference_seconds":0,"confidence":0.99},
    {"id":"cam_b","path":"media/cam-b.mp4","offset_from_reference_seconds":1.25,"confidence":0.91}
  ]
}`)
	if err := os.WriteFile(syncMapPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	out, errOut, code := runCLI(t, "multicam", "create", "--project", project, "--sync-map", syncMapPath, "--duration-frames", "120", "--commit", "--format", "json", "--format-error", "json")
	if code != 0 {
		t.Fatalf("multicam create failed: code=%d stdout=%s stderr=%s", code, out, errOut)
	}
	if !strings.Contains(out, `"vflow-timeline/v1"`) || !strings.Contains(out, `"multicam_groups"`) || !strings.Contains(out, `"active_angle_spans"`) {
		t.Fatalf("multicam output missing canonical metadata: %s", out)
	}
	timelinePath := filepath.Join(project, "timeline", "multicam-timeline.json")
	timelineRaw, err := os.ReadFile(timelinePath)
	if err != nil {
		t.Fatalf("expected multicam timeline artifact: %v", err)
	}
	if !strings.Contains(string(timelineRaw), `"track_type": "video"`) || !strings.Contains(string(timelineRaw), `"sync_map_ref"`) {
		t.Fatalf("timeline artifact missing stacked tracks or sync map refs: %s", timelineRaw)
	}
}
