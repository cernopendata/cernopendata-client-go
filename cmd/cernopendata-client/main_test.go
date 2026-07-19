package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/cernopendata/cernopendata-client-go/internal/downloader"
	"github.com/cernopendata/cernopendata-client-go/internal/filemetadata"
)

func executeArgs(t *testing.T, args ...string) error {
	t.Helper()
	cmd := newRootCommand()
	cmd.SetArgs(args)
	return cmd.Execute()
}

func TestCommandValidationReturnsErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "filter requires an output value",
			args: []string{"get-metadata", "--filter", "name=value"},
			want: "--filter can only be used with --output-value",
		},
		{
			name: "invalid file availability",
			args: []string{"get-file-locations", "--file-availability", "tape"},
			want: "Invalid file availability: tape",
		},
		{
			name: "invalid directory output format",
			args: []string{"list-directory", "/tmp", "--format", "yaml"},
			want: "Invalid format: yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := executeArgs(t, tt.args...)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Execute() error = %v, want error containing %q", err, tt.want)
			}
		})
	}
}

func TestConflictingFlagsReturnError(t *testing.T) {
	err := executeArgs(t, "download-files", "--expand", "--no-expand")
	if err == nil || !strings.Contains(err.Error(), "Cannot specify both --expand and --no-expand") {
		t.Fatalf("Execute() error = %v, want conflicting flags diagnostic", err)
	}
}

func TestUnknownCommandReturnsErrorWithoutCobraOutput(t *testing.T) {
	cmd := newRootCommand()
	var cobraOutput strings.Builder
	cmd.SetErr(&cobraOutput)
	cmd.SetArgs([]string{"does-not-exist"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), `unknown command "does-not-exist"`) {
		t.Fatalf("Execute() error = %v, want unknown command diagnostic", err)
	}
	if cobraOutput.Len() != 0 {
		t.Fatalf("Cobra wrote duplicate diagnostic %q", cobraOutput.String())
	}
}

func TestExecutePrintsOneDiagnosticAndReturnsOne(t *testing.T) {
	cmd := newRootCommand()
	cmd.SetArgs([]string{"does-not-exist"})

	stderr := captureStderr(t, func() {
		if got := execute(cmd); got != 1 {
			t.Fatalf("execute() = %d, want 1", got)
		}
	})

	if got := strings.Count(stderr, "==> ERROR:"); got != 1 {
		t.Fatalf("diagnostic count = %d, want 1; stderr = %q", got, stderr)
	}
	if !strings.Contains(stderr, `Error: unknown command "does-not-exist"`) {
		t.Fatalf("stderr = %q, want existing-style unknown command diagnostic", stderr)
	}
}

func TestCompletionErrorsAreReturned(t *testing.T) {
	t.Run("missing shell", func(t *testing.T) {
		err := executeArgs(t, "completion")
		if err == nil || !strings.Contains(err.Error(), "Please specify bash or zsh") {
			t.Fatalf("Execute() error = %v, want missing shell diagnostic", err)
		}
	})

	t.Run("unsupported shell", func(t *testing.T) {
		err := executeArgs(t, "completion", "fish")
		if err == nil || !strings.Contains(err.Error(), "Unsupported shell: fish") {
			t.Fatalf("Execute() error = %v, want unsupported shell diagnostic", err)
		}
	})

	t.Run("generator failure", func(t *testing.T) {
		cmd := newRootCommand()
		cmd.SetOut(errorWriter{})
		cmd.SetArgs([]string{"completion", "bash"})

		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "Failed to generate bash completion") {
			t.Fatalf("Execute() error = %v, want generator failure", err)
		}
	})
}

func TestDownstreamSearchFailureIsReturned(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}))
	defer server.Close()

	err := executeArgs(t, "search", "--server", server.URL, "--query-pattern", "Higgs")
	if err == nil || !strings.Contains(err.Error(), "Search failed: server returned status 502") {
		t.Fatalf("Execute() error = %v, want downstream search diagnostic", err)
	}
}

func TestDownloadFailureRunsDeferredCleanup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"1","metadata":{"recid":1,"files":[{"uri":"root://example.test/data.root","size":1,"checksum":"adler32:00000001"}]}}`)
	}))
	defer server.Close()

	fake := &failingXRootDDownloader{}
	cmd := newDownloadFilesCommandWithXRootD(func() xrootdFileDownloader { return fake })
	cmd.SetArgs([]string{
		"--recid", "1",
		"--server", server.URL,
		"--download-engine", "xrootd",
		"--no-expand",
		"--output-dir", t.TempDir(),
	})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "1 file(s) failed to download") {
		t.Fatalf("Execute() error = %v, want download failure", err)
	}
	if !fake.closed {
		t.Fatal("XRootD downloader Close was not called after returned error")
	}
}

func TestRootConstructorsDoNotShareFlagState(t *testing.T) {
	first := newRootCommand()
	first.SetArgs([]string{"search", "--filter", "name=value"})
	if err := first.Execute(); err == nil {
		t.Fatal("first Execute() succeeded, want filter validation error")
	}

	second := newRootCommand()
	second.SetArgs([]string{"version"})
	if err := second.Execute(); err != nil {
		t.Fatalf("second Execute() error = %v, want isolated flag state", err)
	}
}

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

type failingXRootDDownloader struct {
	closed bool
}

func (d *failingXRootDDownloader) DownloadFiles(
	context.Context,
	[]filemetadata.File,
	string,
	int,
	int,
	bool,
	bool,
	bool,
) downloader.DownloadStats {
	return downloader.DownloadStats{TotalFiles: 1, TotalBytes: 1, FailedFiles: 1}
}

func (d *failingXRootDDownloader) Close() error {
	d.closed = true
	return nil
}

func captureStderr(t *testing.T, run func()) string {
	t.Helper()

	oldStderr := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stderr = writer
	t.Cleanup(func() {
		os.Stderr = oldStderr
		_ = reader.Close()
		_ = writer.Close()
	})

	run()
	if err := writer.Close(); err != nil {
		t.Fatalf("closing stderr writer: %v", err)
	}
	os.Stderr = oldStderr

	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("reading stderr: %v", err)
	}
	return string(output)
}
