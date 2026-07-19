package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cernopendata/cernopendata-client-go/internal/config"
	"github.com/cernopendata/cernopendata-client-go/internal/downloader"
	"github.com/cernopendata/cernopendata-client-go/internal/printer"
	"github.com/cernopendata/cernopendata-client-go/internal/searcher"
	"github.com/cernopendata/cernopendata-client-go/internal/verifier"
)

var verifyFilesCmd = &cobra.Command{
	Use:   "verify-files",
	Short: "Verify local files against expected checksums and sizes",
	Long: `Verify downloaded data file integrity.

Select a CERN Open Data bibliographic record by a record ID, a
DOI, or a title and verify integrity of downloaded data files
belonging to this record.

Examples:

     $ cernopendata-client verify-files --recid 5500`,
	Run: func(cmd *cobra.Command, args []string) {
		recid, err := cmd.Flags().GetInt("recid")
		if err != nil {
			printer.DisplayMessage(printer.Error, fmt.Sprintf("Invalid recid: %v", err))
			os.Exit(1)
		}
		doi, _ := cmd.Flags().GetString("doi")
		title, _ := cmd.Flags().GetString("title")
		inputDir, _ := cmd.Flags().GetString("input-dir")
		filterName, _ := cmd.Flags().GetString("filter-name")
		filterRegexp, _ := cmd.Flags().GetString("filter-regexp")
		server, _ := cmd.Flags().GetString("server")

		if server == "" {
			server = config.ServerHTTPURI
		}

		parsedRecid, err := searcher.GetRecid(server, doi, title, recid)
		if err != nil {
			printer.DisplayMessage(printer.Error, fmt.Sprintf("Failed to find record: %v", err))
			os.Exit(1)
		}

		if inputDir == "" {
			inputDir = fmt.Sprintf("%d", parsedRecid)
		}

		client := searcher.NewClient(server)
		record, err := client.GetRecord(parsedRecid)
		if err != nil {
			printer.DisplayMessage(printer.Error, fmt.Sprintf("Failed to get record: %v", err))
			os.Exit(1)
		}

		files, err := client.GetFilesList(record, "http", false)
		if err != nil {
			printer.DisplayMessage(printer.Error, fmt.Sprintf("Failed to get files list: %v", err))
			os.Exit(1)
		}

		var fileList []any
		for _, file := range files {
			fileList = append(fileList, map[string]any{
				"uri":      file.URI,
				"size":     float64(file.Size),
				"checksum": file.Checksum,
			})
		}

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

		if len(fileList) == 0 {
			printer.DisplayMessage(printer.Error, "No files matching filters")
			os.Exit(1)
		}

		verifier := verifier.NewVerifier()
		stats, err := verifier.VerifyFiles(inputDir, fileList)
		if err != nil {
			printer.DisplayMessage(printer.Error, fmt.Sprintf("Verification failed: %v", err))
			os.Exit(1)
		}

		printer.DisplayMessage(printer.Info, fmt.Sprintf("Verifying number of files for record %d...", parsedRecid))
		printer.DisplayMessage(printer.Note, fmt.Sprintf("Expected %d, found %d", len(fileList), stats.VerifiedFiles+stats.SizeFailed+stats.ChecksumFailed+stats.MissingFiles))

		if len(fileList) != (stats.VerifiedFiles + stats.SizeFailed + stats.ChecksumFailed + stats.MissingFiles) {
			printer.DisplayMessage(printer.Error, "File count does not match.")
			os.Exit(1)
		}

		printer.DisplayMessage(printer.Info, "\nVerification summary:")
		printer.DisplayMessage(printer.Note, fmt.Sprintf("  Total files:     %d", stats.TotalFiles))
		printer.DisplayMessage(printer.Note, fmt.Sprintf("  Verified:        %d", stats.VerifiedFiles))
		printer.DisplayMessage(printer.Note, fmt.Sprintf("  Size errors:     %d", stats.SizeFailed))
		printer.DisplayMessage(printer.Note, fmt.Sprintf("  Checksum errors: %d", stats.ChecksumFailed))
		printer.DisplayMessage(printer.Note, fmt.Sprintf("  Missing files:   %d", stats.MissingFiles))

		if stats.SizeFailed > 0 || stats.ChecksumFailed > 0 || stats.MissingFiles > 0 {
			os.Exit(1)
		}

		printer.DisplayMessage(printer.Info, "Success!")
	},
}

func init() {
	verifyFilesCmd.Flags().IntP("recid", "r", 0, "Record ID (exact match)")
	verifyFilesCmd.Flags().StringP("doi", "d", "", "Digital Object Identifier (exact match)")
	verifyFilesCmd.Flags().StringP("title", "t", "", "Record title (exact match, no wildcards)")
	verifyFilesCmd.Flags().StringP("input-dir", "i", "", "Input directory containing files to verify")
	verifyFilesCmd.Flags().StringP("filter-name", "n", "", "Verify files matching exactly the file name")
	verifyFilesCmd.Flags().StringP("filter-regexp", "e", "", "Verify files matching the regular expression")
	verifyFilesCmd.Flags().StringP("server", "s", "", "Which CERN Open Data server to query? [default=http://opendata.cern.ch]")
}
