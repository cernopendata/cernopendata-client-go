package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cernopendata/cernopendata-client-go/internal/config"
	"github.com/cernopendata/cernopendata-client-go/internal/downloader"
	"github.com/cernopendata/cernopendata-client-go/internal/filemetadata"
	"github.com/cernopendata/cernopendata-client-go/internal/printer"
	"github.com/cernopendata/cernopendata-client-go/internal/searcher"
	"github.com/cernopendata/cernopendata-client-go/internal/utils"
	"github.com/cernopendata/cernopendata-client-go/internal/verifier"
	"github.com/cernopendata/cernopendata-client-go/internal/xrootddownloader"
)

type xrootdFileDownloader interface {
	DownloadFiles(context.Context, []filemetadata.File, string, int, int, bool, bool, bool) downloader.DownloadStats
	Close() error
}

type xrootdDownloaderFactory func() xrootdFileDownloader

func newDownloadFilesCommand() *cobra.Command {
	return newDownloadFilesCommandWithXRootD(func() xrootdFileDownloader {
		return xrootddownloader.NewDownloader()
	})
}

func newDownloadFilesCommandWithXRootD(newXRootDDownloader xrootdDownloaderFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "download-files",
		Short: "Download files from a record",
		Long: `Download data files belonging to a record.

Select a CERN Open Data bibliographic record by a record ID, a
DOI, or a title and download data files belonging to this record.

Examples:

     $ cernopendata-client download-files --recid 5500

     $ cernopendata-client download-files --recid 5500 --filter-name BuildFile.xml

     $ cernopendata-client download-files --recid 5500 --filter-regexp py$

     $ cernopendata-client download-files --recid 5500 --filter-range 1-4

     $ cernopendata-client download-files --recid 5500 --filter-range 1-2,5-7`,
		RunE: func(cmd *cobra.Command, args []string) error {
			recid, err := cmd.Flags().GetInt("recid")
			if err != nil {
				return commandErrorf("Invalid recid: %w", err)
			}
			doi, _ := cmd.Flags().GetString("doi")
			title, _ := cmd.Flags().GetString("title")
			outputDir, _ := cmd.Flags().GetString("output-dir")
			filterName, _ := cmd.Flags().GetString("filter-name")
			filterRegexp, _ := cmd.Flags().GetString("filter-regexp")
			filterRange, _ := cmd.Flags().GetString("filter-range")
			expand, _ := cmd.Flags().GetBool("expand")
			noExpand, _ := cmd.Flags().GetBool("no-expand")
			retryLimit, _ := cmd.Flags().GetInt("retry-limit")
			retrySleep, _ := cmd.Flags().GetInt("retry-sleep")
			verbose, _ := cmd.Flags().GetBool("verbose")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			verifyFlag, _ := cmd.Flags().GetBool("verify")
			downloadEngine, _ := cmd.Flags().GetString("download-engine")
			protocol, _ := cmd.Flags().GetString("protocol")
			server, _ := cmd.Flags().GetString("server")
			fileAvailability, _ := cmd.Flags().GetString("file-availability")

			if fileAvailability != "" && fileAvailability != "online" && fileAvailability != "all" {
				return commandErrorf("Invalid file availability: %s (choose from 'online', 'all')", fileAvailability)
			}

			if cmd.Flags().Changed("expand") && cmd.Flags().Changed("no-expand") {
				return commandErrorf("Cannot specify both --expand and --no-expand")
			}

			if noExpand {
				expand = false
			}

			if server == "" {
				server = config.ServerHTTPURI
			}

			parsedRecid, err := searcher.GetRecid(server, doi, title, recid)
			if err != nil {
				return commandErrorf("Failed to find record: %w", err)
			}

			if outputDir == "" {
				outputDir = fmt.Sprintf("%d", parsedRecid)
			}

			client := searcher.NewClient(server)
			record, err := client.GetRecord(parsedRecid)
			if err != nil {
				return commandErrorf("Failed to get record: %w", err)
			}

			if protocol == "" {
				if downloadEngine == "xrootd" {
					protocol = "xrootd"
				} else {
					protocol = "http"
				}
			}

			files, err := client.GetFilesList(record, protocol, expand)
			if err != nil {
				return commandErrorf("Failed to get files list: %w", err)
			}
			totalFiles := len(files)
			var totalBytes int64
			for _, f := range files {
				totalBytes += f.Size
			}

			tapeFilesSkipped := 0
			if expand {
				// Check if we have offline files
				hasOfflineFiles := false
				for _, f := range files {
					if f.Availability != "" && f.Availability != "online" {
						hasOfflineFiles = true
						break
					}
				}

				// Apply filtering logic
				if fileAvailability == "online" {
					files, _ = searcher.FilterFilesByAvailability(files, "online")
				} else if fileAvailability == "" && hasOfflineFiles {
					// Default behavior: warn and skip offline files
					printer.DisplayMessage(printer.Warning, "Some files are stored on tape and will be skipped.")
					printer.DisplayMessage(printer.Warning, fmt.Sprintf("Visit https://opendata.cern.ch/record/%d to request file staging.", parsedRecid))
					printer.DisplayMessage(printer.Warning, "Use '--file-availability all' to force attempting to download all files.")
					files, _ = searcher.FilterFilesByAvailability(files, "online")
				}
				// If "all", we keep everything (user explicitly requested it)

				tapeFilesSkipped = totalFiles - len(files)
			}
			fileList := files

			if filterName != "" {
				nameFilters := strings.Split(filterName, ",")
				for i, filter := range nameFilters {
					nameFilters[i] = strings.TrimSpace(filter)
				}
				fileList = downloader.FilterFilesByMultipleNames(fileList, nameFilters)
			}

			if filterRegexp != "" {
				fileList = downloader.FilterFilesByRegex(fileList, filterRegexp)
			}

			if filterRange != "" {
				ranges, err := utils.ParseRanges([]string{filterRange})
				if err != nil {
					return commandErrorf("Invalid range filter: %w", err)
				}
				fileList = downloader.FilterFilesByMultipleRanges(fileList, ranges)
			}

			if len(fileList) == 0 {
				return commandErrorf("No files matching filters")
			}

			var stats downloader.DownloadStats
			if downloadEngine == "xrootd" {
				xrdDownloader := newXRootDDownloader()
				defer func() {
					_ = xrdDownloader.Close()
				}()
				// Enable progress when --progress or --verbose flags are set
				showProgress := verbose
				if progressFlag, _ := cmd.Flags().GetBool("progress"); progressFlag {
					showProgress = true
				}
				stats = xrdDownloader.DownloadFiles(cmd.Context(), fileList, outputDir, retryLimit, retrySleep, verbose, dryRun, showProgress)
			} else {
				httpDownloader := downloader.NewDownloader()
				// Enable progress when --progress or --verbose flags are set
				showProgress := verbose
				if progressFlag, _ := cmd.Flags().GetBool("progress"); progressFlag {
					showProgress = true
				}
				stats = httpDownloader.DownloadFiles(fileList, outputDir, retryLimit, retrySleep, verbose, dryRun, showProgress)
			}

			if verifyFlag {
				printer.DisplayMessage(printer.Info, "\nVerifying downloaded files...")
				v := verifier.NewVerifier()
				verifyStats, err := v.VerifyFiles(outputDir, fileList)
				if err != nil {
					return commandErrorf("Verification failed: %w", err)
				}

				if verifyStats.SizeFailed > 0 || verifyStats.ChecksumFailed > 0 {
					return commandErrorf("Some files failed verification")
				}
			}

			if stats.FailedFiles == 0 {
				printer.DisplayMessage(printer.Info, "Success!")
			}

			// Print summary statistics
			printer.DisplayOutput("")
			printer.DisplayOutput("Summary:")
			printer.DisplayOutput(fmt.Sprintf("- Files downloaded: %d / %d", stats.DownloadedFiles, totalFiles))
			if tapeFilesSkipped > 0 {
				printer.DisplayOutput(fmt.Sprintf("- Files skipped (on tape): %d", tapeFilesSkipped))
			}
			printer.DisplayOutput(fmt.Sprintf("- Bytes downloaded: %s / %s", utils.FormatBytes(float64(stats.DownloadedBytes)), utils.FormatBytes(float64(totalBytes))))

			if stats.FailedFiles > 0 {
				return fmt.Errorf("%d file(s) failed to download", stats.FailedFiles)
			}
			return nil
		},
	}
	cmd.Flags().IntP("recid", "R", 0, "Record ID (exact match)")
	cmd.Flags().StringP("doi", "d", "", "Digital Object Identifier (exact match)")
	cmd.Flags().StringP("title", "t", "", "Record title (exact match, no wildcards)")
	cmd.Flags().StringP("output-dir", "O", "", "Output directory")
	cmd.Flags().StringP("filter-name", "n", "", "Download files matching exactly the file name")
	cmd.Flags().StringP("filter-regexp", "e", "", "Download files matching the regular expression")
	cmd.Flags().StringP("filter-range", "r", "", "Download files from a specified list range (i-j)")
	cmd.Flags().BoolP("expand", "x", true, "Expand file indexes?")
	cmd.Flags().Bool("no-expand", false, "Don't expand file indexes")
	cmd.Flags().IntP("retry-limit", "y", 10, "Number of retries when downloading a file")
	cmd.Flags().IntP("retry-sleep", "Y", 5, "Sleep time in seconds before retrying downloads")
	cmd.Flags().BoolP("verbose", "v", false, "Verbose output")
	cmd.Flags().BoolP("progress", "P", false, "Show progress (alias for verbose)")
	cmd.Flags().BoolP("dry-run", "N", false, "Dry run (don't actually download)")
	cmd.Flags().BoolP("verify", "V", false, "Verify downloaded files")
	cmd.Flags().String("download-engine", "", "Download engine to use (http|xrootd)")
	cmd.Flags().StringP("protocol", "p", "", "Protocol to be used in links [http,xrootd]")
	cmd.Flags().StringP("server", "s", "", "Which CERN Open Data server to query? [default=http://opendata.cern.ch]")
	cmd.Flags().StringP("file-availability", "", "", "Filter files by their availability status [online, all]")
	return cmd
}
