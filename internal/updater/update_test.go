package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		latest   string
		expected int
	}{
		{
			name:     "dev vs release",
			current:  "dev",
			latest:   "v1.0.0",
			expected: -1,
		},
		{
			name:     "release vs dev",
			current:  "v1.0.0",
			latest:   "dev",
			expected: 1,
		},
		{
			name:     "equal versions",
			current:  "v1.0.0",
			latest:   "v1.0.0",
			expected: 0,
		},
		{
			name:     "older vs newer",
			current:  "v1.0.0",
			latest:   "v1.1.0",
			expected: -1,
		},
		{
			name:     "newer vs older",
			current:  "v1.1.0",
			latest:   "v1.0.0",
			expected: 1,
		},
		{
			name:     "major version difference",
			current:  "v1.0.0",
			latest:   "v2.0.0",
			expected: -1,
		},
		{
			name:     "patch version difference",
			current:  "v1.0.0",
			latest:   "v1.0.1",
			expected: -1,
		},
		{
			name:     "without v prefix",
			current:  "1.0.0",
			latest:   "1.1.0",
			expected: -1,
		},
		{
			name:     "with pre-release suffix",
			current:  "1.0.0-rc1",
			latest:   "1.0.0",
			expected: 0,
		},
		{
			name:     "dev prefix",
			current:  "dev-abc123",
			latest:   "v1.0.0",
			expected: -1,
		},
		{
			name:     "multi-part version",
			current:  "v1.2.3.4",
			latest:   "v1.2.3.5",
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CompareVersions(tt.current, tt.latest)
			if result != tt.expected {
				t.Errorf("CompareVersions(%q, %q) = %d, expected %d", tt.current, tt.latest, result, tt.expected)
			}
		})
	}
}

func TestFetchChecksums(t *testing.T) {
	firstChecksum := strings.Repeat("a", 64)
	secondChecksum := strings.Repeat("b", 64)
	testData := fmt.Sprintf("%s  file1.txt\n%s  file2.bin\n", firstChecksum, secondChecksum)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(testData))
	}))
	t.Cleanup(server.Close)

	checksums, err := FetchChecksums(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse checksums: %v", err)
	}

	expectedChecksums := map[string]string{
		"file1.txt": firstChecksum,
		"file2.bin": secondChecksum,
	}

	if len(checksums) != len(expectedChecksums) {
		t.Errorf("Expected %d checksums, got %d", len(expectedChecksums), len(checksums))
	}

	for file, expected := range expectedChecksums {
		if actual, ok := checksums[file]; !ok || actual != expected {
			t.Errorf("Checksum for %s: expected %s, got %s", file, expected, actual)
		}
	}
}

func TestVerifyChecksum(t *testing.T) {
	data := []byte("test data")
	hash := sha256.Sum256(data)
	expectedChecksum := hex.EncodeToString(hash[:])

	tests := []struct {
		name             string
		data             []byte
		expectedChecksum string
		wantErr          bool
	}{
		{
			name:             "valid checksum",
			data:             data,
			expectedChecksum: expectedChecksum,
			wantErr:          false,
		},
		{
			name:             "invalid checksum",
			data:             data,
			expectedChecksum: "0000000000000000000000000000000000000000000000000000000000000000",
			wantErr:          true,
		},
		{
			name:             "uppercase checksum",
			data:             data,
			expectedChecksum: strings.ToUpper(expectedChecksum),
			wantErr:          false,
		},
		{
			name:             "short checksum",
			data:             data,
			expectedChecksum: "abcd",
			wantErr:          true,
		},
		{
			name:             "non-hex checksum",
			data:             data,
			expectedChecksum: strings.Repeat("z", 64),
			wantErr:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyChecksum(tt.data, tt.expectedChecksum)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyChecksum() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDownloadVerifiedBinaryChecksumPolicy(t *testing.T) {
	assetName := fmt.Sprintf("cernopendata-client-%s-%s", runtime.GOOS, runtime.GOARCH)
	binary := []byte("verified release binary")
	hash := sha256.Sum256(binary)
	validChecksum := hex.EncodeToString(hash[:])
	otherChecksum := strings.Repeat("b", 64)

	tests := []struct {
		name                 string
		checksumStatus       int
		checksumManifest     string
		binaryStatus         int
		wantErrContains      string
		wantBinaryRequest    bool
		wantDownloadedBinary bool
	}{
		{
			name:                 "valid checksum",
			checksumStatus:       http.StatusOK,
			checksumManifest:     fmt.Sprintf("%s  another-asset\n%s  %s\n", otherChecksum, validChecksum, assetName),
			binaryStatus:         http.StatusOK,
			wantBinaryRequest:    true,
			wantDownloadedBinary: true,
		},
		{
			name:              "checksum unavailable",
			checksumStatus:    http.StatusServiceUnavailable,
			binaryStatus:      http.StatusOK,
			wantErrContains:   "checksums download failed with status 503",
			wantBinaryRequest: false,
		},
		{
			name:              "empty checksum manifest",
			checksumStatus:    http.StatusOK,
			checksumManifest:  "\n",
			binaryStatus:      http.StatusOK,
			wantErrContains:   "checksums file is empty",
			wantBinaryRequest: false,
		},
		{
			name:              "malformed checksum line",
			checksumStatus:    http.StatusOK,
			checksumManifest:  "not-a-checksum-entry\n",
			binaryStatus:      http.StatusOK,
			wantErrContains:   "malformed checksum entry",
			wantBinaryRequest: false,
		},
		{
			name:              "short checksum",
			checksumStatus:    http.StatusOK,
			checksumManifest:  fmt.Sprintf("abcd  %s\n", assetName),
			binaryStatus:      http.StatusOK,
			wantErrContains:   "exactly 64 hexadecimal characters",
			wantBinaryRequest: false,
		},
		{
			name:              "non-hex checksum",
			checksumStatus:    http.StatusOK,
			checksumManifest:  fmt.Sprintf("%s  %s\n", strings.Repeat("z", 64), assetName),
			binaryStatus:      http.StatusOK,
			wantErrContains:   "only hexadecimal characters",
			wantBinaryRequest: false,
		},
		{
			name:              "absent asset entry",
			checksumStatus:    http.StatusOK,
			checksumManifest:  fmt.Sprintf("%s  another-asset\n", otherChecksum),
			binaryStatus:      http.StatusOK,
			wantErrContains:   "no checksum found",
			wantBinaryRequest: false,
		},
		{
			name:              "duplicate asset entries",
			checksumStatus:    http.StatusOK,
			checksumManifest:  fmt.Sprintf("%s  %s\n%s  %s\n", validChecksum, assetName, validChecksum, assetName),
			binaryStatus:      http.StatusOK,
			wantErrContains:   "duplicate checksum entry",
			wantBinaryRequest: false,
		},
		{
			name:              "mismatched checksum",
			checksumStatus:    http.StatusOK,
			checksumManifest:  fmt.Sprintf("%s  %s\n", strings.Repeat("0", 64), assetName),
			binaryStatus:      http.StatusOK,
			wantErrContains:   "checksum mismatch",
			wantBinaryRequest: true,
		},
		{
			name:              "binary unavailable",
			checksumStatus:    http.StatusOK,
			checksumManifest:  fmt.Sprintf("%s  %s\n", validChecksum, assetName),
			binaryStatus:      http.StatusBadGateway,
			wantErrContains:   "download failed with status 502",
			wantBinaryRequest: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			binaryRequested := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/checksums.txt":
					w.WriteHeader(tt.checksumStatus)
					_, _ = w.Write([]byte(tt.checksumManifest))
				case "/" + assetName:
					binaryRequested = true
					w.WriteHeader(tt.binaryStatus)
					_, _ = w.Write(binary)
				default:
					http.NotFound(w, r)
				}
			}))
			t.Cleanup(server.Close)

			got, err := DownloadVerifiedBinary(server.URL+"/"+assetName, server.URL+"/checksums.txt", assetName, nil)
			if tt.wantErrContains == "" {
				if err != nil {
					t.Fatalf("DownloadVerifiedBinary() unexpected error: %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tt.wantErrContains) {
				t.Fatalf("DownloadVerifiedBinary() error = %v, want error containing %q", err, tt.wantErrContains)
			}
			if binaryRequested != tt.wantBinaryRequest {
				t.Errorf("binary requested = %v, want %v", binaryRequested, tt.wantBinaryRequest)
			}
			if tt.wantDownloadedBinary && string(got) != string(binary) {
				t.Errorf("DownloadVerifiedBinary() = %q, want %q", got, binary)
			}
		})
	}
}

func TestIsHomebrewInstall(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "Apple Silicon Homebrew",
			path:     "/opt/homebrew/bin/cernopendata-client",
			expected: true,
		},
		{
			name:     "Intel Mac Homebrew",
			path:     "/usr/local/Cellar/cernopendata-client-go/1.0.0/bin/cernopendata-client",
			expected: true,
		},
		{
			name:     "Linux Homebrew",
			path:     "/home/linuxbrew/.linuxbrew/bin/cernopendata-client",
			expected: true,
		},
		{
			name:     "User-local Linux Homebrew",
			path:     "/.linuxbrew/bin/cernopendata-client",
			expected: true,
		},
		{
			name:     "usr/local/bin (not Homebrew)",
			path:     "/usr/local/bin/cernopendata-client",
			expected: false,
		},
		{
			name:     "home/bin (user directory)",
			path:     "/home/user/bin/cernopendata-client",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isHomebrewPath(tt.path)
			if result != tt.expected {
				t.Errorf("isHomebrewPath(%q) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestGetAssetForCurrentPlatform(t *testing.T) {
	tests := []struct {
		name        string
		release     ReleaseInfo
		wantBinary  string
		wantCheck   string
		wantErr     bool
		errContains string
	}{
		{
			name: "matching binary",
			release: ReleaseInfo{
				Assets: []ReleaseAsset{
					{Name: fmt.Sprintf("cernopendata-client-%s-%s", runtime.GOOS, runtime.GOARCH), BrowserDownloadURL: "https://example.com/binary"},
					{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums.txt"},
				},
			},
			wantBinary: "https://example.com/binary",
			wantCheck:  "https://example.com/checksums.txt",
			wantErr:    false,
		},
		{
			name: "no matching binary",
			release: ReleaseInfo{
				Assets: []ReleaseAsset{
					{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums.txt"},
				},
			},
			wantBinary:  "",
			wantCheck:   "https://example.com/checksums.txt",
			wantErr:     true,
			errContains: "no binary found",
		},
		{
			name: "missing checksums asset",
			release: ReleaseInfo{
				Assets: []ReleaseAsset{
					{Name: fmt.Sprintf("cernopendata-client-%s-%s", runtime.GOOS, runtime.GOARCH), BrowserDownloadURL: "https://example.com/binary"},
				},
			},
			wantErr:     true,
			errContains: "checksums.txt is required",
		},
		{
			name: "duplicate checksums assets",
			release: ReleaseInfo{
				Assets: []ReleaseAsset{
					{Name: fmt.Sprintf("cernopendata-client-%s-%s", runtime.GOOS, runtime.GOARCH), BrowserDownloadURL: "https://example.com/binary"},
					{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums-1.txt"},
					{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums-2.txt"},
				},
			},
			wantErr:     true,
			errContains: "exactly one checksums.txt",
		},
		{
			name: "duplicate matching binaries",
			release: ReleaseInfo{
				Assets: []ReleaseAsset{
					{Name: fmt.Sprintf("cernopendata-client-%s-%s", runtime.GOOS, runtime.GOARCH), BrowserDownloadURL: "https://example.com/binary-1"},
					{Name: fmt.Sprintf("cernopendata-client-%s-%s", runtime.GOOS, runtime.GOARCH), BrowserDownloadURL: "https://example.com/binary-2"},
					{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums.txt"},
				},
			},
			wantErr:     true,
			errContains: "exactly one binary",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			binaryURL, checksumURL, err := GetAssetForCurrentPlatform(&tt.release)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetAssetForCurrentPlatform() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContains != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Expected error to contain %q, got %q", tt.errContains, err.Error())
				}
			}
			if binaryURL != tt.wantBinary {
				t.Errorf("GetAssetForCurrentPlatform() binaryURL = %v, want %v", binaryURL, tt.wantBinary)
			}
			if !tt.wantErr && checksumURL != tt.wantCheck {
				t.Errorf("GetAssetForCurrentPlatform() checksumURL = %v, want %v", checksumURL, tt.wantCheck)
			}
		})
	}
}

func isHomebrewPath(path string) bool {
	homebrewPaths := []string{
		"/opt/homebrew/",
		"/usr/local/Cellar/",
		"/home/linuxbrew/",
		"/.linuxbrew/",
	}

	for _, prefix := range homebrewPaths {
		if strings.Contains(path, prefix) {
			return true
		}
	}

	return false
}
