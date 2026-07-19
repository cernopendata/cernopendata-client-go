package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallScriptChecksumPolicy(t *testing.T) {
	assetName := fmt.Sprintf("cernopendata-client-%s-%s", runtime.GOOS, runtime.GOARCH)
	binary := []byte("#!/bin/sh\necho fixture-version\n")
	hash := sha256.Sum256(binary)
	validChecksum := hex.EncodeToString(hash[:])

	tests := []struct {
		name            string
		checksumStatus  int
		manifest        string
		restrictedPath  bool
		wantErrContains string
		wantInstall     bool
	}{
		{
			name:           "valid checksum",
			checksumStatus: http.StatusOK,
			manifest:       fmt.Sprintf("%s  %s\n", validChecksum, assetName),
			wantInstall:    true,
		},
		{
			name:            "missing checksum manifest",
			checksumStatus:  http.StatusNotFound,
			wantErrContains: "Failed to download required checksums file",
		},
		{
			name:            "unavailable checksum manifest",
			checksumStatus:  http.StatusServiceUnavailable,
			wantErrContains: "Failed to download required checksums file",
		},
		{
			name:            "empty checksum manifest",
			checksumStatus:  http.StatusOK,
			manifest:        "\n",
			wantErrContains: "Expected exactly one checksum entry",
		},
		{
			name:            "malformed checksum entry",
			checksumStatus:  http.StatusOK,
			manifest:        "not-a-checksum-entry\n",
			wantErrContains: "Malformed checksum entry",
		},
		{
			name:            "short checksum",
			checksumStatus:  http.StatusOK,
			manifest:        fmt.Sprintf("abcd  %s\n", assetName),
			wantErrContains: "Malformed checksum entry",
		},
		{
			name:            "non-hex checksum",
			checksumStatus:  http.StatusOK,
			manifest:        fmt.Sprintf("%s  %s\n", strings.Repeat("z", 64), assetName),
			wantErrContains: "Malformed checksum entry",
		},
		{
			name:            "absent asset entry",
			checksumStatus:  http.StatusOK,
			manifest:        fmt.Sprintf("%s  another-asset\n", validChecksum),
			wantErrContains: "Expected exactly one checksum entry",
		},
		{
			name:            "duplicate asset entries",
			checksumStatus:  http.StatusOK,
			manifest:        fmt.Sprintf("%s  %s\n%s  %s\n", validChecksum, assetName, validChecksum, assetName),
			wantErrContains: "found 2",
		},
		{
			name:            "mismatched checksum",
			checksumStatus:  http.StatusOK,
			manifest:        fmt.Sprintf("%s  %s\n", strings.Repeat("0", 64), assetName),
			wantErrContains: "Checksum verification failed",
		},
		{
			name:            "missing checksum utility",
			checksumStatus:  http.StatusOK,
			manifest:        fmt.Sprintf("%s  %s\n", validChecksum, assetName),
			restrictedPath:  true,
			wantErrContains: "sha256sum or shasum is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/releases/latest"):
					w.Header().Set("Content-Type", "application/json")
					_, _ = fmt.Fprintln(w, `{"tag_name":"v-test"}`)
				case strings.HasSuffix(r.URL.Path, "/checksums.txt"):
					w.WriteHeader(tt.checksumStatus)
					_, _ = w.Write([]byte(tt.manifest))
				case strings.HasSuffix(r.URL.Path, "/"+assetName):
					_, _ = w.Write(binary)
				default:
					http.NotFound(w, r)
				}
			}))
			t.Cleanup(server.Close)

			installDir := t.TempDir()
			homeDir := t.TempDir()
			pathValue := os.Getenv("PATH")
			if tt.restrictedPath {
				pathValue = checksumlessToolPath(t)
			}

			scriptPath := filepath.Join(repoRoot(t), "scripts", "install.sh")
			// #nosec G204 -- scriptPath is derived from this source file, not user input.
			cmd := exec.Command("/bin/bash", scriptPath)
			cmd.Env = append(os.Environ(),
				"GITHUB_API_BASE_URL="+server.URL,
				"GITHUB_RELEASE_BASE_URL="+server.URL,
				"INSTALL_DIR="+installDir,
				"HOME="+homeDir,
				"SHELL=/bin/bash",
				"PATH="+pathValue,
			)
			output, err := cmd.CombinedOutput()

			if tt.wantErrContains == "" {
				if err != nil {
					t.Fatalf("install.sh failed: %v\n%s", err, output)
				}
			} else if err == nil || !strings.Contains(string(output), tt.wantErrContains) {
				t.Fatalf("install.sh error = %v, output = %q, want output containing %q", err, output, tt.wantErrContains)
			}

			_, statErr := os.Stat(filepath.Join(installDir, "cernopendata-client"))
			installed := statErr == nil
			if installed != tt.wantInstall {
				t.Errorf("installed = %v, want %v (stat error: %v)\n%s", installed, tt.wantInstall, statErr, output)
			}
		})
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to locate install script test")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func checksumlessToolPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"awk", "curl", "grep", "mktemp", "rm", "sed", "uname"} {
		target, err := exec.LookPath(name)
		if err != nil {
			t.Fatalf("required test tool %s not found: %v", name, err)
		}
		if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
			t.Fatalf("linking test tool %s: %v", name, err)
		}
	}
	return dir
}
