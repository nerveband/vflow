package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
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

func TestStageVerifiesChecksumAndInstallsBinary(t *testing.T) {
	archive := makeTarGz(t, "vflow", []byte("new-binary"))
	sum := sha256.Sum256(archive)
	checksums := []byte(hex.EncodeToString(sum[:]) + "  vflow_Darwin.tar.gz\n")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/asset":
			_, _ = w.Write(archive)
		case "/checksums":
			_, _ = w.Write(checksums)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cacheDir := t.TempDir()
	installDir := t.TempDir()
	existing := filepath.Join(installDir, "vflow")
	if err := os.WriteFile(existing, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	report := Report{
		Status:        "update_available",
		LatestVersion: "v1.2.3",
		AssetName:     "vflow_Darwin.tar.gz",
		AssetURL:      server.URL + "/asset",
		ChecksumAsset: "checksums.txt",
		ChecksumURL:   server.URL + "/checksums",
	}

	got, err := Stage(context.Background(), report, Options{CacheDir: cacheDir, InstallDir: installDir})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "installed" || !got.ChecksumVerified || got.InstalledPath == "" || got.BackupPath == "" {
		t.Fatalf("unexpected install report: %+v", got)
	}
	raw, err := os.ReadFile(existing)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "new-binary" {
		t.Fatalf("unexpected installed content: %s", raw)
	}
	backupRaw, err := os.ReadFile(existing + ".bak")
	if err != nil {
		t.Fatal(err)
	}
	if string(backupRaw) != "old-binary" {
		t.Fatalf("unexpected backup content: %s", backupRaw)
	}
}

func TestStageRejectsChecksumMismatch(t *testing.T) {
	archive := makeTarGz(t, "vflow", []byte("new-binary"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/asset":
			_, _ = w.Write(archive)
		case "/checksums":
			_, _ = w.Write([]byte("0000000000000000000000000000000000000000000000000000000000000000  vflow_Darwin.tar.gz\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	report := Report{
		Status:        "update_available",
		LatestVersion: "v1.2.3",
		AssetName:     "vflow_Darwin.tar.gz",
		AssetURL:      server.URL + "/asset",
		ChecksumAsset: "checksums.txt",
		ChecksumURL:   server.URL + "/checksums",
	}

	if _, err := Stage(context.Background(), report, Options{CacheDir: t.TempDir(), InstallDir: t.TempDir()}); err == nil {
		t.Fatalf("expected checksum mismatch to fail")
	}
}

func makeTarGz(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	gz := gzip.NewWriter(&out)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
