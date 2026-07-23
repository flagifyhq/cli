package updater

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

// buildTarGz returns a .tar.gz archive containing a single `flagify` entry with
// the given content, mirroring the goreleaser archive layout.
func buildTarGz(t *testing.T, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)
	entry := binaryEntryName()
	if err := tarWriter.WriteHeader(&tar.Header{
		Name:     entry,
		Mode:     0o755,
		Size:     int64(len(content)),
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tarWriter.Write(content); err != nil {
		t.Fatalf("write tar body: %v", err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gzWriter.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buf.Bytes()
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// releaseServer serves the binary archive and checksums.txt and returns a
// *Release pointing at it. checksumOverride, when non-empty, replaces the real
// checksum to simulate a mismatch; omitChecksums drops the checksums.txt asset.
func releaseServer(t *testing.T, archive []byte, checksumOverride string, omitChecksums bool) (*Release, func()) {
	t.Helper()
	assetName, err := CurrentAssetName()
	if err != nil {
		t.Skipf("unsupported platform: %v", err)
	}

	sum := sha256Hex(archive)
	if checksumOverride != "" {
		sum = checksumOverride
	}
	checksums := fmt.Sprintf("%s  %s\n", sum, assetName)

	mux := http.NewServeMux()
	mux.HandleFunc("/asset", func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	})
	mux.HandleFunc("/checksums", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(checksums))
	})
	srv := httptest.NewServer(mux)

	assets := []Asset{{Name: assetName, BrowserDownloadURL: srv.URL + "/asset"}}
	if !omitChecksums {
		assets = append(assets, Asset{Name: "checksums.txt", BrowserDownloadURL: srv.URL + "/checksums"})
	}
	return &Release{TagName: "v9.9.9", Assets: assets}, srv.Close
}

func TestInstallRelease_HappyPath(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "flagify")
	if err := os.WriteFile(target, []byte("OLD BINARY"), 0o755); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	newContent := []byte("NEW BINARY CONTENT")
	archive := buildTarGz(t, newContent)
	if runtime.GOOS == "windows" {
		t.Skip("happy-path replace exercised on Unix; Windows rename-aside is manual-only")
	}
	release, cleanup := releaseServer(t, archive, "", false)
	defer cleanup()

	if err := InstallRelease(context.Background(), release, target, false); err != nil {
		t.Fatalf("InstallRelease: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if !bytes.Equal(got, newContent) {
		t.Fatalf("target content = %q, want %q", got, newContent)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat target: %v", err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Fatalf("target is not executable: %v", info.Mode())
	}
	assertNoTempFiles(t, dir)
}

func TestInstallRelease_ChecksumMismatch(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "flagify")
	original := []byte("OLD BINARY")
	if err := os.WriteFile(target, original, 0o755); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	archive := buildTarGz(t, []byte("NEW BINARY CONTENT"))
	badSum := "0000000000000000000000000000000000000000000000000000000000000000"
	release, cleanup := releaseServer(t, archive, badSum, false)
	defer cleanup()

	err := InstallRelease(context.Background(), release, target, false)
	if err != ErrChecksumMismatch {
		t.Fatalf("got %v, want ErrChecksumMismatch", err)
	}

	got, _ := os.ReadFile(target)
	if !bytes.Equal(got, original) {
		t.Fatalf("original binary was modified: %q", got)
	}
	assertNoTempFiles(t, dir)
}

func TestInstallRelease_MissingChecksums(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "flagify")
	if err := os.WriteFile(target, []byte("OLD"), 0o755); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	archive := buildTarGz(t, []byte("NEW"))
	release, cleanup := releaseServer(t, archive, "", true)
	defer cleanup()

	if err := InstallRelease(context.Background(), release, target, false); err == nil {
		t.Fatal("expected error when checksums.txt is missing")
	}
	assertNoTempFiles(t, dir)
}

func TestInstallRelease_AssetMissing(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "flagify")
	if err := os.WriteFile(target, []byte("OLD"), 0o755); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	// A release whose only asset is checksums.txt — no platform binary.
	release := &Release{TagName: "v9.9.9", Assets: []Asset{{Name: "checksums.txt", BrowserDownloadURL: "http://x/c"}}}
	if err := InstallRelease(context.Background(), release, target, false); err == nil {
		t.Fatal("expected error when platform asset is absent")
	}
	assertNoTempFiles(t, dir)
}

func TestInstallRelease_PermissionDenied(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 0555 permission gate is a Unix-only test")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses directory permissions")
	}

	dir := t.TempDir()
	target := filepath.Join(dir, "flagify")
	if err := os.WriteFile(target, []byte("OLD"), 0o755); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	archive := buildTarGz(t, []byte("NEW"))
	release, cleanup := releaseServer(t, archive, "", false)
	defer cleanup()

	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod dir: %v", err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	if err := InstallRelease(context.Background(), release, target, false); err == nil {
		t.Fatal("expected a permission error writing to a read-only directory")
	}
}

// assertNoTempFiles verifies the installer cleaned up its temporary artifacts.
func assertNoTempFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if len(name) > 9 && name[:9] == ".flagify-" {
			t.Fatalf("temp file left behind: %s", name)
		}
	}
}
