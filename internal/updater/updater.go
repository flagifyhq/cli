// Package updater implements the pure, cobra-free logic behind `flagify update`:
// querying GitHub Releases, comparing semver, detecting how the binary was
// installed, and (in install.go) verifying + atomically replacing the binary.
// It never touches the Flagify API or any CLI state — it only talks to the
// public GitHub REST API and Releases CDN.
package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

// Release and Asset carry only the fields the updater needs from the GitHub
// releases payload; every other field is ignored via partial unmarshal.
type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

// Asset is a single downloadable file attached to a release.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// LatestReleaseURL is the GitHub endpoint queried for the newest CLI release.
// It is a var (not a const) so tests can point it at an httptest server.
var LatestReleaseURL = "https://api.github.com/repos/flagifyhq/cli/releases/latest"

// DefaultHTTPClient bounds the GitHub check to 10s — the same timeout used by
// internal/api/client.go.
var DefaultHTTPClient = &http.Client{Timeout: 10 * time.Second}

// Typed errors let cli/cmd/update.go distinguish failure modes with errors.Is
// and map each to a specific, actionable message.
var (
	ErrReleaseNotFound    = errors.New("release not found")
	ErrRateLimited        = errors.New("github api rate limited")
	ErrMalformedRelease   = errors.New("malformed release response")
	ErrNoAssetForPlatform = errors.New("no release asset for platform")
)

// LatestRelease fetches the newest published CLI release from GitHub. A nil
// httpClient falls back to DefaultHTTPClient. GitHub rejects requests without a
// User-Agent, so one is always sent.
func LatestRelease(ctx context.Context, httpClient *http.Client) (*Release, error) {
	if httpClient == nil {
		httpClient = DefaultHTTPClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, LatestReleaseURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "flagify-cli")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, ErrReleaseNotFound
	case resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0":
		return nil, ErrRateLimited
	case resp.StatusCode == http.StatusTooManyRequests:
		// Secondary rate limit / abuse detection — treat like the primary limit.
		return nil, ErrRateLimited
	case resp.StatusCode >= 400:
		return nil, fmt.Errorf("github returned status %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformedRelease, err)
	}
	if release.TagName == "" {
		return nil, ErrMalformedRelease
	}
	return &release, nil
}

// CompareVersions normalizes both strings to vX.Y.Z, validates them, and
// returns semver.Compare(current, latest): negative if current < latest, 0 if
// equal, positive if current > latest.
func CompareVersions(current, latestTag string) (int, error) {
	normalizedCurrent := normalizeVersion(current)
	normalizedLatest := normalizeVersion(latestTag)
	if !semver.IsValid(normalizedCurrent) {
		return 0, fmt.Errorf("invalid current version %q", current)
	}
	if !semver.IsValid(normalizedLatest) {
		return 0, fmt.Errorf("invalid latest version %q", latestTag)
	}
	return semver.Compare(normalizedCurrent, normalizedLatest), nil
}

// normalizeVersion prefixes a leading "v" when missing so semver accepts it.
func normalizeVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return version
	}
	if !strings.HasPrefix(version, "v") {
		return "v" + version
	}
	return version
}

// InstallMethod is how the running binary was installed, which decides whether
// `flagify update` self-replaces (direct) or prints a manager command.
type InstallMethod int

const (
	MethodDirect InstallMethod = iota
	MethodHomebrew
	MethodNPM
	MethodGoInstall
)

// Label is the human-readable name of the install method.
func (m InstallMethod) Label() string {
	switch m {
	case MethodHomebrew:
		return "Homebrew"
	case MethodNPM:
		return "npm"
	case MethodGoInstall:
		return "go install"
	default:
		return "direct download"
	}
}

// UpgradeCommand is the exact command a user runs to update a manager-managed
// install. It is not meaningful for MethodDirect (never called for that case).
func (m InstallMethod) UpgradeCommand() string {
	switch m {
	case MethodHomebrew:
		return "brew upgrade flagify"
	case MethodNPM:
		return "npm update -g @flagify/cli"
	case MethodGoInstall:
		return "go install github.com/flagifyhq/cli/cmd/flagify@latest"
	default:
		return ""
	}
}

// DetectInstallMethod is a pure path heuristic over the resolved executable
// path. Fixed priority (first match wins): Homebrew → npm → go install →
// direct. Homebrew is checked first because its path never collides with the
// others.
func DetectInstallMethod(resolvedExecPath string) InstallMethod {
	lower := strings.ToLower(resolvedExecPath)

	// Covers macOS Intel (/usr/local/Cellar/), Apple Silicon
	// (/opt/homebrew/Cellar/) and Linuxbrew (/home/linuxbrew/.linuxbrew/Cellar/).
	if strings.Contains(lower, "/cellar/") {
		return MethodHomebrew
	}
	if strings.Contains(resolvedExecPath, "node_modules") {
		return MethodNPM
	}
	if isUnderGoBin(resolvedExecPath) {
		return MethodGoInstall
	}
	return MethodDirect
}

// isUnderGoBin reports whether the path lives under $GOBIN, or $GOPATH/bin
// (falling back to ~/go/bin when $GOPATH is unset) — the destinations
// `go install` writes to.
func isUnderGoBin(execPath string) bool {
	if gobin := os.Getenv("GOBIN"); gobin != "" {
		if pathHasPrefix(execPath, gobin) {
			return true
		}
	}

	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return false
		}
		gopath = filepath.Join(home, "go")
	}
	for _, entry := range filepath.SplitList(gopath) {
		if entry == "" {
			continue
		}
		if pathHasPrefix(execPath, filepath.Join(entry, "bin")) {
			return true
		}
	}
	return false
}

// pathHasPrefix reports whether execPath is inside dir, comparing on cleaned
// path boundaries so /a/binfoo is not treated as being under /a/bin.
func pathHasPrefix(execPath, dir string) bool {
	execPath = filepath.Clean(execPath)
	dir = filepath.Clean(dir)
	if execPath == dir {
		return true
	}
	return strings.HasPrefix(execPath, dir+string(filepath.Separator))
}

// AssetName mirrors .goreleaser.yaml archive naming for the given GOOS/GOARCH:
// flagify_<os>_<arch>.tar.gz (or .zip on Windows). Unsupported platforms return
// ErrNoAssetForPlatform.
func AssetName(goos, goarch string) (string, error) {
	if !isSupportedPlatform(goos, goarch) {
		return "", fmt.Errorf("%w: %s/%s", ErrNoAssetForPlatform, goos, goarch)
	}
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("flagify_%s_%s.%s", goos, goarch, ext), nil
}

// isSupportedPlatform matches the goreleaser build matrix
// (darwin|linux|windows × amd64|arm64).
func isSupportedPlatform(goos, goarch string) bool {
	switch goos {
	case "darwin", "linux", "windows":
	default:
		return false
	}
	switch goarch {
	case "amd64", "arm64":
		return true
	default:
		return false
	}
}

// CurrentAssetName is AssetName for the platform this binary runs on.
func CurrentAssetName() (string, error) {
	return AssetName(runtime.GOOS, runtime.GOARCH)
}
