package update

import (
	"context"
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
	ChecksumRequired bool   `json:"checksum_required"`
	ChecksumVerified bool   `json:"checksum_verified"`
	StagedPath       string `json:"staged_path,omitempty"`
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, report.AssetURL, nil)
	if err != nil {
		return report, err
	}
	req.Header.Set("User-Agent", "vflow-upgrade")
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		return report, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return report, fmt.Errorf("asset download returned %s: %s", resp.Status, compact(raw))
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
	if _, err := io.Copy(tmp, resp.Body); err != nil {
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
	report.Hint = "Asset staged only; verify checksums and install through release tooling before replacing a production binary."
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

func IsHTTPURL(value string) bool {
	u, err := url.Parse(value)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
