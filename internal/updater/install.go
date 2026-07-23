package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ErrChecksumMismatch is returned when a downloaded asset's SHA256 does not
// match the value recorded in checksums.txt. The original binary is never
// touched when this happens.
var ErrChecksumMismatch = errors.New("checksum mismatch")

// ErrWindowsFileInUse is returned when the running binary cannot even be
// renamed aside on Windows (another flagify process holds the name).
var ErrWindowsFileInUse = errors.New("windows binary in use")

// progressInterval controls how often download progress is printed (roughly
// every 1MB).
const progressInterval = 1 << 20

// InstallRelease downloads the correct asset for the current platform, verifies
// its SHA256 against the release's checksums.txt, extracts the binary, and
// atomically replaces targetPath. Every temporary file lives in
// filepath.Dir(targetPath) — never os.TempDir() — so the final os.Rename is
// atomic on the same filesystem. On any failure, targetPath is left untouched.
func InstallRelease(ctx context.Context, release *Release, targetPath string, showProgress bool) error {
	assetName, err := CurrentAssetName()
	if err != nil {
		return fmt.Errorf("%w: %s/%s", ErrNoAssetForPlatform, runtime.GOOS, runtime.GOARCH)
	}

	binaryAsset := findAsset(release, assetName)
	if binaryAsset == nil {
		return fmt.Errorf("%w: %s/%s", ErrNoAssetForPlatform, runtime.GOOS, runtime.GOARCH)
	}

	checksumAsset := findAsset(release, "checksums.txt")
	if checksumAsset == nil {
		return errors.New("release is missing checksums.txt — aborting, cannot verify integrity")
	}

	expectedSum, err := fetchExpectedChecksum(ctx, checksumAsset.BrowserDownloadURL, assetName)
	if err != nil {
		return err
	}

	dir := filepath.Dir(targetPath)
	pid := os.Getpid()

	downloadPath := filepath.Join(dir, fmt.Sprintf(".flagify-download-%d.tmp", pid))
	if err := downloadAndVerify(ctx, binaryAsset.BrowserDownloadURL, downloadPath, expectedSum, showProgress); err != nil {
		os.Remove(downloadPath)
		return err
	}

	binTmpPath := filepath.Join(dir, fmt.Sprintf(".flagify-update-%d.tmp", pid))
	extractErr := extractBinary(downloadPath, binTmpPath, assetName)
	// The download archive is no longer needed once extraction has been attempted.
	os.Remove(downloadPath)
	if extractErr != nil {
		os.Remove(binTmpPath)
		return extractErr
	}

	if err := atomicReplace(binTmpPath, targetPath); err != nil {
		os.Remove(binTmpPath)
		return err
	}
	return nil
}

// findAsset returns the asset whose name matches exactly, or nil.
func findAsset(release *Release, name string) *Asset {
	for i := range release.Assets {
		if release.Assets[i].Name == name {
			return &release.Assets[i]
		}
	}
	return nil
}

// fetchExpectedChecksum downloads checksums.txt in full (a few KB) and returns
// the hex digest recorded for assetName. The file uses the sha256sum format:
// "<hex>␠␠<filename>".
func fetchExpectedChecksum(ctx context.Context, url, assetName string) (string, error) {
	body, err := httpGet(ctx, url)
	if err != nil {
		return "", err
	}
	defer body.Close()

	data, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == assetName {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("checksums.txt has no entry for %s — aborting, cannot verify integrity", assetName)
}

// downloadAndVerify streams the asset to tmpPath while computing its SHA256 in
// the same pass (io.TeeReader), then compares against expectedSum. On mismatch
// it removes the temp file and returns ErrChecksumMismatch.
func downloadAndVerify(ctx context.Context, url, tmpPath, expectedSum string, showProgress bool) error {
	body, err := httpGet(ctx, url)
	if err != nil {
		return err
	}
	defer body.Close()

	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	hasher := sha256.New()
	var reader io.Reader = io.TeeReader(body, hasher)
	if showProgress {
		reader = io.TeeReader(reader, &progressWriter{})
	}

	if _, err := io.Copy(out, reader); err != nil {
		out.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	actualSum := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(actualSum, expectedSum) {
		os.Remove(tmpPath)
		return ErrChecksumMismatch
	}
	return nil
}

// extractBinary pulls the flagify (or flagify.exe on Windows) executable out of
// the downloaded archive into binTmpPath, applying 0o755 on Unix.
func extractBinary(archivePath, binTmpPath, assetName string) error {
	if strings.HasSuffix(assetName, ".zip") {
		return extractFromZip(archivePath, binTmpPath)
	}
	return extractFromTarGz(archivePath, binTmpPath)
}

func binaryEntryName() string {
	if runtime.GOOS == "windows" {
		return "flagify.exe"
	}
	return "flagify"
}

func extractFromTarGz(archivePath, binTmpPath string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzReader.Close()

	wanted := binaryEntryName()
	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if filepath.Base(header.Name) == wanted && header.Typeflag == tar.TypeReg {
			return writeExtracted(binTmpPath, tarReader)
		}
	}
	return fmt.Errorf("archive does not contain %s", wanted)
}

func extractFromZip(archivePath, binTmpPath string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	wanted := binaryEntryName()
	for _, entry := range reader.File {
		if filepath.Base(entry.Name) == wanted && !entry.FileInfo().IsDir() {
			rc, err := entry.Open()
			if err != nil {
				return err
			}
			defer rc.Close()
			return writeExtracted(binTmpPath, rc)
		}
	}
	return fmt.Errorf("archive does not contain %s", wanted)
}

// writeExtracted writes the extracted binary bytes to path with executable
// permissions (0o755 is a no-op semantically on Windows but harmless).
func writeExtracted(path string, src io.Reader) error {
	out, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(path, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// atomicReplace swaps the new binary into targetPath. On Unix, os.Rename is
// atomic and the kernel allows replacing an executable that is in use. On
// Windows, the running binary cannot be overwritten directly, so it is first
// renamed aside (targetPath → targetPath+".old") to free the name.
func atomicReplace(binTmpPath, targetPath string) error {
	if runtime.GOOS != "windows" {
		return os.Rename(binTmpPath, targetPath)
	}

	oldPath := targetPath + ".old"
	_ = os.Remove(oldPath)
	if err := os.Rename(targetPath, oldPath); err != nil {
		return ErrWindowsFileInUse
	}
	if err := os.Rename(binTmpPath, targetPath); err != nil {
		// Roll the original name back so targetPath is not left missing.
		_ = os.Rename(oldPath, targetPath)
		return err
	}
	// Best-effort: the .old file may stay locked until this process exits.
	_ = os.Remove(oldPath)
	return nil
}

// httpGet performs a context-aware GET and returns the body on a 2xx status,
// closing it on any error. Callers must close the returned body.
func httpGet(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "flagify-cli")

	resp, err := DefaultHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, fmt.Errorf("download failed with status %d for %s", resp.StatusCode, url)
	}
	return resp.Body, nil
}

// progressWriter prints plain periodic download progress (no animated widget).
type progressWriter struct {
	written     int64
	lastPrinted int64
}

func (p *progressWriter) Write(chunk []byte) (int, error) {
	p.written += int64(len(chunk))
	if p.written-p.lastPrinted >= progressInterval {
		p.lastPrinted = p.written
		fmt.Printf("  downloaded %.1f MB\n", float64(p.written)/float64(1<<20))
	}
	return len(chunk), nil
}
