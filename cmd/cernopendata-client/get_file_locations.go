package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cernopendata/cernopendata-client-go/internal/config"
	"github.com/cernopendata/cernopendata-client-go/internal/printer"
	"github.com/cernopendata/cernopendata-client-go/internal/searcher"
)

func newGetFileLocationsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-file-locations",
		Short: "Get a list of data file locations of a record",
		Long: `Get a list of data file locations of a record.

Select a CERN Open Data bibliographic record by a record ID, a DOI, or a
title and return the list of data file locations belonging to this record.

Examples:

     $ cernopendata-client get-file-locations --recid 5500

     $ cernopendata-client get-file-locations --recid 5500 --protocol xrootd

     $ cernopendata-client get-file-locations --recid 5500 --verbose

     $ cernopendata-client get-file-locations --recid 8886 --file-availability online

     $ cernopendata-client get-file-locations --recid 5500 --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			recid, err := cmd.Flags().GetInt("recid")
			if err != nil {
				return commandErrorf("Invalid recid: %w", err)
			}
			doi, _ := cmd.Flags().GetString("doi")
			title, _ := cmd.Flags().GetString("title")
			protocol, _ := cmd.Flags().GetString("protocol")
			expand, _ := cmd.Flags().GetBool("expand")
			noExpand, _ := cmd.Flags().GetBool("no-expand")
			verbose, _ := cmd.Flags().GetBool("verbose")
			fileAvailability, _ := cmd.Flags().GetString("file-availability")
			server, _ := cmd.Flags().GetString("server")
			outputFormat, _ := cmd.Flags().GetString("format")

			if fileAvailability != "" && fileAvailability != "online" && fileAvailability != "all" {
				return commandErrorf("Invalid file availability: %s (choose from 'online', 'all')", fileAvailability)
			}

			if outputFormat != "text" && outputFormat != "json" {
				return commandErrorf("Invalid format: %s (choose from 'text', 'json')", outputFormat)
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

			client := searcher.NewClient(server)
			record, err := client.GetRecord(parsedRecid)
			if err != nil {
				return commandErrorf("Failed to get record: %w", err)
			}

			files, err := client.GetFilesList(record, protocol, expand)
			if err != nil {
				return commandErrorf("Failed to list files: %w", err)
			}

			if expand {
				filteredFiles, hasOfflineWarning := searcher.FilterFilesByAvailability(files, fileAvailability)
				if hasOfflineWarning && fileAvailability == "" {
					printer.DisplayMessage(printer.Warning, "WARNING: Some files in the list are not online and may not be downloadable.")
					printer.DisplayMessage(printer.Warning, "To list only online files, use the '--file-availability online' option.")
				}
				files = filteredFiles
			}

			if outputFormat == "json" {
				type FileOutput struct {
					URI          string `json:"uri"`
					Size         int64  `json:"size,omitempty"`
					Checksum     string `json:"checksum,omitempty"`
					Availability string `json:"availability,omitempty"`
				}

				var output []FileOutput
				for _, file := range files {
					output = append(output, FileOutput{
						URI:          file.URI,
						Size:         file.Size,
						Checksum:     file.Checksum,
						Availability: file.Availability,
					})
				}

				jsonBytes, err := json.MarshalIndent(output, "", "  ")
				if err != nil {
					return commandErrorf("Failed to marshal JSON: %w", err)
				}
				printer.DisplayOutput(string(jsonBytes))
				return nil
			}

			for _, file := range files {
				if verbose {
					printer.DisplayOutput(fmt.Sprintf("%s\t%d\t%s\t%s", file.URI, file.Size, file.Checksum, file.Availability))
				} else {
					printer.DisplayOutput(file.URI)
				}
			}
			return nil
		},
	}
	cmd.Flags().IntP("recid", "i", 0, "Record ID (exact match)")
	cmd.Flags().StringP("doi", "D", "", "Digital Object Identifier (exact match)")
	cmd.Flags().StringP("title", "T", "", "Record title (exact match, no wildcards)")
	cmd.Flags().StringP("protocol", "p", "http", "Protocol to be used in links [http,xrootd]")
	cmd.Flags().BoolP("expand", "e", true, "Expand file indexes?")
	cmd.Flags().Bool("no-expand", false, "Don't expand file indexes")
	cmd.Flags().BoolP("verbose", "V", false, "Output also the file size (2nd), checksum (3rd), and availability (4th)")
	cmd.Flags().StringP("file-availability", "", "", "Filter files by their availability status [online, all]")
	cmd.Flags().StringP("server", "S", "", "Which CERN Open Data server to query? [default=http://opendata.cern.ch]")
	cmd.Flags().StringP("format", "m", "text", "Output format (text|json)")
	return cmd
}
