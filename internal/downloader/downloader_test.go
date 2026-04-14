package downloader

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/clelange/cernopendata-client-go/internal/utils"
)

func TestNewDownloader(t *testing.T) {
	d := NewDownloader()

	if d == nil {
		t.Fatal("NewDownloader() returned nil")
	}

	if d.client == nil {
		t.Error("client is nil")
	}

	if d.retryLimit != 10 {
		t.Errorf("retryLimit = %d, want 10", d.retryLimit)
	}

	if d.retrySleep != 5 {
		t.Errorf("retrySleep = %d, want 5", d.retrySleep)
	}
}

func TestFilterFiles(t *testing.T) {
	files := []any{
		map[string]any{"uri": "/path/file1.txt"},
		map[string]any{"uri": "/path/file2.csv"},
		map[string]any{"uri": "/path/file3.log"},
	}

	tests := []struct {
		name     string
		filter   string
		expected int
	}{
		{"no filter", "", 3},
		{"glob filter", "*.txt", 1},
		{"no match", "*.json", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterFiles(files, tt.filter)
			if len(result) != tt.expected {
				t.Errorf("FilterFiles() = %d, want %d", len(result), tt.expected)
			}
		})
	}
}

func TestFilterFilesByRange(t *testing.T) {
	files := []any{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	tests := []struct {
		name     string
		start    int
		end      int
		expected int
	}{
		{"all files", 0, 10, 10},
		{"first three", 0, 3, 3},
		{"middle", 4, 7, 3},
		{"last two", 8, 10, 2},
		{"out of range", 0, 20, 10},
		{"invalid range", 5, 3, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterFilesByRange(files, tt.start, tt.end)
			if len(result) != tt.expected {
				t.Errorf("FilterFilesByRange() = %d, want %d", len(result), tt.expected)
			}
		})
	}
}

func TestDownloadStats(t *testing.T) {
	stats := DownloadStats{
		TotalFiles:      10,
		TotalBytes:      10000,
		DownloadedFiles: 8,
		DownloadedBytes: 8000,
		FailedFiles:     1,
		SkippedFiles:    1,
	}

	if stats.TotalFiles != 10 {
		t.Errorf("TotalFiles = %d, want 10", stats.TotalFiles)
	}

	if stats.DownloadedFiles != 8 {
		t.Errorf("DownloadedFiles = %d, want 8", stats.DownloadedFiles)
	}

	downloadedRatio := float64(stats.DownloadedFiles) / float64(stats.TotalFiles)
	if downloadedRatio < 0.7 || downloadedRatio > 0.9 {
		t.Errorf("DownloadedFiles ratio = %.2f, want ~0.8", downloadedRatio)
	}
}

func TestFileDownloadResult(t *testing.T) {
	result := &FileDownloadResult{
		URL:     "http://example.com/file.txt",
		Path:    "/tmp/file.txt",
		Size:    1024,
		Success: true,
		Retries: 2,
	}

	if result.URL != "http://example.com/file.txt" {
		t.Errorf("URL = %q, want %q", result.URL, "http://example.com/file.txt")
	}

	if result.Size != 1024 {
		t.Errorf("Size = %d, want 1024", result.Size)
	}

	if result.Retries != 2 {
		t.Errorf("Retries = %d, want 2", result.Retries)
	}
}

func TestFilterFilesByMultipleNames(t *testing.T) {
	fileLocations := []any{
		map[string]any{"uri": "http://example.com/a.txt"},
		map[string]any{"uri": "http://example.com/b.txt"},
		map[string]any{"uri": "http://example.com/c.py"},
	}

	result := FilterFilesByMultipleNames(fileLocations, []string{"a.txt", "c.py"})
	if len(result) != 2 {
		t.Errorf("FilterFilesByMultipleNames() = %d files, want 2", len(result))
	}

	expectedFiles := map[string]bool{
		"http://example.com/a.txt": true,
		"http://example.com/c.py":  true,
	}

	for _, file := range result {
		fileMap, ok := file.(map[string]any)
		if !ok {
			t.Errorf("File is not a map")
			continue
		}
		uri, ok := fileMap["uri"].(string)
		if !ok {
			t.Errorf("File URI is not a string")
			continue
		}
		if !expectedFiles[uri] {
			t.Errorf("Unexpected file: %s", uri)
		}
	}
}

func TestFilterFilesByRegex(t *testing.T) {
	fileLocations := []any{
		map[string]any{"uri": "http://example.com/a.py"},
		map[string]any{"uri": "http://example.com/b.txt"},
		map[string]any{"uri": "http://example.com/c.py"},
	}

	result := FilterFilesByRegex(fileLocations, `\.py$`)
	if len(result) != 2 {
		t.Errorf("FilterFilesByRegex() = %d files, want 2", len(result))
	}

	for _, file := range result {
		fileMap, ok := file.(map[string]any)
		if !ok {
			t.Errorf("File is not a map")
			continue
		}
		uri, ok := fileMap["uri"].(string)
		if !ok {
			t.Errorf("File URI is not a string")
			continue
		}
		if uri != "http://example.com/a.py" && uri != "http://example.com/c.py" {
			t.Errorf("Unexpected file: %s", uri)
		}
	}
}

func TestFilterFilesByRegexNoMatch(t *testing.T) {
	fileLocations := []any{
		map[string]any{"uri": "http://example.com/a.txt"},
		map[string]any{"uri": "http://example.com/b.txt"},
	}

	result := FilterFilesByRegex(fileLocations, `\.py$`)
	if len(result) != 0 {
		t.Errorf("FilterFilesByRegex() = %d files, want 0 (no matches)", len(result))
	}
}

func TestFilterFilesByRangeSingleFile(t *testing.T) {
	fileLocations := []any{
		map[string]any{"uri": "http://example.com/file1.txt"},
		map[string]any{"uri": "http://example.com/file2.txt"},
		map[string]any{"uri": "http://example.com/file3.txt"},
	}

	ranges, _ := utils.ParseRanges([]string{"2-2"})
	result := FilterFilesByMultipleRanges(fileLocations, ranges)
	if len(result) != 1 {
		t.Errorf("FilterFilesByMultipleRanges() = %d files, want 1", len(result))
	}

	if len(result) > 0 {
		fileMap, ok := result[0].(map[string]any)
		if ok {
			uri, _ := fileMap["uri"].(string)
			if uri != "http://example.com/file2.txt" {
				t.Errorf("Expected file2.txt, got %s", uri)
			}
		}
	}
}

func TestFilterFilesByRangeWithFilteredFiles(t *testing.T) {
	filteredFiles := []any{
		map[string]any{"uri": "http://example.com/file1.txt"},
		map[string]any{"uri": "http://example.com/file3.txt"},
	}

	ranges, _ := utils.ParseRanges([]string{"1-2"})
	result := FilterFilesByMultipleRanges(filteredFiles, ranges)
	if len(result) != 2 {
		t.Errorf("FilterFilesByMultipleRanges() = %d files, want 2", len(result))
	}

	expectedFiles := map[string]bool{
		"http://example.com/file1.txt": true,
		"http://example.com/file3.txt": true,
	}

	for _, file := range result {
		fileMap, ok := file.(map[string]any)
		if !ok {
			continue
		}
		uri, ok := fileMap["uri"].(string)
		if !ok {
			continue
		}
		if !expectedFiles[uri] {
			t.Errorf("Unexpected file: %s", uri)
		}
	}
}

func TestDownloadFile(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		resume     bool
		wantErr    bool
		wantSize   int64
		wantStatus bool
	}{
		{
			name: "successful download",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("test file content"))
			},
			resume:     false,
			wantErr:    false,
			wantSize:   17,
			wantStatus: true,
		},
		{
			name: "server error 404",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte("not found"))
			},
			resume:     false,
			wantErr:    true,
			wantStatus: false,
		},
		{
			name: "server error 500",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("internal error"))
			},
			resume:     false,
			wantErr:    true,
			wantStatus: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			tmpDir := t.TempDir()
			destPath := filepath.Join(tmpDir, "testfile.txt")

			d := &Downloader{
				client:     server.Client(),
				retryLimit: 1,
				retrySleep: 0,
			}

			result, err := d.DownloadFile(server.URL+"/testfile.txt", destPath, tt.resume, 0)

			if (err != nil) != tt.wantErr {
				t.Errorf("DownloadFile() error = %v, wantErr %v", err, tt.wantErr)
			}

			if result.Success != tt.wantStatus {
				t.Errorf("DownloadFile() success = %v, want %v", result.Success, tt.wantStatus)
			}

			if tt.wantStatus && result.Size != tt.wantSize {
				t.Errorf("DownloadFile() size = %d, want %d", result.Size, tt.wantSize)
			}
		})
	}
}

func TestDownloadFileResume(t *testing.T) {
	// Server that supports range requests
	handler := func(w http.ResponseWriter, r *http.Request) {
		rangeHeader := r.Header.Get("Range")
		if rangeHeader != "" {
			// Simulate partial content response
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte("resumed content"))
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("full content"))
		}
	}

	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "testfile.txt")

	// Create a partial file
	if err := os.WriteFile(destPath, []byte("partial"), 0600); err != nil {
		t.Fatalf("Failed to create partial file: %v", err)
	}

	d := &Downloader{
		client:     server.Client(),
		retryLimit: 1,
		retrySleep: 0,
	}

	result, err := d.DownloadFile(server.URL+"/testfile.txt", destPath, true, 0)
	if err != nil {
		t.Fatalf("DownloadFile() error = %v", err)
	}

	if !result.Success {
		t.Error("DownloadFile() expected success")
	}
}

func TestDownloadFileResumeRetryUsesUpdatedRange(t *testing.T) {
	content := []byte("abcdefghijk")
	existing := []byte("abc")
	firstChunk := []byte("defg")
	secondChunk := []byte("hijk")

	callCount := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		callCount++
		switch callCount {
		case 1:
			if r.Header.Get("Range") != "bytes=3-" {
				t.Errorf("Expected first Range header 'bytes=3-', got %q", r.Header.Get("Range"))
			}
			w.Header().Set("Content-Length", "8")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(firstChunk)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case 2:
			if r.Header.Get("Range") != "bytes=7-" {
				t.Errorf("Expected retry Range header 'bytes=7-', got %q", r.Header.Get("Range"))
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(secondChunk)))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(secondChunk)
		default:
			t.Fatalf("Unexpected request count: %d", callCount)
		}
	}

	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "testfile.txt")

	if err := os.WriteFile(destPath, existing, 0600); err != nil {
		t.Fatalf("Failed to create partial file: %v", err)
	}

	d := &Downloader{
		client:     server.Client(),
		retryLimit: 2,
		retrySleep: 0,
	}

	result, err := d.DownloadFile(server.URL+"/testfile.txt", destPath, true, int64(len(content)))
	if err != nil {
		t.Fatalf("DownloadFile() error = %v", err)
	}
	if !result.Success {
		t.Fatalf("DownloadFile() expected success")
	}

	final, err := os.ReadFile(destPath) // #nosec G304 -- test file path
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(final) != string(content) {
		t.Errorf("File content = %q, want %q", string(final), string(content))
	}
}

func TestDownloadFiles(t *testing.T) {
	downloadCount := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		downloadCount++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("file content"))
	}

	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	files := []any{
		map[string]any{
			"uri":      server.URL + "/file1.txt",
			"size":     float64(12),
			"checksum": "adler32:12345678",
		},
		map[string]any{
			"uri":      server.URL + "/file2.txt",
			"size":     float64(12),
			"checksum": "adler32:87654321",
		},
	}

	tmpDir := t.TempDir()

	d := &Downloader{
		client:     server.Client(),
		retryLimit: 1,
		retrySleep: 0,
	}

	stats := d.DownloadFiles(files, tmpDir, 1, 0, false, false, false)

	if stats.TotalFiles != 2 {
		t.Errorf("TotalFiles = %d, want 2", stats.TotalFiles)
	}

	if stats.DownloadedFiles != 2 {
		t.Errorf("DownloadedFiles = %d, want 2", stats.DownloadedFiles)
	}

	if stats.FailedFiles != 0 {
		t.Errorf("FailedFiles = %d, want 0", stats.FailedFiles)
	}
}

func TestAssignLocalPathsUsesMinimalUniqueSuffix(t *testing.T) {
	files := []any{
		map[string]any{"uri": "http://opendata.cern.ch//eos/opendata/alice/2015/LHC15o/000245349/0002/AO2D.root"},
		map[string]any{"uri": "http://opendata.cern.ch//eos/opendata/alice/2015/LHC15o/000245349/0003/AO2D.root"},
		map[string]any{"uri": "http://opendata.cern.ch//eos/opendata/alice/2015/LHC15o/000245349/0004/AO2D.root"},
	}

	AssignLocalPaths(files)

	expected := []string{"0002/AO2D.root", "0003/AO2D.root", "0004/AO2D.root"}
	for i, file := range files {
		fileMap := file.(map[string]any)
		localPath, _ := fileMap["local_path"].(string)
		if localPath != expected[i] {
			t.Errorf("local_path[%d] = %q, want %q", i, localPath, expected[i])
		}
	}
}

func TestAssignLocalPathsExtendsSuffixUntilUnique(t *testing.T) {
	files := []any{
		map[string]any{"uri": "http://example.com/store/runA/0001/AO2D.root"},
		map[string]any{"uri": "http://example.com/store/runB/0001/AO2D.root"},
	}

	AssignLocalPaths(files)

	expected := []string{"runA/0001/AO2D.root", "runB/0001/AO2D.root"}
	for i, file := range files {
		fileMap := file.(map[string]any)
		localPath, _ := fileMap["local_path"].(string)
		if localPath != expected[i] {
			t.Errorf("local_path[%d] = %q, want %q", i, localPath, expected[i])
		}
	}
}

func TestDownloadFilesPreservesUniqueSubdirectoriesForDuplicateBasenames(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("file content"))
	}

	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	files := []any{
		map[string]any{
			"uri":      server.URL + "/dataset/0002/AO2D.root",
			"size":     float64(12),
			"checksum": "adler32:12345678",
		},
		map[string]any{
			"uri":      server.URL + "/dataset/0003/AO2D.root",
			"size":     float64(12),
			"checksum": "adler32:87654321",
		},
	}

	tmpDir := t.TempDir()
	d := &Downloader{
		client:     server.Client(),
		retryLimit: 1,
		retrySleep: 0,
	}

	stats := d.DownloadFiles(files, tmpDir, 1, 0, false, false, false)
	if stats.DownloadedFiles != 2 {
		t.Fatalf("DownloadedFiles = %d, want 2", stats.DownloadedFiles)
	}

	for _, relPath := range []string{"0002/AO2D.root", "0003/AO2D.root"} {
		if _, err := os.Stat(filepath.Join(tmpDir, relPath)); err != nil {
			t.Errorf("expected downloaded file at %s: %v", relPath, err)
		}
	}
}

func TestDownloadFilesDryRun(t *testing.T) {
	downloadCount := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		downloadCount++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("file content"))
	}

	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	files := []any{
		map[string]any{
			"uri":      server.URL + "/file1.txt",
			"size":     float64(100),
			"checksum": "adler32:12345678",
		},
	}

	tmpDir := t.TempDir()

	d := &Downloader{
		client:     server.Client(),
		retryLimit: 1,
		retrySleep: 0,
	}

	stats := d.DownloadFiles(files, tmpDir, 1, 0, false, true, false) // dry-run = true

	if downloadCount != 0 {
		t.Errorf("Expected no actual downloads in dry-run mode, but got %d", downloadCount)
	}

	if stats.DownloadedFiles != 1 {
		t.Errorf("DownloadedFiles = %d, want 1 (simulated)", stats.DownloadedFiles)
	}
}

func TestDownloadFilesSkipExisting(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("new content"))
	}

	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	tmpDir := t.TempDir()

	// Pre-create a file
	existingFile := filepath.Join(tmpDir, "file1.txt")
	if err := os.WriteFile(existingFile, []byte("existing"), 0600); err != nil {
		t.Fatalf("Failed to create existing file: %v", err)
	}

	files := []any{
		map[string]any{
			"uri":      server.URL + "/file1.txt",
			"size":     float64(8),
			"checksum": "adler32:12345678",
		},
	}

	d := &Downloader{
		client:     server.Client(),
		retryLimit: 1,
		retrySleep: 0,
	}

	stats := d.DownloadFiles(files, tmpDir, 1, 0, false, false, false)

	if stats.SkippedFiles != 1 {
		t.Errorf("SkippedFiles = %d, want 1", stats.SkippedFiles)
	}

	if stats.DownloadedFiles != 0 {
		t.Errorf("DownloadedFiles = %d, want 0", stats.DownloadedFiles)
	}

	// Verify original content wasn't overwritten
	content, _ := os.ReadFile(existingFile) // #nosec G304 -- test file path
	if string(content) != "existing" {
		t.Errorf("Existing file was overwritten: got %q", string(content))
	}
}

func TestDownloadFilesInvalidEntry(t *testing.T) {
	files := []any{
		"not a map", // invalid entry
		map[string]any{
			"uri":  "http://example.com/file.txt",
			"size": float64(100),
		},
	}

	tmpDir := t.TempDir()

	d := NewDownloader()
	stats := d.DownloadFiles(files, tmpDir, 1, 0, false, true, false) // dry-run to avoid network

	if stats.SkippedFiles != 1 {
		t.Errorf("SkippedFiles = %d, want 1 (for invalid entry)", stats.SkippedFiles)
	}
}

func TestDownloadFilesResumePartial(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		rangeHeader := r.Header.Get("Range")
		if rangeHeader != "" {
			// Expecting Range: bytes=8-
			if rangeHeader != "bytes=8-" {
				t.Errorf("Expected Range header 'bytes=8-', got %q", rangeHeader)
			}
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte("resumed"))
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("full content"))
		}
	}

	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	tmpDir := t.TempDir()

	// Pre-create a partial file
	destPath := filepath.Join(tmpDir, "file1.txt")
	// "existing" is 8 bytes
	if err := os.WriteFile(destPath, []byte("existing"), 0600); err != nil {
		t.Fatalf("Failed to create partial file: %v", err)
	}

	files := []any{
		map[string]any{
			"uri":      server.URL + "/file1.txt",
			"size":     float64(15), // "existing" (8) + "resumed" (7) = 15
			"checksum": "adler32:12345678",
		},
	}

	d := &Downloader{
		client:     server.Client(),
		retryLimit: 1,
		retrySleep: 0,
	}

	stats := d.DownloadFiles(files, tmpDir, 1, 0, false, false, false)

	if stats.DownloadedFiles != 1 {
		t.Errorf("DownloadedFiles = %d, want 1", stats.DownloadedFiles)
	}

	if stats.SkippedFiles != 0 {
		t.Errorf("SkippedFiles = %d, want 0", stats.SkippedFiles)
	}

	// Verify merged content
	content, _ := os.ReadFile(destPath) // #nosec G304
	if string(content) != "existingresumed" {
		t.Errorf("File content = %q, want 'existingresumed'", string(content))
	}
}
