package session

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	vframing "github.com/nerveband/vflow/internal/framing"
)

func TestSessionLifecycleLocalhostStatusAndShutdown(t *testing.T) {
	dir := testProject(t)
	srv, res, verr := Start(context.Background(), Options{
		ProjectPath:   dir,
		Source:        "media/source.mp4",
		Listen:        "127.0.0.1:0",
		Timeout:       time.Minute,
		ShutdownToken: "secret",
	})
	if verr != nil {
		t.Fatal(verr)
	}
	defer srv.Shutdown(context.Background())

	if !strings.HasPrefix(res.URL, "http://127.0.0.1:") || res.Port == 0 {
		t.Fatalf("expected localhost high-port URL, got %+v", res)
	}
	if !res.ShutdownTokenPresent || !strings.Contains(res.ShutdownURL, "token=secret") {
		t.Fatalf("expected token metadata in result: %+v", res)
	}
	if _, err := os.Stat(filepath.Join(dir, "tmp", "sessions", res.SessionID+".json")); err != nil {
		t.Fatalf("expected session status file: %v", err)
	}
	httpGetJSON(t, res.HealthURL, http.StatusOK)
	httpGetJSON(t, res.StatusURL, http.StatusOK)

	req, err := http.NewRequest(http.MethodPost, strings.Split(res.ShutdownURL, "?")[0], nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected missing token to be forbidden, got %d", resp.StatusCode)
	}

	req, err = http.NewRequest(http.MethodPost, res.ShutdownURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected shutdown success, got %d", resp.StatusCode)
	}
	select {
	case <-srv.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("server did not shut down")
	}
}

func TestSessionRejectsNonLocalhostAndAbsoluteSource(t *testing.T) {
	dir := testProject(t)
	if _, _, verr := Start(context.Background(), Options{ProjectPath: dir, Listen: "0.0.0.0:0"}); verr == nil || verr.Code != "CALIBRATE_LISTEN_NOT_LOCALHOST" {
		t.Fatalf("expected localhost validation error, got %v", verr)
	}
	if _, _, verr := Start(context.Background(), Options{ProjectPath: dir, Listen: "127.0.0.1:0", Source: filepath.Join(dir, "media", "source.mp4")}); verr == nil || verr.Code != "CALIBRATE_SOURCE_OUTSIDE_PROJECT" {
		t.Fatalf("expected source validation error, got %v", verr)
	}
}

func TestSessionPresetAPIValidationCommitGateAndRoundTrip(t *testing.T) {
	dir := testProject(t)
	srv, res, verr := Start(context.Background(), Options{ProjectPath: dir, Source: "media/source.mp4", Listen: "127.0.0.1:0", Timeout: time.Minute})
	if verr != nil {
		t.Fatal(verr)
	}
	defer srv.Shutdown(context.Background())

	bad := []byte(`{"version":"vflow-framing-presets/v1","source_width":3840,"source_height":2160,"target_aspect":"16:9","presets":[]}`)
	resp := httpPostJSON(t, res.URL+"api/presets", bad)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected invalid presets to fail, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	presets := vframing.Presets{
		Version: "vflow-framing-presets/v1", SourceWidth: 3840, SourceHeight: 2160, TargetAspect: "16:9",
		Presets: []vframing.Preset{
			{ID: "wide", Label: "Wide", Type: "wide", CropPX: vframing.Rect{X: 0, Y: 0, W: 3840, H: 2160}},
			{ID: "speaker_new", Label: "Speaker New", Type: "speaker", Locked: true, CropPX: vframing.Rect{X: 100, Y: 100, W: 1280, H: 720}},
		},
	}
	raw, _ := json.Marshal(presets)
	resp = httpPostJSON(t, res.URL+"api/presets", raw)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected presets accepted, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	resp = httpPostJSON(t, res.URL+"api/commit?commit=true", nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected dry-run session commit rejection, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()
	onDisk, err := vframing.ReadPresets(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(onDisk.Presets) != 1 {
		t.Fatalf("dry-run API should not write presets, got %#v", onDisk.Presets)
	}

	_ = srv.Shutdown(context.Background())
	<-srv.Done()

	srv, res, verr = Start(context.Background(), Options{ProjectPath: dir, Source: "media/source.mp4", Listen: "127.0.0.1:0", Timeout: time.Minute, CommitEnabled: true})
	if verr != nil {
		t.Fatal(verr)
	}
	defer srv.Shutdown(context.Background())
	resp = httpPostJSON(t, res.URL+"api/presets", raw)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected commit session presets accepted, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()
	resp = httpPostJSON(t, res.URL+"api/commit?commit=true", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected committed write, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()
	onDisk, err = vframing.ReadPresets(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(onDisk.Presets) != 2 || onDisk.Presets[1].ID != "speaker_new" || !onDisk.Presets[1].Locked {
		t.Fatalf("preset roundtrip failed: %#v", onDisk.Presets)
	}
}

func httpGetJSON(t *testing.T, url string, want int) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != want {
		t.Fatalf("GET %s status=%d want=%d", url, resp.StatusCode, want)
	}
}

func httpPostJSON(t *testing.T, url string, body []byte) *http.Response {
	t.Helper()
	var r *bytes.Reader
	if body == nil {
		r = bytes.NewReader([]byte(`{}`))
	} else {
		r = bytes.NewReader(body)
	}
	resp, err := http.Post(url, "application/json", r)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func testProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustWriteJSON(t, filepath.Join(dir, "project.json"), map[string]any{
		"version":    "vflow-project/v1",
		"id":         "calibrate_test",
		"root":       dir,
		"created_at": "2026-06-16T00:00:00Z",
		"updated_at": "2026-06-16T00:00:00Z",
	})
	mustWriteJSON(t, filepath.Join(dir, "calibration", "framing-presets.json"), map[string]any{
		"version":       "vflow-framing-presets/v1",
		"source_width":  3840,
		"source_height": 2160,
		"target_aspect": "16:9",
		"presets": []map[string]any{{
			"id": "wide", "label": "Wide", "type": "wide",
			"crop_px": map[string]any{"x": 0, "y": 0, "w": 3840, "h": 2160},
		}},
	})
	mustWriteJSON(t, filepath.Join(dir, "calibration", "speaker-map.json"), map[string]any{
		"version": "vflow-speaker-map/v1",
		"map":     map[string]string{},
	})
	mustWriteJSON(t, filepath.Join(dir, "policy", "framing-policy.json"), vframing.DefaultPolicy())
	if err := os.MkdirAll(filepath.Join(dir, "media"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "media", "source.mp4"), []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func mustWriteJSON(t *testing.T, file string, value any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
