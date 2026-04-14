package xrootddownloader

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-hep.org/x/hep/xrootd"
	"go-hep.org/x/hep/xrootd/xrdfs"
	"go-hep.org/x/hep/xrootd/xrdio"

	"github.com/clelange/cernopendata-client-go/internal/config"
	"github.com/clelange/cernopendata-client-go/internal/downloader"
	"github.com/clelange/cernopendata-client-go/internal/printer"
	"github.com/clelange/cernopendata-client-go/internal/utils"
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
	client       *xrootd.Client
	retryLimit   int
	retrySleep   int
	verbose      bool
	dryRun       bool
	showProgress bool
	address      string
	username     string
}

func NewDownloader() *Downloader {
	return &Downloader{
		retryLimit: 10,
		retrySleep: 5,
		username:   "gopher",
	}
}

// printProgress prints the current download progress.
func (d *Downloader) printProgress(filename string, downloaded int64, initial int64, total int64, startTime time.Time, final bool) {
	elapsed := time.Since(startTime).Seconds()
	if elapsed == 0 {
		elapsed = 0.001 // Avoid division by zero
	}

	rate := float64(downloaded) / elapsed
	rateStr := utils.FormatRate(rate)
	downloadedStr := utils.FormatBytes(float64(downloaded))

	var line string
	if total > 0 {
		currentTotal := downloaded + initial
		percentage := float64(currentTotal) / float64(total) * 100
		if percentage > 100 {
			percentage = 100
		}
		totalStr := utils.FormatBytes(float64(total))
		currentStr := utils.FormatBytes(float64(currentTotal))

		if final {
			line = fmt.Sprintf("  -> %s: %.1f%% (%s / %s) [%s avg] in %.1fs",
				filename, percentage, currentStr, totalStr, rateStr, elapsed)
		} else {
			line = fmt.Sprintf("  -> %s: %.1f%% (%s / %s) [%s]",
				filename, percentage, currentStr, totalStr, rateStr)
		}
	} else {
		// Unknown total size
		if final {
			line = fmt.Sprintf("  -> %s: %s [%s avg] in %.1fs",
				filename, downloadedStr, rateStr, elapsed)
		} else {
			line = fmt.Sprintf("  -> %s: %s [%s]",
				filename, downloadedStr, rateStr)
		}
	}

	// Pad with spaces to overwrite any previous longer line
	padding := 80 - len(line)
	if padding > 0 {
		line += strings.Repeat(" ", padding)
	}

	if final {
		_, _ = fmt.Printf("\r%s\n", line)
	} else {
		_, _ = fmt.Printf("\r%s", line)
	}
}

func (d *Downloader) DownloadFile(ctx context.Context, url, destPath string, resume bool, expectedSize int64) (*FileDownloadResult, error) {
	parsedURL, err := xrdio.Parse(url)
	if err != nil {
		return nil, fmt.Errorf("failed to parse XRootD URL: %w", err)
	}

	if d.client == nil {
		client, err := xrootd.NewClient(ctx, parsedURL.Addr, d.username)
		if err != nil {
			return nil, fmt.Errorf("failed to create XRootD client: %w", err)
		}
		d.client = client
		d.address = parsedURL.Addr
	}

	fs := d.client.FS()

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

		var offset int64 = 0
		if resume && existingSize > 0 {
			offset = existingSize
		}

		file, err := fs.Open(ctx, parsedURL.Path, xrdfs.OpenModeOwnerRead, xrdfs.OpenOptionsOpenRead|xrdfs.OpenOptionsSequentiallyIO)
		if err != nil {
			lastErr = err
			printer.DisplayMessage(printer.Note, fmt.Sprintf("Failed to open file: %v", err))
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0750); err != nil {
			_ = file.Close(ctx)
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}

		var localFile *os.File
		if resume && existingSize > 0 {
			localFile, err = os.OpenFile(destPath, os.O_APPEND|os.O_WRONLY, 0600) // #nosec G302 G304
		} else {
			localFile, err = os.Create(destPath) // #nosec G304
		}

		if err != nil {
			_ = file.Close(ctx)
			return nil, fmt.Errorf("failed to open local file: %w", err)
		}

		bufSize := config.XRootDReadBufferSize
		if expectedSize > 0 && expectedSize < int64(bufSize) {
			bufSize = int(expectedSize)
		}
		buf := make([]byte, bufSize)
		var copied int64
		totalBytes := offset
		filename := filepath.Base(destPath)
		startTime := time.Now()
		lastProgressUpdate := time.Time{}
		progressUpdateInterval := 200 * time.Millisecond

	retryLoop:
		for {
			select {
			case <-ctx.Done():
				_ = localFile.Close()
				_ = file.Close(ctx)
				return &FileDownloadResult{
					URL:     url,
					Path:    destPath,
					Success: false,
					Error:   ctx.Err(),
					Retries: attempt,
				}, ctx.Err()
			default:
			}

			n, err := file.ReadAt(buf, totalBytes)
			if n > 0 {
				if _, err := localFile.Write(buf[:n]); err != nil {
					lastErr = err
					printer.DisplayMessage(printer.Note, fmt.Sprintf("Write error: %v", err))
					break retryLoop
				}
				copied += int64(n)
				totalBytes += int64(n)

				// Print progress if enabled
				if d.showProgress && time.Since(lastProgressUpdate) >= progressUpdateInterval {
					d.printProgress(filename, copied, existingSize, expectedSize, startTime, false)
					lastProgressUpdate = time.Now()
				}
			}
			if err == io.EOF {
				break retryLoop
			}
			if err != nil {
				lastErr = err
				printer.DisplayMessage(printer.Note, fmt.Sprintf("Read error: %v", err))
				break retryLoop
			}
			if n == 0 {
				printer.DisplayMessage(printer.Note, "Read returned 0 bytes, assuming EOF")
				break retryLoop
			}
		}

		if err := localFile.Close(); err != nil {
			_ = file.Close(ctx)
			lastErr = err
			printer.DisplayMessage(printer.Note, fmt.Sprintf("Close error: %v", err))
			continue
		}
		_ = file.Close(ctx)

		if lastErr != nil {
			continue
		}

		if d.showProgress {
			d.printProgress(filename, copied, existingSize, expectedSize, startTime, true)
		}

		if d.verbose {
			printer.DisplayMessage(printer.Note, fmt.Sprintf("Downloaded %d bytes to %s", copied, destPath))
		}

		return &FileDownloadResult{
			URL:     url,
			Path:    destPath,
			Success: true,
			Size:    existingSize + copied,
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

func (d *Downloader) DownloadFiles(ctx context.Context, files []any, baseDir string, retry int, retrySleep int, verbose bool, dryRun bool, showProgress bool) DownloadStats {
	d.retryLimit = retry
	d.retrySleep = retrySleep
	d.verbose = verbose
	d.dryRun = dryRun
	d.showProgress = showProgress
	downloader.AssignLocalPaths(files)

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
		displayPath := downloader.DisplayPath(fileMap)

		stats.TotalBytes += int64(size)

		printer.DisplayMessage(printer.Info, fmt.Sprintf("Downloading file %d/%d: %s", i+1, stats.TotalFiles, displayPath))

		if dryRun {
			printer.DisplayMessage(printer.Note, fmt.Sprintf("Would download: %s -> %s (size: %d, checksum: %s)", uri, displayPath, int64(size), checksum))
			stats.DownloadedFiles++
			stats.DownloadedBytes += int64(size)
			continue
		}

		destPath := downloader.DestinationPath(baseDir, fileMap)

		if fi, err := os.Stat(destPath); err == nil {
			if fi.Size() >= int64(size) {
				printer.DisplayMessage(printer.Note, fmt.Sprintf("File already exists: %s", destPath))
				stats.SkippedFiles++
				continue
			}
		}

		result, err := d.DownloadFile(ctx, uri, destPath, true, int64(size))
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

func (d *Downloader) Close() error {
	if d.client != nil {
		return d.client.Close()
	}
	return nil
}
