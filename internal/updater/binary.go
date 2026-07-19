package updater

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func GetAssetForCurrentPlatform(release *ReleaseInfo) (binaryURL, checksumURL string, err error) {
	osName := runtime.GOOS
	archName := runtime.GOARCH

	assetName := fmt.Sprintf("cernopendata-client-%s-%s", osName, archName)
	binaryCount := 0
	checksumCount := 0

	for _, asset := range release.Assets {
		if asset.Name == assetName {
			binaryCount++
			binaryURL = asset.BrowserDownloadURL
		}
		if asset.Name == "checksums.txt" {
			checksumCount++
			checksumURL = asset.BrowserDownloadURL
		}
	}

	if binaryCount == 0 || binaryURL == "" {
		return "", "", fmt.Errorf("no binary found for %s/%s", osName, archName)
	}
	if binaryCount != 1 {
		return "", "", fmt.Errorf("expected exactly one binary for %s/%s, found %d", osName, archName, binaryCount)
	}
	if checksumCount == 0 || checksumURL == "" {
		return "", "", fmt.Errorf("checksums.txt is required for binary installation")
	}
	if checksumCount != 1 {
		return "", "", fmt.Errorf("expected exactly one checksums.txt asset, found %d", checksumCount)
	}

	return binaryURL, checksumURL, nil
}

func DownloadBinary(url string, progress func(downloaded, total int64)) ([]byte, error) {
	resp, err := http.Get(url) // #nosec G107
	if err != nil {
		return nil, fmt.Errorf("failed to download binary: %w", err)
	}
	defer func() {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	var data []byte
	if progress != nil && resp.ContentLength > 0 {
		data = make([]byte, 0, resp.ContentLength)
		buf := make([]byte, 32*1024)
		var downloaded int64
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				data = append(data, buf[:n]...)
				downloaded += int64(n)
				progress(downloaded, resp.ContentLength)
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("failed to read response: %w", err)
			}
		}
	} else {
		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read binary: %w", err)
		}
	}

	return data, nil
}

func FetchChecksums(url string) (map[string]string, error) {
	resp, err := http.Get(url) // #nosec G107
	if err != nil {
		return nil, fmt.Errorf("failed to download checksums: %w", err)
	}
	defer func() {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("checksums download failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read checksums: %w", err)
	}

	return parseChecksums(body)
}

// FetchChecksum fetches and validates a release checksum manifest, returning
// the single checksum for assetName. Duplicate or missing entries fail closed.
func FetchChecksum(url, assetName string) (string, error) {
	checksums, err := FetchChecksums(url)
	if err != nil {
		return "", err
	}

	checksum, ok := checksums[assetName]
	if !ok {
		return "", fmt.Errorf("no checksum found for %s", assetName)
	}

	return checksum, nil
}

func parseChecksums(data []byte) (map[string]string, error) {
	checksums := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) != 2 {
			return nil, fmt.Errorf("malformed checksum entry on line %d", lineNumber)
		}
		if err := validateSHA256(parts[0]); err != nil {
			return nil, fmt.Errorf("malformed checksum entry on line %d: %w", lineNumber, err)
		}
		if _, exists := checksums[parts[1]]; exists {
			return nil, fmt.Errorf("duplicate checksum entry for %s", parts[1])
		}

		checksums[parts[1]] = strings.ToLower(parts[0])
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read checksums: %w", err)
	}
	if len(checksums) == 0 {
		return nil, fmt.Errorf("checksums file is empty")
	}

	return checksums, nil
}

func validateSHA256(checksum string) error {
	if len(checksum) != sha256.Size*2 {
		return fmt.Errorf("SHA-256 checksum must contain exactly 64 hexadecimal characters")
	}
	if _, err := hex.DecodeString(checksum); err != nil {
		return fmt.Errorf("SHA-256 checksum must contain only hexadecimal characters")
	}
	return nil
}

// DownloadVerifiedBinary downloads a checksum manifest and binary, then
// verifies the binary before returning any data to the installer.
func DownloadVerifiedBinary(binaryURL, checksumURL, assetName string, progress func(downloaded, total int64)) ([]byte, error) {
	expectedChecksum, err := FetchChecksum(checksumURL, assetName)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain checksum: %w", err)
	}

	binary, err := DownloadBinary(binaryURL, progress)
	if err != nil {
		return nil, err
	}
	if err := VerifyChecksum(binary, expectedChecksum); err != nil {
		return nil, err
	}

	return binary, nil
}

func VerifyChecksum(data []byte, expectedChecksum string) error {
	if err := validateSHA256(expectedChecksum); err != nil {
		return fmt.Errorf("invalid expected checksum: %w", err)
	}

	hash := sha256.Sum256(data)
	actual := hex.EncodeToString(hash[:])

	if !strings.EqualFold(actual, expectedChecksum) {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actual)
	}

	return nil
}

func ReplaceBinary(newBinary []byte) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve symlinks: %w", err)
	}

	info, err := os.Stat(execPath)
	if err != nil {
		return fmt.Errorf("failed to stat executable: %w", err)
	}

	dir := filepath.Dir(execPath)
	tmpFile, err := os.CreateTemp(dir, "cernopendata-client-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	defer func() {
		if tmpPath != "" {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.Write(newBinary); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write new binary: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Chmod(tmpPath, info.Mode()); err != nil { // #nosec G703
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	if err := os.Rename(tmpPath, execPath); err != nil { // #nosec G703
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	tmpPath = ""

	return nil
}

func IsHomebrewInstall() bool {
	execPath, err := os.Executable()
	if err != nil {
		return false
	}

	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return false
	}

	homebrewPaths := []string{
		"/opt/homebrew/",
		"/usr/local/Cellar/",
		"/home/linuxbrew/",
		"/.linuxbrew/",
	}

	for _, prefix := range homebrewPaths {
		if strings.Contains(execPath, prefix) {
			return true
		}
	}

	return false
}
