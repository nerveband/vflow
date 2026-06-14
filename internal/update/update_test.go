package update

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCheckReportsNoReleaseOnGitHub404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	report, err := Check(context.Background(), Options{MetadataURL: server.URL, Current: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "no_release" {
		t.Fatalf("status = %q", report.Status)
	}
}

func TestCheckSelectsCurrentPlatformAsset(t *testing.T) {
	assetName := fmt.Sprintf("vflow_Darwin_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(fmt.Sprintf(`{"tag_name":"v1.2.3","assets":[{"name":%q,"browser_download_url":"%s/asset"},{"name":"checksums.txt","browser_download_url":"%s/checksums"}]}`, assetName, "https://example.test", "https://example.test")))
	}))
	defer server.Close()

	report, err := Check(context.Background(), Options{MetadataURL: server.URL, Current: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "update_available" || report.AssetName != assetName || report.ChecksumAsset != "checksums.txt" {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestStageDownloadsAssetToCache(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("archive"))
	}))
	defer server.Close()
	cacheDir := t.TempDir()
	report := Report{Status: "update_available", LatestVersion: "v1.2.3", AssetName: "vflow_Darwin.tar.gz", AssetURL: server.URL}

	got, err := Stage(context.Background(), report, Options{CacheDir: cacheDir})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "staged" || got.StagedPath == "" {
		t.Fatalf("unexpected staged report: %+v", got)
	}
	raw, err := os.ReadFile(filepath.Join(cacheDir, "v1.2.3", "vflow_Darwin.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "archive" {
		t.Fatalf("unexpected staged content: %s", raw)
	}
}
