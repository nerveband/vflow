package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Options struct {
	Repo        string
	MetadataURL string
	CacheDir    string
	InstallDir  string
	Current     string
	Commit      string
}

type Report struct {
	Version          string `json:"version"`
	Status           string `json:"status"`
	Repo             string `json:"repo"`
	MetadataURL      string `json:"metadata_url"`
	CurrentVersion   string `json:"current_version"`
	LatestVersion    string `json:"latest_version,omitempty"`
	AssetName        string `json:"asset_name,omitempty"`
	AssetURL         string `json:"asset_url,omitempty"`
	ChecksumAsset    string `json:"checksum_asset,omitempty"`
	ChecksumURL      string `json:"checksum_url,omitempty"`
	ChecksumRequired bool   `json:"checksum_required"`
	ChecksumVerified bool   `json:"checksum_verified"`
	StagedPath       string `json:"staged_path,omitempty"`
	InstalledPath    string `json:"installed_path,omitempty"`
	BackupPath       string `json:"backup_path,omitempty"`
	Hint             string `json:"hint,omitempty"`
}

type githubRelease struct {
	TagName string      `json:"tag_name"`
	HTMLURL string      `json:"html_url"`
	Assets  []assetInfo `json:"assets"`
}

type assetInfo struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

func Check(ctx context.Context, opts Options) (Report, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	opts = normalizeOptions(opts)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, opts.MetadataURL, nil)
	if err != nil {
		return Report{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "vflow-upgrade")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return Report{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	report := Report{
		Version:          "vflow-upgrade-check/v1",
		Status:           "checked",
		Repo:             opts.Repo,
		MetadataURL:      opts.MetadataURL,
		CurrentVersion:   opts.Current,
		ChecksumRequired: true,
	}
	if resp.StatusCode == http.StatusNotFound {
		report.Status = "no_release"
		report.Hint = "Create a GitHub release with GoReleaser assets and checksums before using upgrade --commit."
		return report, nil
	}
	if resp.StatusCode >= 300 {
		return Report{}, fmt.Errorf("release metadata returned %s: %s", resp.Status, compact(raw))
	}
	var release githubRelease
	if err := json.Unmarshal(raw, &release); err != nil {
		return Report{}, err
	}
	report.LatestVersion = release.TagName
	if sameVersion(opts.Current, release.TagName) {
		report.Status = "current"
		return report, nil
	}
	asset, checksum := selectAssets(release)
	if checksum.Name != "" {
		report.ChecksumAsset = checksum.Name
		report.ChecksumURL = checksum.BrowserDownloadURL
	}
	if asset.Name == "" {
		report.Status = "release_missing_asset"
		report.Hint = "Release exists, but no asset matched this OS/arch."
		return report, nil
	}
	report.Status = "update_available"
	report.AssetName = asset.Name
	report.AssetURL = asset.BrowserDownloadURL
	return report, nil
}

func Stage(ctx context.Context, report Report, opts Options) (Report, error) {
	if report.Status != "update_available" {
		return report, nil
	}
	if report.AssetURL == "" {
		return report, fmt.Errorf("upgrade asset URL is empty")
	}
	opts = normalizeOptions(opts)
	if opts.CacheDir == "" {
		opts.CacheDir = filepath.Join(os.Getenv("HOME"), ".vflow", "cache", "upgrades")
	}
	target := filepath.Join(opts.CacheDir, report.LatestVersion, report.AssetName)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return report, err
	}
	assetRaw, err := download(ctx, report.AssetURL)
	if err != nil {
		return report, err
	}
	if report.ChecksumURL != "" {
		checksumRaw, err := download(ctx, report.ChecksumURL)
		if err != nil {
			return report, err
		}
		if err := verifyChecksum(report.AssetName, assetRaw, checksumRaw); err != nil {
			return report, err
		}
		report.ChecksumVerified = true
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".vflow-upgrade-*")
	if err != nil {
		return report, err
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(assetRaw); err != nil {
		_ = tmp.Close()
		return report, err
	}
	if err := tmp.Close(); err != nil {
		return report, err
	}
	if err := os.Rename(tmpPath, target); err != nil {
		return report, err
	}
	committed = true
	report.Status = "staged"
	report.StagedPath = filepath.ToSlash(target)
	report.Hint = "Asset staged only; pass --install-dir with --commit to install the verified binary."
	if strings.TrimSpace(opts.InstallDir) != "" {
		report, err = Install(report, opts)
		if err != nil {
			return report, err
		}
	}
	return report, nil
}

func Install(report Report, opts Options) (Report, error) {
	if report.StagedPath == "" {
		return report, fmt.Errorf("staged path is empty")
	}
	installDir := strings.TrimSpace(opts.InstallDir)
	if installDir == "" {
		return report, fmt.Errorf("install dir is required")
	}
	binary, err := extractBinary(report.StagedPath)
	if err != nil {
		return report, err
	}
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return report, err
	}
	name := "vflow"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	target := filepath.Join(installDir, name)
	tmp, err := os.CreateTemp(installDir, ".vflow-install-*")
	if err != nil {
		return report, err
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(binary); err != nil {
		_ = tmp.Close()
		return report, err
	}
	if err := tmp.Chmod(0o755); err != nil {
		_ = tmp.Close()
		return report, err
	}
	if err := tmp.Close(); err != nil {
		return report, err
	}
	if _, err := os.Stat(target); err == nil {
		backup := target + ".bak"
		_ = os.Remove(backup)
		if err := os.Rename(target, backup); err != nil {
			return report, err
		}
		report.BackupPath = filepath.ToSlash(backup)
	}
	if err := os.Rename(tmpPath, target); err != nil {
		return report, err
	}
	committed = true
	report.Status = "installed"
	report.InstalledPath = filepath.ToSlash(target)
	report.Hint = "Installed verified vflow binary. Ensure install_dir is on PATH."
	return report, nil
}

func normalizeOptions(opts Options) Options {
	if opts.Repo == "" {
		opts.Repo = "github.com/nerveband/vflow"
	}
	if opts.Current == "" {
		opts.Current = "dev"
	}
	if opts.MetadataURL == "" {
		if env := strings.TrimSpace(os.Getenv("VFLOW_UPGRADE_METADATA_URL")); env != "" {
			opts.MetadataURL = env
		} else {
			opts.MetadataURL = githubLatestReleaseURL(opts.Repo)
		}
	}
	return opts
}

func githubLatestReleaseURL(repo string) string {
	repo = strings.TrimPrefix(repo, "https://")
	repo = strings.TrimPrefix(repo, "http://")
	repo = strings.TrimPrefix(repo, "github.com/")
	repo = strings.Trim(repo, "/")
	return "https://api.github.com/repos/" + repo + "/releases/latest"
}

func selectAssets(release githubRelease) (asset, checksum assetInfo) {
	osName := strings.ToLower(runtime.GOOS)
	arch := strings.ToLower(runtime.GOARCH)
	for _, candidate := range release.Assets {
		name := strings.ToLower(candidate.Name)
		if strings.Contains(name, "checksum") {
			checksum = candidate
			continue
		}
		if strings.Contains(name, osName) && strings.Contains(name, arch) {
			asset = candidate
		}
	}
	return asset, checksum
}

func sameVersion(current, latest string) bool {
	current = strings.TrimSpace(current)
	latest = strings.TrimSpace(latest)
	if current == "" || current == "dev" || latest == "" {
		return false
	}
	return current == latest || "v"+current == latest || current == strings.TrimPrefix(latest, "v")
}

func compact(raw []byte) string {
	text := strings.Join(strings.Fields(string(raw)), " ")
	if len(text) > 300 {
		return text[:300] + "..."
	}
	return text
}

func download(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "vflow-upgrade")
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download returned %s: %s", resp.Status, compact(raw))
	}
	return raw, nil
}

func verifyChecksum(assetName string, assetRaw, checksumRaw []byte) error {
	sum := sha256.Sum256(assetRaw)
	got := hex.EncodeToString(sum[:])
	for _, line := range strings.Split(string(checksumRaw), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[len(fields)-1], "*")
		if filepath.Base(name) == filepath.Base(assetName) {
			want := strings.ToLower(fields[0])
			if got != want {
				return fmt.Errorf("checksum mismatch for %s", assetName)
			}
			return nil
		}
	}
	return fmt.Errorf("checksum for %s not found", assetName)
}

func extractBinary(archivePath string) ([]byte, error) {
	name := strings.ToLower(filepath.Base(archivePath))
	switch {
	case strings.HasSuffix(name, ".tar.gz"):
		return extractTarGzBinary(archivePath)
	case strings.HasSuffix(name, ".zip"):
		return extractZipBinary(archivePath)
	default:
		return nil, fmt.Errorf("unsupported release archive %s", archivePath)
	}
}

func extractTarGzBinary(archivePath string) ([]byte, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	reader := tar.NewReader(gz)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if header.Typeflag == tar.TypeReg && filepath.Base(header.Name) == "vflow" {
			return io.ReadAll(reader)
		}
	}
	return nil, fmt.Errorf("vflow binary not found in %s", archivePath)
}

func extractZipBinary(archivePath string) ([]byte, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	for _, file := range reader.File {
		base := filepath.Base(file.Name)
		if base != "vflow" && base != "vflow.exe" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		raw, readErr := io.ReadAll(rc)
		closeErr := rc.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		return raw, nil
	}
	return nil, fmt.Errorf("vflow binary not found in %s", archivePath)
}

func IsHTTPURL(value string) bool {
	u, err := url.Parse(value)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
