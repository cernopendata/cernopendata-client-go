package verifier

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cernopendata/cernopendata-client-go/internal/checksum"
	"github.com/cernopendata/cernopendata-client-go/internal/downloader"
	"github.com/cernopendata/cernopendata-client-go/internal/filemetadata"
	"github.com/cernopendata/cernopendata-client-go/internal/printer"
)

type VerificationResult struct {
	Path          string
	ExpectedSize  int64
	ActualSize    int64
	ExpectedSum   string
	ActualSum     string
	SizeMatch     bool
	ChecksumMatch bool
	FileExists    bool
}

type VerificationStats struct {
	TotalFiles     int
	VerifiedFiles  int
	SizeFailed     int
	ChecksumFailed int
	MissingFiles   int
}

type Verifier struct{}

func NewVerifier() *Verifier {
	return &Verifier{}
}

func (v *Verifier) VerifyLocalFiles(directory string) (*VerificationStats, error) {
	stats := &VerificationStats{}

	files, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", directory, err)
	}

	stats.TotalFiles = len(files)

	for _, file := range files {
		if file.IsDir() {
			stats.TotalFiles--
			continue
		}

		filePath := filepath.Join(directory, file.Name())

		actualSize, err := checksum.GetFileSize(filePath)
		if err != nil {
			stats.MissingFiles++
			printer.DisplayMessage(printer.Error, fmt.Sprintf("Failed to get size: %s", filePath))
			continue
		}

		actualChecksum, err := checksum.CalculateChecksum(filePath)
		if err != nil {
			stats.MissingFiles++
			printer.DisplayMessage(printer.Error, fmt.Sprintf("Failed to calculate checksum: %s", filePath))
			continue
		}

		printer.DisplayOutput(fmt.Sprintf("%s\t%d\t%s", filePath, actualSize, actualChecksum))
		stats.VerifiedFiles++
	}

	return stats, nil
}

func (v *Verifier) VerifyFiles(directory string, expectedFiles []filemetadata.File) (*VerificationStats, error) {
	stats := &VerificationStats{}
	stats.TotalFiles = len(expectedFiles)
	downloader.AssignLocalPaths(expectedFiles)

	for _, file := range expectedFiles {
		displayPath := downloader.DisplayPath(file)
		filePath := downloader.DestinationPath(directory, file)

		result := VerificationResult{
			Path:         filePath,
			ExpectedSize: file.Size,
			ExpectedSum:  file.Checksum,
		}

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			result.FileExists = false
			stats.MissingFiles++
			printer.DisplayMessage(printer.Error, fmt.Sprintf("File not found: %s", filePath))
			continue
		}
		result.FileExists = true

		actualSize, err := checksum.GetFileSize(filePath)
		if err != nil {
			stats.MissingFiles++
			printer.DisplayMessage(printer.Error, fmt.Sprintf("Failed to get size: %s", filePath))
			continue
		}
		result.ActualSize = actualSize

		actualChecksum, err := checksum.CalculateChecksum(filePath)
		if err != nil {
			stats.MissingFiles++
			printer.DisplayMessage(printer.Error, fmt.Sprintf("Failed to calculate checksum: %s", filePath))
			continue
		}
		result.ActualSum = actualChecksum

		result.SizeMatch = (result.ActualSize == result.ExpectedSize)
		result.ChecksumMatch = (result.ActualSum == result.ExpectedSum)

		if !result.SizeMatch {
			stats.SizeFailed++
			printer.DisplayMessage(printer.Error, fmt.Sprintf("Size mismatch: %s (expected: %d, actual: %d)",
				displayPath, result.ExpectedSize, result.ActualSize))
		}

		if !result.ChecksumMatch {
			stats.ChecksumFailed++
			printer.DisplayMessage(printer.Error, fmt.Sprintf("Checksum mismatch: %s (expected: %s, actual: %s)",
				displayPath, result.ExpectedSum, result.ActualSum))
		}

		if result.SizeMatch && result.ChecksumMatch {
			stats.VerifiedFiles++
			printer.DisplayMessage(printer.Info, fmt.Sprintf("Verified: %s", displayPath))
		}
	}

	return stats, nil
}

func (v *Verifier) GetFileChecksum(filePath string) (string, error) {
	return checksum.CalculateChecksum(filePath)
}

func (v *Verifier) GetFileSize(filePath string) (int64, error) {
	return checksum.GetFileSize(filePath)
}

func ParseChecksumMetadata(metadata string) (string, int64, error) {
	parts := strings.Split(metadata, "\t")
	if len(parts) < 2 {
		return "", 0, fmt.Errorf("invalid metadata format")
	}

	checksum := strings.TrimSpace(parts[0])
	sizeStr := strings.TrimSpace(parts[1])
	var size int64
	_, err := fmt.Sscanf(sizeStr, "%d", &size)
	if err != nil {
		return "", 0, fmt.Errorf("invalid size format: %w", err)
	}

	return checksum, size, nil
}
