package progress

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/cernopendata/cernopendata-client-go/internal/utils"
)

// Writer wraps an io.Writer to track and display download progress.
type Writer struct {
	writer       io.Writer
	totalBytes   int64
	writtenBytes int64
	initialBytes int64
	startTime    time.Time
	lastUpdate   time.Time
	filename     string
	output       io.Writer
	updateEvery  time.Duration
}

// NewWriter creates a new progress Writer.
// If totalBytes is 0 or negative, percentage won't be displayed.
func NewWriter(writer io.Writer, filename string, totalBytes int64) *Writer {
	return &Writer{
		writer:      writer,
		totalBytes:  totalBytes,
		filename:    filename,
		startTime:   time.Now(),
		lastUpdate:  time.Time{},
		output:      os.Stdout,
		updateEvery: 200 * time.Millisecond,
	}
}

// SetInitialProgress sets the number of bytes already present (for resumed downloads).
func (pw *Writer) SetInitialProgress(bytes int64) {
	pw.initialBytes = bytes
}

// Write implements io.Writer and tracks progress.
func (pw *Writer) Write(p []byte) (n int, err error) {
	n, err = pw.writer.Write(p)
	pw.writtenBytes += int64(n)

	now := time.Now()
	if now.Sub(pw.lastUpdate) >= pw.updateEvery {
		pw.printProgress(false)
		pw.lastUpdate = now
	}

	return n, err
}

// Finish prints the final progress line with completion statistics.
func (pw *Writer) Finish() {
	pw.printProgress(true)
}

// printProgress prints the current progress to output.
func (pw *Writer) printProgress(final bool) {
	elapsed := time.Since(pw.startTime).Seconds()
	if elapsed == 0 {
		elapsed = 0.001 // Avoid division by zero
	}

	rate := float64(pw.writtenBytes) / elapsed
	rateStr := utils.FormatRate(rate)
	writtenStr := utils.FormatBytes(float64(pw.writtenBytes))

	var line string
	if pw.totalBytes > 0 {
		currentTotal := pw.writtenBytes + pw.initialBytes
		percentage := float64(currentTotal) / float64(pw.totalBytes) * 100
		if percentage > 100 {
			percentage = 100
		}
		totalStr := utils.FormatBytes(float64(pw.totalBytes))
		currentStr := utils.FormatBytes(float64(currentTotal))

		if final {
			line = fmt.Sprintf("  -> %s: %.1f%% (%s / %s) [%s avg] in %.1fs",
				pw.filename, percentage, currentStr, totalStr, rateStr, elapsed)
		} else {
			line = fmt.Sprintf("  -> %s: %.1f%% (%s / %s) [%s]",
				pw.filename, percentage, currentStr, totalStr, rateStr)
		}
	} else {
		// Unknown total size
		if final {
			line = fmt.Sprintf("  -> %s: %s [%s avg] in %.1fs",
				pw.filename, writtenStr, rateStr, elapsed)
		} else {
			line = fmt.Sprintf("  -> %s: %s [%s]",
				pw.filename, writtenStr, rateStr)
		}
	}

	// Pad with spaces to overwrite any previous longer line
	padding := 80 - len(line)
	if padding > 0 {
		line += strings.Repeat(" ", padding)
	}

	if final {
		_, _ = fmt.Fprintf(pw.output, "\r%s\n", line)
	} else {
		_, _ = fmt.Fprintf(pw.output, "\r%s", line)
	}
}

// WrittenBytes returns the number of bytes written so far.
func (pw *Writer) WrittenBytes() int64 {
	return pw.writtenBytes
}
