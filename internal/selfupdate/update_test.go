package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeTestTarball returns a gzipped tar archive containing a single
// regular file named binName whose contents are body. Used to build
// fixture tarballs the orchestrator can extract from.
func makeTestTarball(t *testing.T, binName string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	hdr := &tar.Header{
		Name:     binName,
		Mode:     0o755,
		Size:     int64(len(body)),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("tar header: %v", err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatalf("tar write: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gz close: %v", err)
	}
	return buf.Bytes()
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestReadExpectedChecksum_Goreleaser(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	checksums := filepath.Join(dir, "checksums.txt")
	body := "abc123  trvl_1.2.0_linux_amd64.tar.gz\n" +
		"def456  trvl_1.2.0_darwin_arm64.tar.gz\n"
	if err := os.WriteFile(checksums, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readExpectedChecksum(checksums, "trvl_1.2.0_darwin_arm64.tar.gz")
	if err != nil {
		t.Fatalf("readExpectedChecksum: %v", err)
	}
	if got != "def456" {
		t.Errorf("got %q want def456", got)
	}
}

func TestReadExpectedChecksum_BSDStyle(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	checksums := filepath.Join(dir, "checksums.txt")
	body := "SHA256 (trvl_1.2.0_linux_amd64.tar.gz) = abc123\n"
	if err := os.WriteFile(checksums, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readExpectedChecksum(checksums, "trvl_1.2.0_linux_amd64.tar.gz")
	if err != nil {
		t.Fatalf("readExpectedChecksum: %v", err)
	}
	if got != "abc123" {
		t.Errorf("got %q want abc123", got)
	}
}

func TestReadExpectedChecksum_NotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	checksums := filepath.Join(dir, "checksums.txt")
	body := "abc123  trvl_1.2.0_linux_amd64.tar.gz\n"
	if err := os.WriteFile(checksums, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := readExpectedChecksum(checksums, "nonexistent.tar.gz")
	if err == nil {
		t.Errorf("expected error for missing entry, got nil")
	}
}

func TestSHA256File(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "data")
	body := []byte("the quick brown fox")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := sha256File(path)
	if err != nil {
		t.Fatalf("sha256File: %v", err)
	}
	want := sha256Hex(body)
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestVerifySHA256_OK(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tarball := filepath.Join(dir, "trvl.tar.gz")
	body := []byte("fake tarball bytes")
	if err := os.WriteFile(tarball, body, 0o600); err != nil {
		t.Fatal(err)
	}
	checksums := filepath.Join(dir, "checksums.txt")
	body2 := fmt.Sprintf("%s  trvl.tar.gz\n", sha256Hex(body))
	if err := os.WriteFile(checksums, []byte(body2), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifySHA256(tarball, checksums, "trvl.tar.gz"); err != nil {
		t.Errorf("verifySHA256 unexpected: %v", err)
	}
}

func TestVerifySHA256_Mismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tarball := filepath.Join(dir, "trvl.tar.gz")
	if err := os.WriteFile(tarball, []byte("real"), 0o600); err != nil {
		t.Fatal(err)
	}
	checksums := filepath.Join(dir, "checksums.txt")
	body := fmt.Sprintf("%s  trvl.tar.gz\n", sha256Hex([]byte("forged")))
	if err := os.WriteFile(checksums, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	err := verifySHA256(tarball, checksums, "trvl.tar.gz")
	if err == nil {
		t.Errorf("expected mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "mismatch") {
		t.Errorf("expected error to mention mismatch, got: %v", err)
	}
}

func TestExtractBinaryFromTarGz_OK(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tarballPath := filepath.Join(dir, "trvl.tar.gz")
	wantBody := []byte("fake trvl binary content")
	if err := os.WriteFile(tarballPath, makeTestTarball(t, "trvl", wantBody), 0o600); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "extracted-trvl")
	if err := extractBinaryFromTarGz(tarballPath, "trvl", dest); err != nil {
		t.Fatalf("extract: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read extracted: %v", err)
	}
	if !bytes.Equal(got, wantBody) {
		t.Errorf("extracted bytes mismatch")
	}
}

func TestExtractBinaryFromTarGz_BinaryNotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tarballPath := filepath.Join(dir, "trvl.tar.gz")
	if err := os.WriteFile(tarballPath, makeTestTarball(t, "something-else", []byte("body")), 0o600); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "extracted-trvl")
	err := extractBinaryFromTarGz(tarballPath, "trvl", dest)
	if err == nil {
		t.Errorf("expected error for missing binary, got nil")
	}
}

func TestExtractBinaryFromTarGz_NotGzip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tarballPath := filepath.Join(dir, "trvl.tar.gz")
	if err := os.WriteFile(tarballPath, []byte("not gzip data"), 0o600); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "extracted")
	err := extractBinaryFromTarGz(tarballPath, "trvl", dest)
	if err == nil {
		t.Errorf("expected gunzip error, got nil")
	}
}

func TestAtomicReplace_SimpleRename(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "new-trvl")
	dst := filepath.Join(dir, "current-trvl")

	if err := os.WriteFile(src, []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := atomicReplace(src, dst); err != nil {
		t.Fatalf("atomicReplace: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != "new" {
		t.Errorf("got %q want %q", got, "new")
	}
}

func TestAtomicReplace_NoExistingDst(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "new-trvl")
	dst := filepath.Join(dir, "current-trvl")

	if err := os.WriteFile(src, []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}
	// dst does not exist.

	if err := atomicReplace(src, dst); err != nil {
		t.Fatalf("atomicReplace: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != "new" {
		t.Errorf("got %q want %q", got, "new")
	}
}

func TestCopyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	if err := os.WriteFile(src, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Errorf("got %q want hello", got)
	}
}

func TestUpdater_DownloadTo_Status404(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	u := NewUpdater(WithUpdaterBaseURL(srv.URL))
	dir := t.TempDir()
	dst := filepath.Join(dir, "tarball")
	err := u.downloadTo(context.Background(), srv.URL+"/missing", dst)
	if err == nil {
		t.Errorf("expected 404 error, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 in error, got: %v", err)
	}
}

func TestUpdater_DownloadTo_OK(t *testing.T) {
	t.Parallel()
	body := []byte("file body")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	u := NewUpdater(WithUpdaterBaseURL(srv.URL))
	dir := t.TempDir()
	dst := filepath.Join(dir, "out")
	if err := u.downloadTo(context.Background(), srv.URL+"/anything", dst); err != nil {
		t.Fatalf("downloadTo: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("body mismatch")
	}
}

func TestUpdater_PerformUpdate_BadInputs(t *testing.T) {
	t.Parallel()
	u := NewUpdater()
	// Empty version.
	if _, err := u.PerformUpdate(context.Background(), "", "/tmp/x"); err == nil {
		t.Errorf("expected error for empty version")
	}
	// Empty exePath.
	if _, err := u.PerformUpdate(context.Background(), "1.2.3", ""); err == nil {
		t.Errorf("expected error for empty exePath")
	}
}

// TestUpdater_PerformUpdate_DownloadFails verifies the orchestrator
// surfaces network failures cleanly. The mock server returns 500 for
// every request, so the first download (tarball) fails immediately.
func TestUpdater_PerformUpdate_DownloadFails(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fail", http.StatusInternalServerError)
	}))
	defer srv.Close()

	u := NewUpdater(
		WithUpdaterBaseURL(srv.URL),
		WithUpdaterPlatform("linux", "amd64"),
	)
	dir := t.TempDir()
	exePath := filepath.Join(dir, "trvl")
	if err := os.WriteFile(exePath, []byte("orig"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := u.PerformUpdate(context.Background(), "1.2.3", exePath)
	if err == nil {
		t.Errorf("expected download error, got nil")
	}
	// Existing binary must be untouched on failure.
	got, _ := os.ReadFile(exePath)
	if string(got) != "orig" {
		t.Errorf("binary was modified on failure: %q", got)
	}
}

// TestUpdater_PerformUpdate_SHA256Mismatch verifies the orchestrator
// aborts before swap if the checksum is wrong, leaving the on-disk
// binary intact. Uses a working tarball download but a bogus checksum.
func TestUpdater_PerformUpdate_SHA256Mismatch(t *testing.T) {
	t.Parallel()
	tarballBody := makeTestTarball(t, "trvl", []byte("new binary"))
	mux := http.NewServeMux()
	mux.HandleFunc("/v1.2.3/trvl_1.2.3_linux_amd64.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(tarballBody)
	})
	mux.HandleFunc("/v1.2.3/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		// Wrong digest.
		_, _ = fmt.Fprintln(w, "0000000000000000000000000000000000000000000000000000000000000000  trvl_1.2.3_linux_amd64.tar.gz")
	})
	mux.HandleFunc("/v1.2.3/trvl_1.2.3_linux_amd64.tar.gz.mldsa65.sig", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(make([]byte, 3309))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	u := NewUpdater(
		WithUpdaterBaseURL(srv.URL),
		WithUpdaterPlatform("linux", "amd64"),
	)
	dir := t.TempDir()
	exePath := filepath.Join(dir, "trvl")
	if err := os.WriteFile(exePath, []byte("orig"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := u.PerformUpdate(context.Background(), "1.2.3", exePath)
	if err == nil {
		t.Errorf("expected sha256 mismatch error")
	}
	if !strings.Contains(err.Error(), "sha256") {
		t.Errorf("expected sha256 in error, got: %v", err)
	}
	got, _ := os.ReadFile(exePath)
	if string(got) != "orig" {
		t.Errorf("binary was modified on sha256 failure: %q", got)
	}
}

// TestUpdater_PerformUpdate_MLDSAMismatch verifies the orchestrator
// aborts when the ML-DSA signature is wrong (using a forged sig of the
// correct length). SHA-256 passes but ML-DSA fails, simulating an
// attacker who recomputed the checksum after tampering with the
// tarball but cannot mint a valid ML-DSA signature.
func TestUpdater_PerformUpdate_MLDSAMismatch(t *testing.T) {
	t.Parallel()
	tarballBody := makeTestTarball(t, "trvl", []byte("attacker payload"))
	correctSHA := sha256Hex(tarballBody)
	mux := http.NewServeMux()
	mux.HandleFunc("/v1.2.3/trvl_1.2.3_linux_amd64.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(tarballBody)
	})
	mux.HandleFunc("/v1.2.3/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "%s  trvl_1.2.3_linux_amd64.tar.gz\n", correctSHA)
	})
	mux.HandleFunc("/v1.2.3/trvl_1.2.3_linux_amd64.tar.gz.mldsa65.sig", func(w http.ResponseWriter, r *http.Request) {
		// Right size, wrong contents — never verifies under any pubkey.
		_, _ = w.Write(make([]byte, 3309))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	u := NewUpdater(
		WithUpdaterBaseURL(srv.URL),
		WithUpdaterPlatform("linux", "amd64"),
	)
	dir := t.TempDir()
	exePath := filepath.Join(dir, "trvl")
	if err := os.WriteFile(exePath, []byte("orig"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := u.PerformUpdate(context.Background(), "1.2.3", exePath)
	if err == nil {
		t.Errorf("expected mldsa mismatch error")
	}
	if !strings.Contains(err.Error(), "mldsa65") {
		t.Errorf("expected mldsa65 in error, got: %v", err)
	}
	got, _ := os.ReadFile(exePath)
	if string(got) != "orig" {
		t.Errorf("binary was modified on mldsa failure: %q", got)
	}
}

// readSeekToReader is a tiny wrapper used in test helpers below.
type readSeekToReader struct{ io.Reader }

func (r readSeekToReader) Read(p []byte) (int, error) { return r.Reader.Read(p) }
