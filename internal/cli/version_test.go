package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestVersionEmitsJSONEnvelope(t *testing.T) {
	out, errOut, code := runCLI(t, "version", "--format", "json")
	if code != 0 {
		t.Fatalf("expected success, got %d stderr=%s", code, errOut)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("expected JSON output, got %q: %v", out, err)
	}
	if got["ok"] != true {
		t.Fatalf("expected ok=true, got %#v", got["ok"])
	}
	if got["schema_version"] != "vflow-response/v1" {
		t.Fatalf("unexpected schema_version: %#v", got["schema_version"])
	}
	if got["command"] != "version" {
		t.Fatalf("unexpected command: %#v", got["command"])
	}
}

func TestUpgradeDryRunReportsNoRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	out, errOut, code := runCLI(t, "upgrade", "--metadata-url", server.URL, "--format", "json", "--format-error", "json")
	if code != 0 {
		t.Fatalf("expected success, got %d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, `"status": "no_release"`) {
		t.Fatalf("expected no_release: %s", out)
	}
}

func TestUpgradeCommitStagesMatchingAsset(t *testing.T) {
	var server *httptest.Server
	assetName := fmt.Sprintf("vflow_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest":
			_, _ = w.Write([]byte(fmt.Sprintf(`{"tag_name":"v1.2.3","assets":[{"name":%q,"browser_download_url":"%s/asset"},{"name":"checksums.txt","browser_download_url":"%s/checksums"}]}`, assetName, server.URL, server.URL)))
		case "/asset":
			_, _ = w.Write([]byte("archive"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	cacheDir := filepath.Join(t.TempDir(), "cache")

	out, errOut, code := runCLI(t, "upgrade", "--metadata-url", server.URL+"/latest", "--cache-dir", cacheDir, "--commit", "--format", "json", "--format-error", "json")
	if code != 0 {
		t.Fatalf("expected success, got %d stdout=%s stderr=%s", code, out, errOut)
	}
	for _, want := range []string{`"status": "staged"`, assetName, `"checksum_asset": "checksums.txt"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %s in:\n%s", want, out)
		}
	}
}
