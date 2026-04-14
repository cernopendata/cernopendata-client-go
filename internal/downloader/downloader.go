package downloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/clelange/cernopendata-client-go/internal/config"
	"github.com/clelange/cernopendata-client-go/internal/printer"
	"github.com/clelange/cernopendata-client-go/internal/progress"
)

type DownloadStats struct {
	TotalFiles      int
	TotalBytes      int64
	DownloadedFiles int
	DownloadedBytes int64
	FailedFiles     int
	SkippedFiles    int
}

type FileDownloadResult struct {
	URL      string
	Path     string
	Size     int64
	Checksum string
	Success  bool
	Error    error
	Retries  int
}

type Downloader struct {
	client       *http.Client
	retryLimit   int
	retrySleep   int
	verbose      bool
	dryRun       bool
	showProgress bool
}

func NewDownloader() *Downloader {
	return &Downloader{
		client: &http.Client{
			Timeout: 30 * time.Minute,
		},
		retryLimit: config.DownloadRetryLimit,
		retrySleep: config.DownloadRetrySleep,
	}
}

func (d *Downloader) DownloadFile(url, destPath string, resume bool, expectedSize int64) (*FileDownloadResult, error) {
	var lastErr error
	var attempt int

	for attempt = 0; attempt < d.retryLimit; attempt++ {
		if attempt > 0 {
			printer.DisplayMessage(printer.Note, fmt.Sprintf("Retry attempt %d/%d after %ds...", attempt+1, d.retryLimit, d.retrySleep))
			time.Sleep(time.Duration(d.retrySleep) * time.Second)
		}

		// Strict resume: re-check file size on each attempt and resume from the true end.
		var existingSize int64
		if resume {
			if fi, err := os.Stat(destPath); err == nil {
				existingSize = fi.Size()
				if existingSize > 0 {
					printer.DisplayMessage(printer.Note, fmt.Sprintf("Resuming %s from %d bytes", destPath, existingSize))
				}
			} else if !os.IsNotExist(err) {
				return nil, fmt.Errorf("error checking file: %w", err)
			}
		}

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		if resume && existingSize > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingSize))
		}

		resp, err := d.client.Do(req) // #nosec G704
		if err != nil {
			lastErr = err
			printer.DisplayMessage(printer.Note, fmt.Sprintf("Download failed: %v", err))
			continue
		}

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
			printer.DisplayMessage(printer.Note, fmt.Sprintf("Server error: %d", resp.StatusCode))
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0750); err != nil {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}

		if resume && existingSize > 0 && resp.StatusCode == http.StatusOK {
			// Server ignored Range; restart from scratch.
			existingSize = 0
		}

		var file *os.File
		if resume && existingSize > 0 && resp.StatusCode == http.StatusPartialContent {
			file, err = os.OpenFile(destPath, os.O_APPEND|os.O_WRONLY, 0600) // #nosec G302 G304
		} else {
			file, err = os.Create(destPath) // #nosec G304
		}

		if err != nil {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("failed to open file: %w", err)
		}

		var written int64
		if d.showProgress {
			// Use content length from response, or fall back to expected size
			totalSize := resp.ContentLength
			isResumed := resume && existingSize > 0 && resp.StatusCode == http.StatusPartialContent

			if isResumed && totalSize > 0 {
				totalSize += existingSize
			}
			if totalSize <= 0 {
				totalSize = expectedSize
			}
			// When resuming, totalSize is the full file size
			pw := progress.NewWriter(file, filepath.Base(destPath), totalSize)
			if isResumed {
				pw.SetInitialProgress(existingSize)
			}
			written, err = io.Copy(pw, resp.Body)
			pw.Finish()
		} else {
			written, err = io.Copy(file, resp.Body)
		}
		_ = file.Close()
		_ = resp.Body.Close()

		if err != nil {
			lastErr = err
			printer.DisplayMessage(printer.Note, fmt.Sprintf("Write error: %v", err))
			continue
		}

		if d.verbose {
			printer.DisplayMessage(printer.Note, fmt.Sprintf("Downloaded %d bytes to %s", written, destPath))
		}

		return &FileDownloadResult{
			URL:     url,
			Path:    destPath,
			Success: true,
			Size:    existingSize + written,
			Retries: attempt,
		}, nil
	}

	return &FileDownloadResult{
		URL:     url,
		Path:    destPath,
		Success: false,
		Error:   lastErr,
		Retries: attempt - 1,
	}, lastErr
}

func (d *Downloader) DownloadFiles(files []any, baseDir string, retry int, retrySleep int, verbose bool, dryRun bool, showProgress bool) DownloadStats {
	d.retryLimit = retry
	d.retrySleep = retrySleep
	d.verbose = verbose
	d.dryRun = dryRun
	d.showProgress = showProgress
	AssignLocalPaths(files)

	stats := DownloadStats{}
	stats.TotalFiles = len(files)

	if err := os.MkdirAll(baseDir, 0750); err != nil {
		printer.DisplayMessage(printer.Error, fmt.Sprintf("Failed to create directory %s: %v", baseDir, err))
		return stats
	}

	for i, file := range files {
		fileMap, ok := file.(map[string]any)
		if !ok {
			printer.DisplayMessage(printer.Note, fmt.Sprintf("Skipping invalid file entry %d", i))
			stats.SkippedFiles++
			continue
		}

		uri, _ := fileMap["uri"].(string)
		size, _ := fileMap["size"].(float64)
		checksum, _ := fileMap["checksum"].(string)
		displayPath := DisplayPath(fileMap)

		stats.TotalBytes += int64(size)

		printer.DisplayMessage(printer.Info, fmt.Sprintf("Downloading file %d/%d: %s", i+1, stats.TotalFiles, displayPath))

		if dryRun {
			printer.DisplayMessage(printer.Note, fmt.Sprintf("Would download: %s -> %s (size: %d, checksum: %s)", uri, displayPath, int64(size), checksum))
			stats.DownloadedFiles++
			stats.DownloadedBytes += int64(size)
			continue
		}

		destPath := DestinationPath(baseDir, fileMap)

		if fi, err := os.Stat(destPath); err == nil {
			if fi.Size() >= int64(size) {
				printer.DisplayMessage(printer.Note, fmt.Sprintf("File already exists: %s", destPath))
				stats.SkippedFiles++
				continue
			}
		}

		result, err := d.DownloadFile(uri, destPath, true, int64(size))
		if err != nil {
			printer.DisplayMessage(printer.Error, fmt.Sprintf("Failed to download %s: %v", uri, err))
			stats.FailedFiles++
		} else if result.Success {
			stats.DownloadedFiles++
			stats.DownloadedBytes += result.Size
		}
	}

	printer.DisplayMessage(printer.Info, "\nDownload summary:")
	printer.DisplayMessage(printer.Note, fmt.Sprintf("  Total files:     %d", stats.TotalFiles))
	printer.DisplayMessage(printer.Note, fmt.Sprintf("  Downloaded:     %d", stats.DownloadedFiles))
	printer.DisplayMessage(printer.Note, fmt.Sprintf("  Skipped:        %d", stats.SkippedFiles))
	printer.DisplayMessage(printer.Note, fmt.Sprintf("  Failed:         %d", stats.FailedFiles))
	printer.DisplayMessage(printer.Note, fmt.Sprintf("  Total bytes:    %d", stats.DownloadedBytes))

	return stats
}

func ParseFileList(files []any) []any {
	return files
}

func FilterFiles(files []any, filter string) []any {
	if filter == "" {
		return files
	}

	var result []any
	for _, file := range files {
		fileMap, ok := file.(map[string]any)
		if !ok {
			continue
		}

		uri, _ := fileMap["uri"].(string)
		matched, err := filepath.Match(filter, filepath.Base(uri))
		if err != nil {
			continue
		}
		if matched {
			result = append(result, file)
		}
	}

	return result
}

func FilterFilesByRange(files []any, start, end int) []any {
	if start < 0 || end < 0 {
		return files
	}

	total := len(files)

	if start >= total {
		return []any{}
	}

	if end > total {
		end = total
	}

	if start > end {
		return []any{}
	}

	return files[start:end]
}

func FilterFilesByMultipleRanges(files []any, ranges [][2]int) []any {
	if len(ranges) == 0 {
		return files
	}

	var result []any
	for _, r := range ranges {
		start := r[0]
		end := r[1]

		if start > 0 {
			start--
		}

		filtered := FilterFilesByRange(files, start, end)
		result = append(result, filtered...)
	}

	return result
}

func FilterFilesByMultipleNames(files []any, filters []string) []any {
	if len(filters) == 0 {
		return files
	}

	var result []any
	for _, filter := range filters {
		filtered := FilterFiles(files, filter)
		result = append(result, filtered...)
	}

	return result
}

func FilterFilesByRegex(files []any, pattern string) []any {
	if pattern == "" {
		return files
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return files
	}

	var result []any
	for _, file := range files {
		fileMap, ok := file.(map[string]any)
		if !ok {
			continue
		}

		uri, _ := fileMap["uri"].(string)
		if re.MatchString(filepath.Base(uri)) {
			result = append(result, file)
		}
	}

	return result
}
