package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/cernopendata/cernopendata-client-go/internal/lister"
)

func TestPrintEntries(t *testing.T) {
	tests := []struct {
		name     string
		entries  []lister.FileInfo
		verbose  bool
		expected []string
	}{
		{
			name: "Basic output",
			entries: []lister.FileInfo{
				{Name: "file1", IsDir: false, Size: 100},
				{Name: "dir1", IsDir: true, Size: 0},
			},
			verbose: false,
			expected: []string{
				"file1",
				"dir1/",
			},
		},
		{
			name: "Verbose output",
			entries: []lister.FileInfo{
				{Name: "file1", IsDir: false, Size: 100, ModTime: "2023-01-01"},
				{Name: "dir1", IsDir: true, Size: 0, ModTime: "2023-01-02"},
			},
			verbose: true,
			expected: []string{
				"file1\t100\t2023-01-01",
				"dir1\t0\t2023-01-02/",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			printEntries(tt.entries, tt.verbose)

			_ = w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			_, _ = io.Copy(&buf, r)
			output := buf.String()

			for _, exp := range tt.expected {
				if !strings.Contains(output, exp) {
					t.Errorf("Expected output to contain %q, but it didn't.\nOutput:\n%s", exp, output)
				}
			}
		})
	}
}
