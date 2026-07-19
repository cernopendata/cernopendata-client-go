package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cernopendata/cernopendata-client-go/internal/config"
	"github.com/cernopendata/cernopendata-client-go/internal/downloader"
	"github.com/cernopendata/cernopendata-client-go/internal/printer"
	"github.com/cernopendata/cernopendata-client-go/internal/searcher"
	"github.com/cernopendata/cernopendata-client-go/internal/verifier"
)

func newVerifyFilesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify-files",
		Short: "Verify local files against expected checksums and sizes",
		Long: `Verify downloaded data file integrity.

Select a CERN Open Data bibliographic record by a record ID, a
DOI, or a title and verify integrity of downloaded data files
belonging to this record.

Examples:

     $ cernopendata-client verify-files --recid 5500`,
		RunE: func(cmd *cobra.Command, args []string) error {
			recid, err := cmd.Flags().GetInt("recid")
			if err != nil {
				return commandErrorf("Invalid recid: %w", err)
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
				return commandErrorf("Failed to find record: %w", err)
			}

			if inputDir == "" {
				inputDir = fmt.Sprintf("%d", parsedRecid)
			}

			client := searcher.NewClient(server)
			record, err := client.GetRecord(parsedRecid)
			if err != nil {
				return commandErrorf("Failed to get record: %w", err)
			}

			files, err := client.GetFilesList(record, "http", false)
			if err != nil {
				return commandErrorf("Failed to get files list: %w", err)
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

			if len(fileList) == 0 {
				return commandErrorf("No files matching filters")
			}

			verifier := verifier.NewVerifier()
			stats, err := verifier.VerifyFiles(inputDir, fileList)
			if err != nil {
				return commandErrorf("Verification failed: %w", err)
			}

			printer.DisplayMessage(printer.Info, fmt.Sprintf("Verifying number of files for record %d...", parsedRecid))
			printer.DisplayMessage(printer.Note, fmt.Sprintf("Expected %d, found %d", len(fileList), stats.VerifiedFiles+stats.SizeFailed+stats.ChecksumFailed+stats.MissingFiles))

			if len(fileList) != (stats.VerifiedFiles + stats.SizeFailed + stats.ChecksumFailed + stats.MissingFiles) {
				return commandErrorf("File count does not match")
			}

			printer.DisplayMessage(printer.Info, "\nVerification summary:")
			printer.DisplayMessage(printer.Note, fmt.Sprintf("  Total files:     %d", stats.TotalFiles))
			printer.DisplayMessage(printer.Note, fmt.Sprintf("  Verified:        %d", stats.VerifiedFiles))
			printer.DisplayMessage(printer.Note, fmt.Sprintf("  Size errors:     %d", stats.SizeFailed))
			printer.DisplayMessage(printer.Note, fmt.Sprintf("  Checksum errors: %d", stats.ChecksumFailed))
			printer.DisplayMessage(printer.Note, fmt.Sprintf("  Missing files:   %d", stats.MissingFiles))

			if stats.SizeFailed > 0 || stats.ChecksumFailed > 0 || stats.MissingFiles > 0 {
				return commandErrorf("Some files failed verification")
			}

			printer.DisplayMessage(printer.Info, "Success!")
			return nil
		},
	}
	cmd.Flags().IntP("recid", "r", 0, "Record ID (exact match)")
	cmd.Flags().StringP("doi", "d", "", "Digital Object Identifier (exact match)")
	cmd.Flags().StringP("title", "t", "", "Record title (exact match, no wildcards)")
	cmd.Flags().StringP("input-dir", "i", "", "Input directory containing files to verify")
	cmd.Flags().StringP("filter-name", "n", "", "Verify files matching exactly the file name")
	cmd.Flags().StringP("filter-regexp", "e", "", "Verify files matching the regular expression")
	cmd.Flags().StringP("server", "s", "", "Which CERN Open Data server to query? [default=http://opendata.cern.ch]")
	return cmd
}
