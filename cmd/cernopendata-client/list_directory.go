package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/cernopendata/cernopendata-client-go/internal/config"
	"github.com/cernopendata/cernopendata-client-go/internal/lister"
	"github.com/cernopendata/cernopendata-client-go/internal/printer"
)

// printEntries formats and prints directory entries based on verbose mode
func printEntries(entries []lister.FileInfo, verbose bool) {
	for _, entry := range entries {
		dirMarker := ""
		if entry.IsDir {
			dirMarker = "/"
		}
		if verbose {
			printer.DisplayOutput(fmt.Sprintf("%s\t%d\t%s%s", entry.Name, entry.Size, entry.ModTime, dirMarker))
		} else {
			printer.DisplayOutput(fmt.Sprintf("%s%s", entry.Name, dirMarker))
		}
	}
}

var listDirectoryCmd = &cobra.Command{
	Use:   "list-directory [path]",
	Short: "List contents of a EOSPUBLIC Open Data directory.",
	Long: `List contents of a EOSPUBLIC Open Data directory.

Returns the list of files and subdirectories of a given EOSPUBLIC directory.

Examples:

     $ cernopendata-client list-directory /eos/opendata/cms/validated-runs/Commissioning10

     $ cernopendata-client list-directory /eos/opendata/cms/Run2010B/BTau/AOD --recursive

     $ cernopendata-client list-directory /eos/opendata/cms/Run2010B --recursive --timeout 10

     $ cernopendata-client list-directory /eos/opendata/cms/Run2010B --format json`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]
		recursive, _ := cmd.Flags().GetBool("recursive")
		timeout, _ := cmd.Flags().GetInt("timeout")
		verbose, _ := cmd.Flags().GetBool("verbose")
		outputFormat, _ := cmd.Flags().GetString("format")

		if outputFormat != "text" && outputFormat != "json" {
			printer.DisplayMessage(printer.Error, fmt.Sprintf("Invalid format: %s (choose from 'text', 'json')", outputFormat))
			os.Exit(1)
		}

		ctx := cmd.Context()
		if timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
			defer cancel()
		}

		l := lister.NewLister()

		var entries []lister.FileInfo
		var err error

		if recursive {
			entries, err = l.ListDirectoryRecursive(ctx, path)
			if err != nil {
				printer.DisplayMessage(printer.Error, fmt.Sprintf("Failed to list directory: %v", err))
				os.Exit(1)
			}
		} else {
			entries, err = l.ListDirectory(ctx, path)
			if err != nil {
				printer.DisplayMessage(printer.Error, fmt.Sprintf("Failed to list directory: %v", err))
				os.Exit(1)
			}
		}

		if outputFormat == "json" {
			type DirOutput struct {
				Name    string `json:"name"`
				Size    int64  `json:"size,omitempty"`
				ModTime string `json:"mod_time,omitempty"`
				IsDir   bool   `json:"is_dir"`
			}

			var output []DirOutput
			for _, entry := range entries {
				dirEntry := DirOutput{
					Name:  entry.Name,
					IsDir: entry.IsDir,
				}
				if verbose {
					dirEntry.Size = entry.Size
					dirEntry.ModTime = entry.ModTime
				}
				output = append(output, dirEntry)
			}

			jsonBytes, err := json.MarshalIndent(output, "", "  ")
			if err != nil {
				printer.DisplayMessage(printer.Error, fmt.Sprintf("Failed to marshal JSON: %v", err))
				os.Exit(1)
			}
			printer.DisplayOutput(string(jsonBytes))
			return
		}

		printEntries(entries, verbose)
	},
}

func init() {
	listDirectoryCmd.Flags().BoolP("verbose", "v", false, "Verbose output")
	listDirectoryCmd.Flags().BoolP("recursive", "r", false, "Iterate recursively in the given directory path")
	listDirectoryCmd.Flags().IntP("timeout", "t", config.ListDirectoryTimeout, "Timeout in seconds after which to exit running the command")
	listDirectoryCmd.Flags().StringP("format", "m", "text", "Output format (text|json)")
}
