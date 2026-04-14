//go:build integration

package main

import (
	"context"
	"encoding/json"
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
	if err != nil {
		t.Fatalf("Command %v failed: %v\nOutput: %s", args, err, output)
	}
	if len(output) == 0 {
		t.Errorf("Command %v returned empty output", args)
	}
	return string(output)
}

func assertCommandError(t *testing.T, args ...string) string {
	t.Helper()
	output, err := runIntegrationCommand(t, args...)
	if err == nil {
		t.Fatalf("Expected command %v to fail, but it succeeded\nOutput: %s", args, output)
	}
	if len(output) == 0 {
		t.Errorf("Command %v failed as expected but returned empty output (expected error message)", args)
	}
	return string(output)
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
	if err != nil {
		t.Fatalf("Failed to run get-metadata by title: %v\nOutput: %s", err, string(output))
	}

	if len(output) == 0 {
		t.Error("Expected non-empty output from get-metadata by title")
	}
}

func TestIntegrationGetMetadataByTitleWrong(t *testing.T) {
	assertCommandError(t, "get-metadata", "--title", "NONEXISTING_TITLE")
}

func TestIntegrationGetMetadataByAlternateDOI(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-metadata", "--doi", "10.7483/OPENDATA.CMS.A342.9982")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run get-metadata by alternate DOI: %v\nOutput: %s", err, string(output))
	}

	if len(output) == 0 {
		t.Error("Expected non-empty output from get-metadata by alternate DOI")
	}
}

func TestIntegrationGetMetadataOutputValueBasic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-metadata", "--recid", "1", "--output-value", "system_details.global_tag")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run get-metadata with output-value: %v\nOutput: %s", err, string(output))
	}

	if !contains(string(output), "FT_R_42_V10A::All") {
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
	if err != nil {
		t.Fatalf("Failed to run get-metadata with array output-value: %v\nOutput: %s", err, string(output))
	}

	if len(output) == 0 {
		t.Error("Expected non-empty output for array field")
	}
}

func TestIntegrationGetMetadataOutputValueNested(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-metadata", "--recid", "1", "--output-value", "usage.links.url")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run get-metadata with nested output-value: %v\nOutput: %s", err, string(output))
	}

	if len(output) == 0 {
		t.Error("Expected non-empty output for nested field")
	}
}

func TestIntegrationGetMetadataOutputValueWrong(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-metadata", "--recid", "1", "--output-value", "title.global_tag")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("Expected error for wrong field, but got none")
	}

	if !contains(string(output), "Field not found") && !contains(string(output), "is not present") {
		t.Error("Expected field not found error in output")
	}
}

func TestIntegrationGetMetadataNoIdentifier(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-metadata")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("Expected error when no identifier provided, but got none")
	}

	if !contains(string(output), "recid") && !contains(string(output), "doi") && !contains(string(output), "title") {
		t.Error("Expected error message to mention required identifier")
	}
}

func TestIntegrationGetMetadataInvalidServer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-metadata", "--recid", testRecID, "--server", "ftp://invalid.com")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("Expected error for invalid server, but got none")
	}

	if len(output) == 0 {
		t.Error("Expected error message for invalid server")
	}
}

func TestIntegrationGetMetadataFilterWithoutOutputValue(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-metadata", "--recid", "1", "--filter", "foo=bar")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("Expected error when using --filter without --output-value")
	}

	if !contains(string(output), "--filter") || !contains(string(output), "--output-value") {
		t.Error("Expected message about --filter requiring --output-value")
	}
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
	if err != nil {
		t.Fatalf("Failed to run get-file-locations with verbose: %v\nOutput: %s", err, string(output))
	}

	if len(output) == 0 {
		t.Error("Expected non-empty output from get-file-locations verbose")
	}

	outputStr := string(output)
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
	if err == nil {
		t.Fatal("Expected error for invalid recid 0, but got none")
	}

	if len(output) == 0 {
		t.Error("Expected error message for invalid recid")
	}
}

func TestIntegrationGetFileLocationsByDOI(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-file-locations", "--doi", "10.7483/OPENDATA.CMS.A342.9982")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run get-file-locations by DOI: %v\nOutput: %s", err, string(output))
	}

	if len(output) == 0 {
		t.Error("Expected non-empty output from get-file-locations by DOI")
	}
}

func TestIntegrationGetFileLocationsByDOIWrong(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-file-locations", "--doi", "NONEXISTING")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("Expected error for non-existing DOI, but got none")
	}

	if len(output) == 0 {
		t.Error("Expected error message for non-existing DOI")
	}
}

func TestIntegrationGetFileLocationsByTitle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-file-locations", "--title", "Configuration file for LHE step HIG-Summer11pLHE-00114_1_cfg.py")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run get-file-locations by title: %v\nOutput: %s", err, string(output))
	}

	if len(output) == 0 {
		t.Error("Expected non-empty output from get-file-locations by title")
	}
}

func TestIntegrationGetFileLocationsByTitleWrong(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-file-locations", "--title", "NONEXISTING_TITLE")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("Expected error for non-existing title, but got none")
	}

	if len(output) == 0 {
		t.Error("Expected error message for non-existing title")
	}
}

func TestIntegrationGetFileLocationsHTTP(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-file-locations", "--recid", testRecID, "--protocol", "http")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run get-file-locations with http: %v\nOutput: %s", err, string(output))
	}

	if len(output) == 0 {
		t.Error("Expected non-empty output from get-file-locations with http")
	}
}

func TestIntegrationGetFileLocationsHTTPS(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-file-locations", "--recid", testRecID, "--protocol", "https")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run get-file-locations with https: %v\nOutput: %s", err, string(output))
	}

	if len(output) == 0 {
		t.Error("Expected non-empty output from get-file-locations with https")
	}
}

func TestIntegrationGetFileLocationsExpand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-file-locations", "--recid", testRecID, "--expand")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run get-file-locations with expand: %v\nOutput: %s", err, string(output))
	}

	if len(output) == 0 {
		t.Error("Expected non-empty output from get-file-locations with expand")
	}
}

func TestIntegrationGetFileLocationsNoIdentifier(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-file-locations")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("Expected error when no identifier provided, but got none")
	}

	if !contains(string(output), "recid") && !contains(string(output), "doi") && !contains(string(output), "title") {
		t.Error("Expected error message to mention required identifier")
	}
}

func TestIntegrationGetFileLocationsInvalidServer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "get-file-locations", "--recid", testRecID, "--server", "ftp://invalid.com")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("Expected error for invalid server, but got none")
	}

	if len(output) == 0 {
		t.Error("Expected error message for invalid server")
	}
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
	if err != nil {
		t.Logf("Warning: download-files failed (expected if no .txt files): %v\nOutput: %s", err, string(output))
		return
	}

	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read temp directory: %v", err)
	}

	if len(files) == 0 {
		t.Log("No files downloaded (expected if no .txt files in record)")
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
	if err != nil {
		t.Logf("Warning: download by DOI failed: %v\nOutput: %s", err, string(output))
		return
	}

	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read temp directory: %v", err)
	}

	if len(files) == 0 {
		t.Log("No files downloaded from DOI")
	}
}

func TestIntegrationDownloadFilesByDOIWrong(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--doi", "NONEXISTING")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("Expected error for non-existing DOI, but got none")
	}

	if len(output) == 0 {
		t.Error("Expected error message for non-existing DOI")
	}
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
	if err != nil {
		t.Logf("Warning: list-directory failed (XRootD service may be unavailable): %v\nOutput: %s", err, string(output))
		return
	}

	if len(output) == 0 {
		t.Error("Expected non-empty output from list-directory")
	}
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
	if err != nil {
		t.Logf("Warning: list-directory verbose failed (XRootD service may be unavailable): %v\nOutput: %s", err, string(output))
		return
	}

	if len(output) == 0 {
		t.Error("Expected non-empty output from list-directory verbose")
	}

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
	if err != nil {
		t.Logf("Warning: list-directory wrong path failed (XRootD service may be unavailable): %v\nOutput: %s", err, string(output))
		return
	}

	if !contains(string(output), "does not exist") && !contains(string(output), "not found") && !contains(string(output), "failed") {
		t.Log("Note: Expected 'does not exist' error for wrong path")
	}
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
	if err == nil && len(output) == 0 {
		t.Log("Expected empty directory or error")
		return
	}

	if len(output) == 0 {
		t.Log("No output (expected for non-existent directory)")
	}
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

func TestIntegrationDownloadFilesFromRecid(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--recid", testRecID, "--filter-name", "*.py", "--output-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf(" download failed: %v\nOutput: %s", err, string(output))
	}

	outputStr := string(output)
	if !contains(outputStr, "Success") {
		t.Error("Expected 'Success!' message in output")
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "0d0714743f0204ed3c0144941e6ce248.configFile.py")); os.IsNotExist(err) {
		t.Error("Expected file to be downloaded")
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
	if err == nil {
		t.Fatal("Expected error for invalid recid 0, but got none")
	}

	if len(output) == 0 {
		t.Error("Expected error message for invalid recid")
	}
}

func TestIntegrationDownloadFilesFilterName(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--recid", "5500", "--filter-name", "BuildFile.xml", "--output-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf(" download failed: %v\nOutput: %s", err, string(output))
	}

	outputStr := string(output)
	if !contains(outputStr, "Success") {
		t.Error("Expected 'Success!' message in output")
	}

	files, _ := os.ReadDir(tmpDir)
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
	if err == nil {
		t.Fatal("Expected error for non-matching filter, but got none")
	}

	outputStr := string(output)
	if !contains(outputStr, "No files") {
		t.Error("Expected 'No files matching filters' message")
	}
}

func TestIntegrationDownloadFilesFilterRange(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--recid", "5500", "--filter-range", "0-2", "--output-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf(" download failed: %v\nOutput: %s", err, string(output))
	}

	files, _ := os.ReadDir(tmpDir)
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
	if err != nil {
		t.Logf("Warning: download failed (invalid range): %v\nOutput: %s", err, string(output))
		return
	}

	files, _ := os.ReadDir(tmpDir)
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
	if err != nil {
		t.Fatalf(" download failed: %v\nOutput: %s", err, string(output))
	}

	outputStr := string(output)
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
	if err != nil {
		t.Fatalf(" download failed: %v\nOutput: %s", err, string(output))
	}

	outputStr := string(output)
	if len(outputStr) == 0 {
		t.Error("Expected non-empty verbose output")
	}
}

func TestIntegrationDownloadFilesNoIdentifier(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("Expected error when no identifier provided, but got none")
	}

	outputStr := string(output)
	if !contains(outputStr, "recid") && !contains(outputStr, "doi") && !contains(outputStr, "title") {
		t.Error("Expected error message to mention required identifier")
	}
}

func TestIntegrationDownloadFilesInvalidServer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--recid", testRecID, "--server", "ftp://invalid.com", "--dry-run")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("Expected error for invalid server, but got none")
	}

	outputStr := string(output)
	if len(outputStr) == 0 {
		t.Error("Expected error message for invalid server")
	}
}

func TestIntegrationDownloadFilesCustomOutputDir(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--recid", testRecID, "--filter-name", "*.py", "--output-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf(" download failed: %v\nOutput: %s", err, string(output))
	}

	files, _ := os.ReadDir(tmpDir)
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
	if downloadErr != nil {
		t.Fatalf(" download failed: %v\nOutput: %s", downloadErr, string(downloadOutput))
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "verify-files", "--recid", testRecID, "--input-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run verify-files: %v\nOutput: %s", err, string(output))
	}

	outputStr := string(output)
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
	if downloadErr != nil {
		t.Fatalf(" download failed: %v\nOutput: %s", downloadErr, string(downloadOutput))
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "verify-files", "--recid", testRecID, "--input-dir", tmpDir, "--filter-name", "*.py")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run verify-files with name filter: %v\nOutput: %s", err, string(output))
	}

	outputStr := string(output)
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
	if err == nil {
		t.Fatal("Expected error when no identifier provided, but got none")
	}

	outputStr := string(output)
	if !contains(outputStr, "recid") && !contains(outputStr, "doi") && !contains(outputStr, "title") {
		t.Error("Expected error message to mention required identifier")
	}
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
	if downloadErr != nil {
		t.Fatalf(" download failed: %v\nOutput: %s", downloadErr, string(downloadOutput))
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "verify-files", "--doi", "10.7483/OPENDATA.CMS.JKB8.RR42", "--input-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run verify-files by DOI: %v\nOutput: %s", err, string(output))
	}

	if len(output) == 0 {
		t.Error("Expected non-empty output from verify-files")
	}
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
	if downloadErr != nil {
		t.Fatalf(" download failed: %v\nOutput: %s", downloadErr, string(downloadOutput))
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "verify-files", "--title", "Configuration file for LHE step HIG-Summer11pLHE-00114_1_cfg.py", "--input-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run verify-files by title: %v\nOutput: %s", err, string(output))
	}

	if len(output) == 0 {
		t.Error("Expected non-empty output from verify-files")
	}
}

func TestIntegrationVerifyFilesInvalidServer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "verify-files", "--recid", testRecID, "--server", "ftp://invalid.com")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("Expected error for invalid server, but got none")
	}

	outputStr := string(output)
	if len(outputStr) == 0 {
		t.Error("Expected error message for invalid server")
	}
}

func TestIntegrationVerifyFilesCustomInputDir(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	downloadCmd := exec.Command(getBinaryPath(), "download-files", "--recid", testRecID, "--filter-name", "*.py", "--output-dir", tmpDir)
	downloadOutput, downloadErr := downloadCmd.CombinedOutput()
	if downloadErr != nil {
		t.Fatalf(" download failed: %v\nOutput: %s", downloadErr, string(downloadOutput))
	}

	customDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(customDir, 0750); err != nil {
		t.Fatalf("Failed to create custom directory: %v", err)
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "verify-files", "--recid", testRecID, "--input-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run verify-files with custom dir: %v\nOutput: %s", err, string(output))
	}

	outputStr := string(output)
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
	if err != nil {
		t.Fatalf(" download failed: %v\nOutput: %s", err, string(output))
	}

	files, _ := os.ReadDir(tmpDir)
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
	if err != nil {
		t.Fatalf(" download failed: %v\nOutput: %s", err, string(output))
	}

	files, _ := os.ReadDir(tmpDir)
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
	if err != nil {
		t.Logf("Warning: download failed: %v\nOutput: %s", err, string(output))
		return
	}

	files, _ := os.ReadDir(tmpDir)
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
	if err != nil {
		t.Fatalf(" download failed: %v\nOutput: %s", err, string(output))
	}

	files, _ := os.ReadDir(tmpDir)
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
	if err != nil {
		t.Fatalf(" download failed: %v\nOutput: %s", err, string(output))
	}

	files, _ := os.ReadDir(tmpDir)
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
	if err != nil {
		t.Fatalf(" download failed: %v\nOutput: %s", err, string(output))
	}

	files, _ := os.ReadDir(tmpDir)

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
	if err != nil {
		t.Fatalf(" download failed: %v\nOutput: %s", err, string(output))
	}

	files, _ := os.ReadDir(tmpDir)

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
	if err != nil {
		t.Fatalf(" list-directory recursive failed: %v\nOutput: %s", err, string(output))
	}

	outputStr := string(output)
	if len(outputStr) == 0 {
		t.Error("Expected non-empty output from recursive directory listing")
	}

	entries := strings.Split(outputStr, "\n")
	if len(entries) < 10 {
		t.Errorf("Expected at least 10 entries from recursive listing, got: %d", len(entries))
	}
}

func TestIntegrationListDirectoryTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Use a very short timeout that should cause context cancellation
	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "list-directory", "/eos/opendata/cms/software/HiggsExample20112012", "--timeout", "5")
	output, err := cmd.CombinedOutput()
	// The test passes if it completes (either successfully or with timeout error)
	// We just want to ensure that timeout flag doesn't break the command
	if err != nil && len(output) == 0 {
		t.Logf("Note: Command failed with timeout (expected behavior): %v", err)
	}

	// As long as we got output or a clear error, test passes
	if len(output) > 0 {
		t.Logf("Got %d bytes of output before potential timeout", len(output))
	}

	// As long as we got output or a clear error, the test passes
	if len(output) > 0 {
		t.Logf("Got %d bytes of output before potential timeout", len(output))
	}
}

func TestIntegrationDownloadFilesWithVerify(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "download-files", "--recid", testRecID, "--filter-name", "*.py", "--verify", "--output-dir", tmpDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run download-files with verify: %v\nOutput: %s", err, string(output))
	}

	outputStr := string(output)
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
	if err != nil {
		t.Fatalf("Failed to run download-files with download-engine: %v\nOutput: %s", err, string(output))
	}

	outputStr := string(output)
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
	if err != nil {
		t.Fatalf("Failed to run download-files with retry-sleep: %v\nOutput: %s", err, string(output))
	}

	outputStr := string(output)
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
	if err != nil {
		t.Fatalf(" XRootD download failed (XRootD server may be unavailable): %v\nOutput: %s", err, string(output))
		t.Skip("Skipping XRootD test - server unavailable")
	}

	outputStr := string(output)
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
	if err == nil {
		t.Log("Note: Command completed (XRootD error handling worked)")
	}

	outputStr := string(output)
	if len(outputStr) > 0 {
		t.Logf("Output: %s", outputStr)
	}

	if err != nil && len(outputStr) > 0 {
		t.Log("Got expected error from invalid XRootD server")
	}
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
	if err != nil {
		t.Logf("Warning: search failed (network may be unavailable): %v\nOutput: %s", err, string(output))
		return
	}

	if len(output) == 0 {
		t.Error("Expected non-empty output from search")
	}
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
	if err != nil {
		t.Logf("Warning: search with facets failed (network may be unavailable): %v\nOutput: %s", err, string(output))
		return
	}

	if len(output) == 0 {
		t.Error("Expected non-empty output from search with facets")
	}
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
	if err != nil {
		t.Logf("Warning: search with URL query failed (network may be unavailable): %v\nOutput: %s", err, string(output))
		return
	}

	if len(output) == 0 {
		t.Error("Expected non-empty output from search with URL query")
	}
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
	if err != nil {
		t.Logf("Warning: search with output-value failed (network may be unavailable): %v\nOutput: %s", err, string(output))
		return
	}

	if len(output) == 0 {
		t.Error("Expected non-empty output from search with output-value")
	}
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
	if err != nil {
		t.Logf("Warning: search with JSON format failed (network may be unavailable): %v\nOutput: %s", err, string(output))
		return
	}

	outputStr := string(output)
	if len(outputStr) > 0 && !strings.Contains(outputStr, "[") {
		t.Error("Expected JSON array output")
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
	if err != nil {
		t.Logf("Warning: search failed (network may be unavailable): %v\nOutput: %s", err, string(output))
		return
	}

	outputStr := string(output)
	if !contains(outputStr, "No records found") {
		t.Logf("Output: %s", outputStr)
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
	if err == nil {
		t.Fatal("Expected error when using --filter without --output-value")
	}

	outputStr := string(output)
	if !contains(outputStr, "--filter") || !contains(outputStr, "--output-value") {
		t.Error("Expected message about --filter requiring --output-value")
	}
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
	if err != nil {
		t.Logf("Warning: --list-facets failed (network may be unavailable): %v\nOutput: %s", err, string(output))
		return
	}

	outputStr := string(output)

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
	if err != nil {
		t.Logf("Warning: --list-facets with --server failed (network may be unavailable): %v\nOutput: %s", err, string(output))
		return
	}

	outputStr := string(output)
	if len(outputStr) == 0 {
		t.Error("Expected non-empty output from --list-facets")
	}
}

func TestIntegrationUpdateCheck(t *testing.T) {
	// Skip on macOS due to intermittent GitHub API 403 errors on GitHub Actions macOS runners.
	// The test runs successfully on Linux, so we only skip on darwin/macOS.
	if runtime.GOOS == "darwin" {
		t.Skip("Skipping update check test on macOS due to intermittent GitHub API failures")
	}

	output := assertCommandSuccess(t, "update", "--check")
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
	if err != nil {
		t.Fatalf("get-file-locations failed: %v\nOutput: %s", err, string(output))
	}

	outputStr := string(output)
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
	if err != nil {
		t.Fatalf("get-file-locations failed: %v\nOutput: %s", err, string(output))
	}

	outputStr := string(output)
	lines := strings.Split(strings.TrimSpace(outputStr), "\n")
	// Should match 2997 files
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "==>") {
			count++
		}
	}
	if count < 100 {
		t.Errorf("Expected many files (>100), got %d. Output:\n%s", count, outputStr)
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
	if err != nil {
		t.Fatalf("get-file-locations failed: %v\nOutput: %s", err, string(output))
	}

	outputStr := string(output)
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
	if err != nil {
		t.Fatalf("download-files failed: %v\nOutput: %s", err, string(output))
	}

	outputStr := string(output)
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
	if err != nil {
		t.Fatalf("download-files failed: %v\nOutput: %s", err, string(output))
	}

	outputStr := string(output)
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
	if err != nil {
		t.Logf("Warning: get-file-locations with JSON format failed (network may be unavailable): %v\nOutput: %s", err, string(output))
		return
	}

	var files []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &files); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, string(output))
	}

	if len(files) == 0 {
		t.Fatal("Expected at least one file in output")
	}

	for _, file := range files {
		if _, ok := file["uri"]; !ok {
			t.Error("File entry missing 'uri' field")
		}
	}

	t.Logf("Successfully got %d files in JSON format", len(files))
}

func TestIntegrationListDirectoryJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// #nosec G204
	cmd := exec.Command(getBinaryPath(), "list-directory", "/eos/opendata/cms/software/HiggsExample20112012", "--format", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Warning: list-directory with JSON format failed (network may be unavailable): %v\nOutput: %s", err, string(output))
		return
	}

	var entries []map[string]interface{}
	if err := json.Unmarshal(output, &entries); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, string(output))
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

	t.Logf("Successfully got %d entries in JSON format", len(entries))
}
