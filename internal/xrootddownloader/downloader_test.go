package xrootddownloader

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cernopendata/cernopendata-client-go/internal/filemetadata"
)

type fakeRemoteFile struct {
	data []byte
	err  error
}

func (f *fakeRemoteFile) ReadAtContext(ctx context.Context, p []byte, offset int64) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if f.err != nil {
		return 0, f.err
	}
	if offset >= int64(len(f.data)) {
		return 0, io.EOF
	}
	n := copy(p, f.data[offset:])
	if offset+int64(n) >= int64(len(f.data)) {
		return n, io.EOF
	}
	return n, nil
}

func (f *fakeRemoteFile) Close(context.Context) error { return nil }

func TestNewDownloader(t *testing.T) {
	d := NewDownloader()

	if d == nil {
		t.Fatal("NewDownloader returned nil")
	}

	if d.retryLimit != 10 {
		t.Errorf("Expected retryLimit 10, got %d", d.retryLimit)
	}

	if d.retrySleep != 5 {
		t.Errorf("Expected retrySleep 5, got %d", d.retrySleep)
	}

	if d.username != "gopher" {
		t.Errorf("Expected username 'gopher', got '%s'", d.username)
	}
}

func TestDownloadStats(t *testing.T) {
	stats := DownloadStats{
		TotalFiles:      10,
		TotalBytes:      1000000,
		DownloadedFiles: 8,
		DownloadedBytes: 800000,
		FailedFiles:     1,
		SkippedFiles:    1,
	}

	if stats.TotalFiles != 10 {
		t.Errorf("Expected TotalFiles 10, got %d", stats.TotalFiles)
	}

	if stats.DownloadedFiles != 8 {
		t.Errorf("Expected DownloadedFiles 8, got %d", stats.DownloadedFiles)
	}

	if stats.FailedFiles != 1 {
		t.Errorf("Expected FailedFiles 1, got %d", stats.FailedFiles)
	}

	if stats.SkippedFiles != 1 {
		t.Errorf("Expected SkippedFiles 1, got %d", stats.SkippedFiles)
	}
}

func TestFileDownloadResult(t *testing.T) {
	result := FileDownloadResult{
		URL:      "root://test/file.dat",
		Path:     "/tmp/file.dat",
		Size:     1024,
		Checksum: "abc123",
		Success:  true,
		Retries:  2,
	}

	if result.URL != "root://test/file.dat" {
		t.Errorf("Expected URL 'root://test/file.dat', got '%s'", result.URL)
	}

	if result.Path != "/tmp/file.dat" {
		t.Errorf("Expected Path '/tmp/file.dat', got '%s'", result.Path)
	}

	if result.Size != 1024 {
		t.Errorf("Expected Size 1024, got %d", result.Size)
	}

	if !result.Success {
		t.Error("Expected Success to be true")
	}

	if result.Retries != 2 {
		t.Errorf("Expected Retries 2, got %d", result.Retries)
	}
}

func TestClose(t *testing.T) {
	d := NewDownloader()

	err := d.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}

	err = d.Close()
	if err != nil {
		t.Errorf("Close after Close returned error: %v", err)
	}
}

func TestDownloadFilesDryRun(t *testing.T) {
	d := NewDownloader()
	d.dryRun = true
	d.verbose = true

	files := []filemetadata.File{
		{URI: "root://test/file1.dat", Size: 1000, Checksum: "abc123"},
		{URI: "root://test/file2.dat", Size: 2000, Checksum: "def456"},
	}

	ctx := context.Background()
	stats := d.DownloadFiles(ctx, files, "/tmp/test", 3, 2, true, true, false)

	if stats.TotalFiles != 2 {
		t.Errorf("Expected TotalFiles 2, got %d", stats.TotalFiles)
	}

	if stats.DownloadedFiles != 2 {
		t.Errorf("Expected DownloadedFiles 2 (dry run), got %d", stats.DownloadedFiles)
	}

	if stats.FailedFiles != 0 {
		t.Errorf("Expected FailedFiles 0 (dry run), got %d", stats.FailedFiles)
	}

	if stats.SkippedFiles != 0 {
		t.Errorf("Expected SkippedFiles 0 (dry run), got %d", stats.SkippedFiles)
	}

	if stats.DownloadedBytes != 3000 {
		t.Errorf("Expected DownloadedBytes 3000, got %d", stats.DownloadedBytes)
	}
}

func TestDownloadFilesEmpty(t *testing.T) {
	d := NewDownloader()

	ctx := context.Background()
	stats := d.DownloadFiles(ctx, nil, t.TempDir(), 3, 2, true, true, false)

	if stats.TotalFiles != 0 {
		t.Errorf("Expected TotalFiles 0, got %d", stats.TotalFiles)
	}
}

func TestDownloadFileRetrySuccessResetsAttemptError(t *testing.T) {
	temporaryError := errors.New("temporary open failure")
	openCalls := 0
	d := NewDownloader()
	d.retryLimit = 2
	d.retrySleep = 0
	d.openFile = func(context.Context, string) (remoteFile, error) {
		openCalls++
		if openCalls == 1 {
			return nil, temporaryError
		}
		return &fakeRemoteFile{data: []byte("payload")}, nil
	}

	destPath := filepath.Join(t.TempDir(), "file.dat")
	result, err := d.DownloadFile(context.Background(), "root://test/file.dat", destPath, false, 7)
	if err != nil {
		t.Fatalf("DownloadFile returned error after successful retry: %v", err)
	}
	if !result.Success || result.Retries != 1 {
		t.Fatalf("result = %+v, want successful result with one retry", result)
	}
	data, err := os.ReadFile(destPath) // #nosec G304 -- destination is inside t.TempDir
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "payload" {
		t.Fatalf("downloaded data = %q, want payload", data)
	}
}

func TestDownloadFileRetryExhaustion(t *testing.T) {
	wantErr := errors.New("remote unavailable")
	openCalls := 0
	d := NewDownloader()
	d.retryLimit = 3
	d.retrySleep = 0
	d.openFile = func(context.Context, string) (remoteFile, error) {
		openCalls++
		return nil, wantErr
	}

	result, err := d.DownloadFile(context.Background(), "root://test/file.dat", filepath.Join(t.TempDir(), "file.dat"), false, 7)
	if !errors.Is(err, wantErr) {
		t.Fatalf("DownloadFile error = %v, want %v", err, wantErr)
	}
	if openCalls != 3 {
		t.Fatalf("open calls = %d, want 3", openCalls)
	}
	if result.Success || result.Retries != 2 || !errors.Is(result.Error, wantErr) {
		t.Fatalf("result = %+v, want exhausted failure", result)
	}
}

func TestDownloadFileCancellationInterruptsRetryWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := NewDownloader()
	d.retryLimit = 3
	d.retrySleep = 30
	d.openFile = func(context.Context, string) (remoteFile, error) {
		cancel()
		return nil, errors.New("temporary failure")
	}

	start := time.Now()
	result, err := d.DownloadFile(ctx, "root://test/file.dat", filepath.Join(t.TempDir(), "file.dat"), false, 7)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("DownloadFile error = %v, want context canceled", err)
	}
	if time.Since(start) > time.Second {
		t.Fatalf("cancellation did not interrupt retry wait")
	}
	if result.Success || !errors.Is(result.Error, context.Canceled) {
		t.Fatalf("result = %+v, want canceled failure", result)
	}
}
