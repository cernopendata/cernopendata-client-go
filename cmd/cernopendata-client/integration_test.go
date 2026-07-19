//go:build integration

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/adler32"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const (
	testRecID            = "3005"
	testUnavailableRecID = "8886"
)

type liveService string

const (
	liveCERNHTTP liveService = "CERN HTTP"
	liveGitHub   liveService = "GitHub"
	liveXRootD   liveService = "XRootD"
)

func environmentalFailureReason(service liveService, err, contextErr error, output string) (string, bool) {
	if err == nil {
		return "", false
	}

	if errors.Is(err, context.DeadlineExceeded) || errors.Is(contextErr, context.DeadlineExceeded) {
		return "request timed out", true
	}

	details := strings.ToLower(err.Error() + "\n" + output)
	transportFailures := []struct {
		pattern string
		reason  string
	}{
		{"context deadline exceeded", "request timed out"},
		{"i/o timeout", "request timed out"},
		{"operation timed out", "request timed out"},
		{"no such host", "DNS lookup failed"},
		{"temporary failure in name resolution", "DNS lookup failed"},
		{"server misbehaving", "DNS lookup failed"},
		{"tls handshake timeout", "TLS handshake failed"},
		{"remote error: tls", "TLS handshake failed"},
		{"x509:", "TLS certificate validation failed"},
		{"certificate signed by unknown authority", "TLS certificate validation failed"},
		{"connection refused", "connection failed"},
		{"connection reset by peer", "connection failed"},
		{"network is unreachable", "connection failed"},
		{"no route to host", "connection failed"},
		{"connection timed out", "connection failed"},
	}
	for _, failure := range transportFailures {
		if strings.Contains(details, failure.pattern) {
			return failure.reason, true
		}
	}

	switch service {
	case liveGitHub:
		for _, status := range []string{"github api returned status 403", "github api returned status 429"} {
			if strings.Contains(details, status) {
				return status, true
			}
		}
	case liveCERNHTTP:
		for _, status := range []string{"429", "502", "503", "504"} {
			if strings.Contains(details, "server returned status "+status) ||
				strings.Contains(details, "server error: "+status) ||
				strings.Contains(details, "server returned "+status+":") {
				return "CERN HTTP status " + status, true
			}
		}
	case liveXRootD:
		for _, pattern := range []string{
			"failed to create xrootd client",
			"could not connect to xrootd server",
			"xrootd: failed to connect",
			"xrootd: connection closed",
			"xrootd: transport error",
			"xrootd: connection: wrong handshake",
		} {
			if strings.Contains(details, pattern) {
				return "XRootD transport failed", true
			}
		}
	}

	return "", false
}

func assertLiveCommandSuccess(
	t *testing.T,
	service liveService,
	err, contextErr error,
	output string,
	description string,
) string {
	t.Helper()
	if err != nil {
		if reason, recognized := environmentalFailureReason(service, err, contextErr, output); recognized {
			t.Skipf("Skipping %s after recognized %s environmental failure (%s): %v\nOutput: %s", description, service, reason, err, output)
		}
		t.Fatalf("%s failed unexpectedly: %v\nOutput: %s", description, err, output)
	}
	if strings.TrimSpace(output) == "" {
		t.Fatalf("%s returned empty output", description)
	}
	return output
}

func assertCommandErrorContains(
	t *testing.T,
	err error,
	output string,
	description string,
	want ...string,
) string {
	t.Helper()
	if err == nil {
		t.Fatalf("Expected %s to fail, but it succeeded\nOutput: %s", description, output)
	}
	if strings.TrimSpace(output) == "" {
		t.Fatalf("%s failed without the expected product diagnostic: %v", description, err)
	}
	for _, fragment := range want {
		if !strings.Contains(output, fragment) {
			t.Fatalf("%s returned the wrong product diagnostic; expected %q\nOutput: %s", description, fragment, output)
		}
	}
	return output
}

func TestEnvironmentalFailureReason(t *testing.T) {
	tests := []struct {
		name       string
		service    liveService
		err        error
		contextErr error
		output     string
		want       bool
	}{
		{name: "context timeout", service: liveCERNHTTP, err: errors.New("signal: killed"), contextErr: context.DeadlineExceeded, want: true},
		{name: "reported timeout", service: liveCERNHTTP, err: errors.New("exit status 1"), output: "dial tcp: i/o timeout", want: true},
		{name: "DNS", service: liveCERNHTTP, err: errors.New("exit status 1"), output: "lookup opendata.cern.ch: no such host", want: true},
		{name: "TLS", service: liveCERNHTTP, err: errors.New("exit status 1"), output: "tls handshake timeout", want: true},
		{name: "connect", service: liveCERNHTTP, err: errors.New("exit status 1"), output: "dial tcp: connection refused", want: true},
		{name: "XRootD transport", service: liveXRootD, err: errors.New("exit status 1"), output: "xrootd: transport error: socket closed", want: true},
		{name: "XRootD connect", service: liveXRootD, err: errors.New("exit status 1"), output: `xrdio: could not connect to xrootd server "eospublic.cern.ch:1094"`, want: true},
		{name: "GitHub forbidden", service: liveGitHub, err: errors.New("exit status 1"), output: "GitHub API returned status 403", want: true},
		{name: "GitHub rate limit", service: liveGitHub, err: errors.New("exit status 1"), output: "GitHub API returned status 429", want: true},
		{name: "CERN rate limit", service: liveCERNHTTP, err: errors.New("exit status 1"), output: "server returned status 429 for search", want: true},
		{name: "CERN bad gateway", service: liveCERNHTTP, err: errors.New("exit status 1"), output: "server returned status 502 for record 3005", want: true},
		{name: "CERN unavailable", service: liveCERNHTTP, err: errors.New("exit status 1"), output: "server returned status 503 for search", want: true},
		{name: "CERN binary unavailable", service: liveCERNHTTP, err: errors.New("exit status 1"), output: "Server error: 503", want: true},
		{name: "CERN download unavailable", service: liveCERNHTTP, err: errors.New("exit status 1"), output: "server returned 503: service unavailable", want: true},
		{name: "CERN gateway timeout", service: liveCERNHTTP, err: errors.New("exit status 1"), output: "server returned status 504 for search", want: true},
		{name: "success cannot be environmental", service: liveCERNHTTP, output: "server returned status 503 for search", want: false},
		{name: "unexpected CERN status", service: liveCERNHTTP, err: errors.New("exit status 1"), output: "server returned status 500 for search", want: false},
		{name: "unbounded CERN status", service: liveCERNHTTP, err: errors.New("exit status 1"), output: "downloaded 503 bytes before failure", want: false},
		{name: "GitHub status on CERN request", service: liveCERNHTTP, err: errors.New("exit status 1"), output: "GitHub API returned status 403", want: false},
		{name: "CERN status on GitHub request", service: liveGitHub, err: errors.New("exit status 1"), output: "server returned status 503 for search", want: false},
		{name: "XRootD missing fixture", service: liveXRootD, err: errors.New("exit status 1"), output: "xrootd: error 3011: no such file or directory", want: false},
		{name: "unexpected command error", service: liveCERNHTTP, err: errors.New("exit status 1"), output: "failed to parse output", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got := environmentalFailureReason(tt.service, tt.err, tt.contextErr, tt.output)
			if got != tt.want {
				t.Fatalf("environmentalFailureReason() recognized = %v, want %v", got, tt.want)
			}
		})
	}
}

func getBinaryPath() string {
	_, callerFile, _, _ := runtime.Caller(0)
	projectRoot := filepath.Dir(filepath.Dir(callerFile))
	projectRoot = filepath.Dir(projectRoot)
	return filepath.Join(projectRoot, "cernopendata-client")
}

func runIntegrationCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func assertCommandSuccess(t *testing.T, args ...string) string {
	t.Helper()
	output, err := runIntegrationCommand(t, args...)
	return assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, output, fmt.Sprintf("command %v", args))
}

func assertCERNCommandError(t *testing.T, want string, args ...string) string {
	t.Helper()
	output, err := runIntegrationCommand(t, args...)
	return assertCommandErrorContains(t, err, output, fmt.Sprintf("command %v", args), want)
}

func TestIntegrationGetMetadata(t *testing.T) {
	assertCommandSuccess(t, "get-metadata", "--recid", testRecID)
}

func TestIntegrationGetMetadataByDOI(t *testing.T) {
	assertCommandSuccess(t, "get-metadata", "--doi", "10.7483/OPENDATA.CMS.A342.9982")
}

func TestIntegrationGetMetadataByTitle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-metadata", "--title", "Configuration file for LHE step HIG-Summer11pLHE-00114_1_cfg.py")
	output, err := cmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "get-metadata by title")
}

func TestIntegrationGetMetadataByTitleWrong(t *testing.T) {
	assertCERNCommandError(t, "no record found with title: NONEXISTING_TITLE", "get-metadata", "--title", "NONEXISTING_TITLE")
}

func TestIntegrationGetMetadataByAlternateDOI(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-metadata", "--doi", "10.7483/OPENDATA.CMS.A342.9982")
	output, err := cmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "get-metadata by alternate DOI")
}

func TestIntegrationGetMetadataOutputValueBasic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-metadata", "--recid", "1", "--output-value", "system_details.global_tag")
	output, err := cmd.CombinedOutput()
	outputStr := assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "get-metadata with output value")
	if !contains(outputStr, "FT_R_42_V10A::All") {
		t.Error("Expected 'FT_R_42_V10A::All' in output")
	}
}

func TestIntegrationGetMetadataOutputValueArray(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-metadata", "--recid", "1", "--output-value", "usage.links")
	output, err := cmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "get-metadata with array output value")
}

func TestIntegrationGetMetadataOutputValueNested(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-metadata", "--recid", "1", "--output-value", "usage.links.url")
	output, err := cmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "get-metadata with nested output value")
}

func TestIntegrationGetMetadataOutputValueWrong(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-metadata", "--recid", "1", "--output-value", "title.global_tag")
	output, err := cmd.CombinedOutput()
	assertCommandErrorContains(t, err, string(output), "get-metadata with an invalid nested field", "Field not found", "cannot access field global_tag on non-map type")
}

func TestIntegrationGetMetadataNoIdentifier(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-metadata")
	output, err := cmd.CombinedOutput()
	assertCommandErrorContains(t, err, string(output), "get-metadata without an identifier", "please provide recid, doi, or title")
}

func TestIntegrationGetMetadataInvalidServer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-metadata", "--recid", testRecID, "--server", "ftp://invalid.com")
	output, err := cmd.CombinedOutput()
	assertCommandErrorContains(t, err, string(output), "get-metadata with an invalid server scheme", "Failed to get metadata", "unsupported protocol scheme \"ftp\"")
}

func TestIntegrationGetMetadataFilterWithoutOutputValue(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-metadata", "--recid", "1", "--filter", "foo=bar")
	output, err := cmd.CombinedOutput()
	assertCommandErrorContains(t, err, string(output), "get-metadata filter without output value", "--filter can only be used with --output-value")
}

func TestIntegrationGetFileLocations(t *testing.T) {
	assertCommandSuccess(t, "get-file-locations", "--recid", testRecID)
}

func TestIntegrationGetFileLocationsNoExpand(t *testing.T) {
	assertCommandSuccess(t, "get-file-locations", "--recid", testRecID, "--no-expand")
}

func TestIntegrationGetFileLocationsVerbose(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-file-locations", "--recid", testRecID, "--verbose")
	output, err := cmd.CombinedOutput()
	outputStr := assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "verbose get-file-locations")
	if len(outputStr) < 10 {
		t.Error("Expected verbose output to have more content")
	}
}

func TestIntegrationGetFileLocationsByRecidWrong(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-file-locations", "--recid", "0")
	output, err := cmd.CombinedOutput()
	assertCommandErrorContains(t, err, string(output), "get-file-locations with recid zero", "please provide recid, doi, or title")
}

func TestIntegrationGetFileLocationsByDOI(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-file-locations", "--doi", "10.7483/OPENDATA.CMS.A342.9982")
	output, err := cmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "get-file-locations by DOI")
}

func TestIntegrationGetFileLocationsByDOIWrong(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-file-locations", "--doi", "NONEXISTING")
	output, err := cmd.CombinedOutput()
	assertCommandErrorContains(t, err, string(output), "get-file-locations with a nonexistent DOI", "no record found with doi: NONEXISTING")
}

func TestIntegrationGetFileLocationsByTitle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-file-locations", "--title", "Configuration file for LHE step HIG-Summer11pLHE-00114_1_cfg.py")
	output, err := cmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "get-file-locations by title")
}

func TestIntegrationGetFileLocationsByTitleWrong(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-file-locations", "--title", "NONEXISTING_TITLE")
	output, err := cmd.CombinedOutput()
	assertCommandErrorContains(t, err, string(output), "get-file-locations with a nonexistent title", "no record found with title: NONEXISTING_TITLE")
}

func TestIntegrationGetFileLocationsHTTP(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-file-locations", "--recid", testRecID, "--protocol", "http")
	output, err := cmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "get-file-locations with HTTP URLs")
}

func TestIntegrationGetFileLocationsHTTPS(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-file-locations", "--recid", testRecID, "--protocol", "https")
	output, err := cmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "get-file-locations with HTTPS URLs")
}

func TestIntegrationGetFileLocationsExpand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-file-locations", "--recid", testRecID, "--expand")
	output, err := cmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "expanded get-file-locations")
}

func TestIntegrationGetFileLocationsNoIdentifier(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-file-locations")
	output, err := cmd.CombinedOutput()
	assertCommandErrorContains(t, err, string(output), "get-file-locations without an identifier", "please provide recid, doi, or title")
}

func TestIntegrationGetFileLocationsInvalidServer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-file-locations", "--recid", testRecID, "--server", "ftp://invalid.com")
	output, err := cmd.CombinedOutput()
	assertCommandErrorContains(t, err, string(output), "get-file-locations with an invalid server scheme", "Failed to get record", "unsupported protocol scheme \"ftp\"")
}

func TestIntegrationVersion(t *testing.T) {
	assertCommandSuccess(t, "version")
}

func TestIntegrationDownloadFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--recid", testRecID, "--filter-name", "*.txt", "--output-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	assertCommandErrorContains(t, err, string(output), "download-files with an unmatched name filter", "No files matching filters")

	files := mustReadDir(t, tmpDir)

	if len(files) != 0 {
		t.Fatalf("Expected no files for the unmatched filter, got %d", len(files))
	}
}

func TestIntegrationDownloadFilesByDOI(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--doi", "10.7483/OPENDATA.CMS.W26R.J96R", "--filter-name", "readme.txt", "--output-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "download-files by DOI")

	files := mustReadDir(t, tmpDir)

	if len(files) != 1 || files[0].Name() != "readme.txt" {
		t.Fatalf("Expected exactly readme.txt from DOI fixture, got %v", files)
	}
}

func TestIntegrationDownloadFilesByDOIWrong(t *testing.T) {
	assertCERNCommandError(t, "no record found with doi: NONEXISTING", "download-files", "--doi", "NONEXISTING")
}

func TestIntegrationListDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// #nosec G204
	cmd := exec.CommandContext(ctx, getBinaryPath(), "list-directory", "/eos/opendata/cms")
	output, err := cmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveXRootD, err, ctx.Err(), string(output), "list-directory")
}

func TestIntegrationListDirectoryVerbose(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// #nosec G204
	cmd := exec.CommandContext(ctx, getBinaryPath(), "list-directory", "/eos/opendata/cms", "--verbose")
	output, err := cmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveXRootD, err, ctx.Err(), string(output), "verbose list-directory")

	outputStr := string(output)
	if len(outputStr) < 20 {
		t.Error("Expected verbose output to have more content")
	}
}

func TestIntegrationListDirectoryWrongPath(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// #nosec G204
	cmd := exec.CommandContext(ctx, getBinaryPath(), "list-directory", "/eos/opendata/foobar")
	output, err := cmd.CombinedOutput()
	assertCommandErrorContains(t, err, string(output), "list-directory with a nonexistent path", "Failed to list directory", "No such file or directory")
}

func TestIntegrationListDirectoryEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// #nosec G204
	cmd := exec.CommandContext(ctx, getBinaryPath(), "list-directory", "/eos/opendata/test/nonexistent")
	output, err := cmd.CombinedOutput()
	assertCommandErrorContains(t, err, string(output), "list-directory with the nonexistent test path", "Failed to list directory", "No such file or directory")
}

func TestIntegrationHelp(t *testing.T) {
	output := assertCommandSuccess(t, "--help")
	if !strings.Contains(output, "Usage:") {
		t.Error("Expected 'Usage:' in help output")
	}
}

func TestIntegrationBinaryExists(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	_, callerFile, _, _ := runtime.Caller(0)
	projectRoot := filepath.Dir(filepath.Dir(callerFile))
	projectRoot = filepath.Dir(projectRoot)
	binaryPath := filepath.Join(projectRoot, "cernopendata-client")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Errorf("Binary does not exist at %s. Run 'make build' first.", binaryPath)
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func adlerChecksum(content []byte) string {
	return fmt.Sprintf("adler32:%08x", adler32.Checksum(content))
}

func mustReadDir(t *testing.T, path string) []os.DirEntry {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatalf("Failed to read directory %s: %v", path, err)
	}
	return entries
}

func TestIntegrationDownloadFilesFromRecid(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--recid", testRecID, "--filter-name", "*.py", "--output-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	outputStr := assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "download-files by recid")
	if !contains(outputStr, "Success") {
		t.Error("Expected 'Success!' message in output")
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "0d0714743f0204ed3c0144941e6ce248.configFile.py")); err != nil {
		t.Fatalf("Expected fixture file to be downloaded: %v", err)
	}
}

func TestIntegrationDownloadFilesDuplicateBasenamesWithLocalServer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	const recid = 11537

	fileA := []byte("content from 0002")
	fileB := []byte("content from 0003")

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case fmt.Sprintf("/api/records/%d", recid):
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": fmt.Sprintf("%d", recid),
				"metadata": map[string]any{
					"recid": recid,
					"_file_indices": []any{
						map[string]any{
							"key":  "index-1",
							"size": len(fileA) + len(fileB),
							"files": []any{
								map[string]any{
									"uri":      server.URL + "/files/0002/AO2D.root",
									"size":     len(fileA),
									"checksum": adlerChecksum(fileA),
								},
								map[string]any{
									"uri":      server.URL + "/files/0003/AO2D.root",
									"size":     len(fileB),
									"checksum": adlerChecksum(fileB),
								},
							},
						},
					},
				},
			})
		case "/files/0002/AO2D.root":
			_, _ = w.Write(fileA)
		case "/files/0003/AO2D.root":
			_, _ = w.Write(fileB)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	tmpDir := t.TempDir()

	// #nosec G204
	cmd := exec.Command(
		getBinaryPath(),
		"download-files",
		"--recid", fmt.Sprintf("%d", recid),
		"--server", server.URL,
		"--output-dir", tmpDir,
		"--verify",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("download-files against local server failed: %v\nOutput: %s", err, string(output))
	}

	for relPath, want := range map[string]string{
		"0002/AO2D.root": string(fileA),
		"0003/AO2D.root": string(fileB),
	} {
		// #nosec G304 -- relPath is constrained to fixed test cases in this table
		got, readErr := os.ReadFile(filepath.Join(tmpDir, relPath))
		if readErr != nil {
			t.Fatalf("failed to read downloaded file %s: %v", relPath, readErr)
		}
		if string(got) != want {
			t.Errorf("downloaded content for %s = %q, want %q", relPath, string(got), want)
		}
	}

	outputStr := string(output)
	if !contains(outputStr, "Verified: 0002/AO2D.root") {
		t.Errorf("expected verification output for 0002/AO2D.root, got:\n%s", outputStr)
	}
	if !contains(outputStr, "Verified: 0003/AO2D.root") {
		t.Errorf("expected verification output for 0003/AO2D.root, got:\n%s", outputStr)
	}
	if !contains(outputStr, "Success") {
		t.Errorf("expected success output, got:\n%s", outputStr)
	}
}

func TestIntegrationDownloadFilesFromRecidWrong(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--recid", "0")
	output, err := cmd.CombinedOutput()
	assertCommandErrorContains(t, err, string(output), "download-files with recid zero", "please provide recid, doi, or title")
}

func TestIntegrationDownloadFilesFilterName(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--recid", "5500", "--filter-name", "BuildFile.xml", "--output-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	outputStr := assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "download-files with a name filter")
	if !contains(outputStr, "Success") {
		t.Error("Expected 'Success!' message in output")
	}

	files := mustReadDir(t, tmpDir)
	found := false
	for _, f := range files {
		if contains(f.Name(), "BuildFile") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected BuildFile.xml to be downloaded")
	}
}

func TestIntegrationDownloadFilesFilterNameWrong(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--recid", "5500", "--filter-name", "nonexistent.txt", "--output-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	assertCommandErrorContains(t, err, string(output), "download-files with a non-matching name filter", "No files matching filters")
}

func TestIntegrationDownloadFilesFilterRange(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--recid", "5500", "--filter-range", "0-2", "--output-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "download-files with a range filter")

	files := mustReadDir(t, tmpDir)
	if len(files) == 0 {
		t.Error("Expected some files to be downloaded with range filter")
	}
}

func TestIntegrationDownloadFilesFilterRangeInvalid(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--recid", "5500", "--filter-range", "5-2", "--output-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	assertCommandErrorContains(t, err, string(output), "download-files with an invalid range", "Invalid range filter", "range end must be >= start")

	files := mustReadDir(t, tmpDir)
	if len(files) > 0 {
		t.Error("Expected no files to be downloaded with invalid range")
	}
}

func TestIntegrationDownloadFilesRetry(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--recid", testRecID, "--retry-limit", "2", "--filter-name", "*.py", "--output-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	outputStr := assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "download-files with retries")
	if !contains(outputStr, "Success") {
		t.Error("Expected 'Success!' message in output")
	}
}

func TestIntegrationDownloadFilesVerbose(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--recid", testRecID, "--filter-name", "*.py", "--verbose", "--output-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "verbose download-files")
}

func TestIntegrationDownloadFilesNoIdentifier(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files")
	output, err := cmd.CombinedOutput()
	assertCommandErrorContains(t, err, string(output), "download-files without an identifier", "please provide recid, doi, or title")
}

func TestIntegrationDownloadFilesInvalidServer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--recid", testRecID, "--server", "ftp://invalid.com", "--dry-run")
	output, err := cmd.CombinedOutput()
	assertCommandErrorContains(t, err, string(output), "download-files with an invalid server scheme", "Failed to get record", "unsupported protocol scheme \"ftp\"")
}

func TestIntegrationDownloadFilesCustomOutputDir(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--recid", testRecID, "--filter-name", "*.py", "--output-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "download-files with a custom output directory")

	files := mustReadDir(t, tmpDir)
	if len(files) == 0 {
		t.Error("Expected files to be downloaded to custom output directory")
	}
}

func TestIntegrationVerifyFilesBasic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	downloadCmd := exec.Command(getBinaryPath(), "download-files", "--recid", testRecID, "--filter-name", "*.py", "--output-dir", tmpDir)
	downloadOutput, downloadErr := downloadCmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveCERNHTTP, downloadErr, nil, string(downloadOutput), "download fixture for verification")

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "verify-files", "--recid", testRecID, "--input-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	outputStr := assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "verify-files")
	if !contains(outputStr, "Verification summary") {
		t.Error("Expected verification summary in output")
	}
	if !contains(outputStr, "Total files") {
		t.Error("Expected 'Total files' in verification output")
	}
	if !contains(outputStr, "Verified") {
		t.Error("Expected 'Verified' in verification output")
	}
}

func TestIntegrationVerifyFilesByNameFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	downloadCmd := exec.Command(getBinaryPath(), "download-files", "--recid", testRecID, "--filter-name", "*.py", "--output-dir", tmpDir)
	downloadOutput, downloadErr := downloadCmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveCERNHTTP, downloadErr, nil, string(downloadOutput), "download fixture for filtered verification")

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "verify-files", "--recid", testRecID, "--input-dir", tmpDir, "--filter-name", "*.py")
	output, err := cmd.CombinedOutput()
	outputStr := assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "verify-files with a name filter")
	if !contains(outputStr, "Verification summary") {
		t.Error("Expected verification summary in output")
	}
}

func TestIntegrationVerifyFilesNoIdentifier(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "verify-files")
	output, err := cmd.CombinedOutput()
	assertCommandErrorContains(t, err, string(output), "verify-files without an identifier", "please provide recid, doi, or title")
}

func TestIntegrationVerifyFilesByDOI(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Note: recid 3005 has no DOI in its metadata, so we use recid 5500 which has a valid DOI (10.7483/OPENDATA.CMS.JKB8.RR42)
	// We download all files (not just *.py) to ensure verification can find all expected files
	tmpDir := t.TempDir()

	// #nosec G204
	downloadCmd := exec.Command(getBinaryPath(), "download-files", "--recid", "5500", "--output-dir", tmpDir)
	downloadOutput, downloadErr := downloadCmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveCERNHTTP, downloadErr, nil, string(downloadOutput), "download DOI verification fixture")

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "verify-files", "--doi", "10.7483/OPENDATA.CMS.JKB8.RR42", "--input-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "verify-files by DOI")
}

func TestIntegrationVerifyFilesByTitle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Note: API requires exact title match. recid 3005's exact title is "Configuration file for LHE step HIG-Summer11pLHE-00114_1_cfg.py"
	// Using the partial title "Configuration file for LHE step" will not match
	tmpDir := t.TempDir()

	// #nosec G204
	downloadCmd := exec.Command(getBinaryPath(), "download-files", "--recid", testRecID, "--filter-name", "*.py", "--output-dir", tmpDir)
	downloadOutput, downloadErr := downloadCmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveCERNHTTP, downloadErr, nil, string(downloadOutput), "download title verification fixture")

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "verify-files", "--title", "Configuration file for LHE step HIG-Summer11pLHE-00114_1_cfg.py", "--input-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "verify-files by title")
}

func TestIntegrationVerifyFilesInvalidServer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "verify-files", "--recid", testRecID, "--server", "ftp://invalid.com")
	output, err := cmd.CombinedOutput()
	assertCommandErrorContains(t, err, string(output), "verify-files with an invalid server scheme", "Failed to get record", "unsupported protocol scheme \"ftp\"")
}

func TestIntegrationVerifyFilesCustomInputDir(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	downloadCmd := exec.Command(getBinaryPath(), "download-files", "--recid", testRecID, "--filter-name", "*.py", "--output-dir", tmpDir)
	downloadOutput, downloadErr := downloadCmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveCERNHTTP, downloadErr, nil, string(downloadOutput), "download custom-input verification fixture")

	customDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(customDir, 0750); err != nil {
		t.Fatalf("Failed to create custom directory: %v", err)
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "verify-files", "--recid", testRecID, "--input-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	outputStr := assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "verify-files with a custom input directory")
	if !contains(outputStr, "Verification summary") {
		t.Error("Expected verification summary in output")
	}
}

func TestIntegrationDownloadFilesRegexp(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--recid", testRecID, "--filter-regexp", ".*\\.py$", "--output-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "download-files with a regular-expression filter")

	files := mustReadDir(t, tmpDir)
	if len(files) == 0 {
		t.Error("Expected some Python files to be downloaded with regex filter")
	}

	for _, f := range files {
		if !contains(f.Name(), ".py") {
			t.Errorf("Expected only .py files, got: %s", f.Name())
		}
	}
}

func TestIntegrationDownloadFilesRegexpMultiple(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--recid", testRecID, "--filter-regexp", "(.*\\.py$|.*\\.txt$)", "--output-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "download-files with multiple regular-expression filters")

	files := mustReadDir(t, tmpDir)
	if len(files) == 0 {
		t.Error("Expected some files to be downloaded with multiple regex filter")
	}
}

func TestIntegrationDownloadFilesRegexpWrong(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--recid", testRecID, "--filter-regexp", "nonexistentfile.*", "--output-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	assertCommandErrorContains(t, err, string(output), "download-files with an unmatched regular expression", "No files matching filters")

	files := mustReadDir(t, tmpDir)
	if len(files) > 0 {
		t.Error("Expected no files to be downloaded with non-matching regex filter")
	}
}

func TestIntegrationDownloadFilesMultipleNameFilters(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--recid", testRecID, "--filter-name", "*.py,*.txt", "--output-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "download-files with multiple name filters")

	files := mustReadDir(t, tmpDir)
	if len(files) == 0 {
		t.Error("Expected some files to be downloaded with multiple name filters")
	}

	for _, f := range files {
		ext := filepath.Ext(f.Name())
		if ext != ".py" && ext != ".txt" {
			t.Errorf("Expected only .py or .txt files, got: %s", f.Name())
		}
	}
}

func TestIntegrationDownloadFilesMultipleRanges(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--recid", "5500", "--filter-range", "0-1,3-4", "--output-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "download-files with multiple ranges")

	files := mustReadDir(t, tmpDir)
	if len(files) < 2 {
		t.Error("Expected at least 2 files to be downloaded with multiple ranges (0-1,3-4)")
	}
}

func TestIntegrationDownloadFilesRegexpAndRange(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--recid", testRecID, "--filter-regexp", ".*\\.py$", "--filter-range", "0-2", "--output-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "download-files with regular-expression and range filters")

	files := mustReadDir(t, tmpDir)

	for _, f := range files {
		if !contains(f.Name(), ".py") {
			t.Errorf("Expected only .py files with regexp filter, got: %s", f.Name())
		}
	}
}

func TestIntegrationDownloadFilesRegexpAndMultipleRanges(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--recid", "5500", "--filter-regexp", ".*\\.xml$", "--filter-range", "0-1,3-4", "--output-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "download-files with regular-expression and multiple-range filters")

	files := mustReadDir(t, tmpDir)

	for _, f := range files {
		if !contains(f.Name(), ".xml") {
			t.Errorf("Expected only .xml files with regexp filter, got: %s", f.Name())
		}
	}
}

func TestIntegrationListDirectoryRecursive(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "list-directory", "/eos/opendata/cms/software/HiggsExample20112012", "--recursive")
	output, err := cmd.CombinedOutput()
	outputStr := assertLiveCommandSuccess(t, liveXRootD, err, nil, string(output), "recursive list-directory")

	entries := strings.Split(outputStr, "\n")
	if len(entries) < 10 {
		t.Errorf("Expected at least 10 entries from recursive listing, got: %d", len(entries))
	}
}

func TestIntegrationListDirectoryTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Exercise a bounded XRootD listing. A recognized transport outage may skip;
	// otherwise the command must complete successfully with nonempty output.
	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "list-directory", "/eos/opendata/cms/software/HiggsExample20112012", "--timeout", "5")
	output, err := cmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveXRootD, err, nil, string(output), "list-directory with timeout flag")
}

func TestIntegrationDownloadFilesWithVerify(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--recid", testRecID, "--filter-name", "*.py", "--verify", "--output-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	outputStr := assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "download-files with verification")
	if !contains(outputStr, "Success") {
		t.Error("Expected 'Success!' message in output")
	}

	if !contains(outputStr, "Verifying") {
		t.Error("Expected verification message in output")
	}
}

func TestIntegrationDownloadFilesWithDownloadEngine(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--recid", testRecID, "--filter-name", "*.py", "--download-engine", "http", "--output-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	outputStr := assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "download-files with the HTTP engine")
	if !contains(outputStr, "Success") {
		t.Error("Expected 'Success!' message in output")
	}
}

func TestIntegrationDownloadFilesWithRetrySleep(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--recid", testRecID, "--filter-name", "*.py", "--retry-sleep", "2", "--output-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	outputStr := assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "download-files with retry sleep")
	if !contains(outputStr, "Success") {
		t.Error("Expected 'Success!' message in output")
	}
}

func TestIntegrationDownloadFilesWithXRootD(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--recid", testRecID, "--filter-name", "*.py", "--download-engine", "xrootd", "--output-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	outputStr := assertLiveCommandSuccess(t, liveXRootD, err, nil, string(output), "XRootD download")
	if !contains(outputStr, "Success") {
		t.Error("Expected 'Success!' message in output")
	}

	if !contains(outputStr, "Downloading file") {
		t.Error("Expected download progress messages")
	}

	if !contains(outputStr, "Download summary") {
		t.Error("Expected download summary")
	}
}

func TestIntegrationDownloadFilesXRootDError(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--recid", testRecID, "--filter-name", "*.py", "--download-engine", "xrootd", "--server", "http://invalid.cern.ch", "--output-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	assertCommandErrorContains(t, err, string(output), "XRootD download with an invalid metadata server", "Failed to get record")
}

func TestIntegrationSearchBasic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// #nosec G204
	cmd := exec.CommandContext(ctx, getBinaryPath(), "search", "--query-pattern", "Higgs", "--size", "3")
	output, err := cmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveCERNHTTP, err, ctx.Err(), string(output), "basic search")
}

func TestIntegrationSearchWithFacets(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// #nosec G204
	cmd := exec.CommandContext(ctx, getBinaryPath(), "search", "--query-pattern", "muon", "--query-facet", "experiment=CMS", "--size", "5")
	output, err := cmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveCERNHTTP, err, ctx.Err(), string(output), "search with facets")
}

func TestIntegrationSearchWithURL(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// #nosec G204
	cmd := exec.CommandContext(ctx, getBinaryPath(), "search", "--query", "q=test&f=experiment:CMS", "--size", "3")
	output, err := cmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveCERNHTTP, err, ctx.Err(), string(output), "search with URL query")
}

func TestIntegrationSearchOutputValue(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// #nosec G204
	cmd := exec.CommandContext(ctx, getBinaryPath(), "search", "--query-pattern", "Higgs", "--output-value", "title", "--size", "3")
	output, err := cmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveCERNHTTP, err, ctx.Err(), string(output), "search with output value")
}

func TestIntegrationSearchFormatJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// #nosec G204
	cmd := exec.CommandContext(ctx, getBinaryPath(), "search", "--query-pattern", "Higgs", "--output-value", "title", "--format", "json", "--size", "3")
	output, err := cmd.CombinedOutput()
	outputStr := assertLiveCommandSuccess(t, liveCERNHTTP, err, ctx.Err(), string(output), "JSON search")
	var titles []string
	if err := json.Unmarshal([]byte(outputStr), &titles); err != nil {
		t.Fatalf("JSON search returned malformed output: %v\nOutput: %s", err, outputStr)
	}
	if len(titles) == 0 {
		t.Fatal("JSON search returned an empty result fixture")
	}
}

func TestIntegrationSearchNoResults(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// #nosec G204
	cmd := exec.CommandContext(ctx, getBinaryPath(), "search", "--query-pattern", "xyznonexistent12345")
	output, err := cmd.CombinedOutput()
	outputStr := assertLiveCommandSuccess(t, liveCERNHTTP, err, ctx.Err(), string(output), "search with no results")
	if !contains(outputStr, "No records found") {
		t.Fatalf("Expected the no-results product diagnostic\nOutput: %s", outputStr)
	}
}

func TestIntegrationSearchHelp(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "search", "--help")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run search --help: %v\nOutput: %s", err, string(output))
	}

	outputStr := string(output)
	if !contains(outputStr, "--query-pattern") {
		t.Error("Expected --query-pattern in help output")
	}
	if !contains(outputStr, "--query-facet") {
		t.Error("Expected --query-facet in help output")
	}
	if !contains(outputStr, "--output-value") {
		t.Error("Expected --output-value in help output")
	}
	if !contains(outputStr, "--size") {
		t.Error("Expected --size in help output")
	}
}

func TestIntegrationSearchFilterWithoutOutputValue(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "search", "--query-pattern", "test", "--filter", "foo=bar")
	output, err := cmd.CombinedOutput()
	assertCommandErrorContains(t, err, string(output), "search filter without output value", "--filter can only be used with --output-value")
}

func TestIntegrationSearchListFacets(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// #nosec G204
	cmd := exec.CommandContext(ctx, getBinaryPath(), "search", "--list-facets")
	output, err := cmd.CombinedOutput()
	outputStr := assertLiveCommandSuccess(t, liveCERNHTTP, err, ctx.Err(), string(output), "search --list-facets")

	// Check that we got some facet output
	if !contains(outputStr, "Available facets") {
		t.Error("Expected 'Available facets' in output")
	}

	// Check for common facets
	if !contains(outputStr, "experiment:") {
		t.Error("Expected 'experiment:' facet in output")
	}
}

func TestIntegrationSearchListFacetsWithServer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// #nosec G204
	cmd := exec.CommandContext(ctx, getBinaryPath(), "search", "--list-facets", "--server", "https://opendata.cern.ch")
	output, err := cmd.CombinedOutput()
	assertLiveCommandSuccess(t, liveCERNHTTP, err, ctx.Err(), string(output), "search --list-facets with explicit server")
}

func TestIntegrationUpdateCheck(t *testing.T) {
	output, err := runIntegrationCommand(t, "update", "--check")
	output = assertLiveCommandSuccess(t, liveGitHub, err, nil, output, "update --check")
	if !strings.Contains(output, "Current version:") {
		t.Error("Expected 'Current version:' in output")
	}
	if !strings.Contains(output, "Checking for updates...") {
		t.Error("Expected 'Checking for updates...' in output")
	}
}

func TestIntegrationSearchSizeAll(t *testing.T) {
	assertCommandSuccess(t, "search", "--query-pattern", "recid:"+testRecID, "--size", "-1")
}

func TestIntegrationSearchSizeLimit(t *testing.T) {
	output := assertCommandSuccess(t, "search", "--query-pattern", "Higgs", "--size", "2")
	lines := strings.Split(strings.TrimSpace(output), "\n")
	count := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip empty lines, summary messages, and message markers
		if trimmed == "" || strings.HasPrefix(trimmed, "==>") || strings.HasPrefix(trimmed, "Showing") || strings.HasPrefix(trimmed, "Total:") {
			continue
		}
		count++
	}
	if count != 2 {
		t.Errorf("Expected 2 search results, got %d. Output:\n%s", count, output)
	}
}

func TestIntegrationGetFileLocationsAvailabilityOnline(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-file-locations", "--recid", testUnavailableRecID, "--file-availability", "online")
	output, err := cmd.CombinedOutput()
	outputStr := assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "get-file-locations for online files")
	lines := strings.Split(strings.TrimSpace(outputStr), "\n")
	// Should match exactly 1 line (record 8886 has 1 online file)
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "==>") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Expected exactly 1 online file, got %d. Output:\n%s", count, outputStr)
	}
}

func TestIntegrationGetFileLocationsAvailabilityAll(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-file-locations", "--recid", testUnavailableRecID, "--file-availability", "all")
	output, err := cmd.CombinedOutput()
	outputStr := assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "get-file-locations for all files")
	lines := strings.Split(strings.TrimSpace(outputStr), "\n")
	// Record 8886 currently exposes exactly 5,089 files across online and tape storage.
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "==>") {
			count++
		}
	}
	if count != 5089 {
		t.Errorf("Expected exactly 5089 files, got %d. Output:\n%s", count, outputStr)
	}
}

func TestIntegrationGetFileLocationsAvailabilityWarning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-file-locations", "--recid", testUnavailableRecID)
	// We expect success (err == nil) but warning in stderr
	output, err := cmd.CombinedOutput()
	outputStr := assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "get-file-locations availability warning")
	// Note: CombinedOutput captures both stdout and stderr
	if !contains(outputStr, "WARNING: Some files in the list are not online") {
		t.Error("Expected warning about offline files not found in output")
	}
}

func TestIntegrationDownloadFilesAvailabilityOnline(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--recid", testUnavailableRecID, "--file-availability", "online", "--dry-run", "--output-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	outputStr := assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "download-files for online files")
	if !contains(outputStr, "Files downloaded: 1 /") {
		t.Error("Expected 'Files downloaded: 1' in summary")
	}
	if !contains(outputStr, "Files skipped (on tape)") {
		t.Error("Expected 'Files skipped (on tape)' in summary")
	}
}

func TestIntegrationDownloadFilesAvailabilityWarning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--recid", testUnavailableRecID, "--dry-run", "--output-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	outputStr := assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "download-files availability warning")
	if !contains(outputStr, "WARNING: Some files are stored on tape and will be skipped") {
		t.Error("Expected warning about skipped files")
	}
	if !contains(outputStr, "record/8886") {
		t.Error("Expected staging guidance link to record 8886")
	}
}

func TestIntegrationGetFileLocationsJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-file-locations", "--recid", testRecID, "--format", "json")
	output, err := cmd.CombinedOutput()
	outputStr := assertLiveCommandSuccess(t, liveCERNHTTP, err, nil, string(output), "get-file-locations JSON")

	var files []map[string]interface{}
	if err := json.Unmarshal([]byte(outputStr), &files); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, outputStr)
	}

	if len(files) == 0 {
		t.Fatal("Expected at least one file in output")
	}

	for _, file := range files {
		if _, ok := file["uri"]; !ok {
			t.Error("File entry missing 'uri' field")
		}
	}

}

func TestIntegrationListDirectoryJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "list-directory", "/eos/opendata/cms/software/HiggsExample20112012", "--format", "json")
	output, err := cmd.CombinedOutput()
	outputStr := assertLiveCommandSuccess(t, liveXRootD, err, nil, string(output), "list-directory JSON")

	var entries []map[string]interface{}
	if err := json.Unmarshal([]byte(outputStr), &entries); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, outputStr)
	}

	if len(entries) == 0 {
		t.Fatal("Expected at least one entry in output")
	}

	for _, entry := range entries {
		if _, ok := entry["name"]; !ok {
			t.Error("Entry missing 'name' field")
		}
		if _, ok := entry["is_dir"]; !ok {
			t.Error("Entry missing 'is_dir' field")
		}
	}

}
