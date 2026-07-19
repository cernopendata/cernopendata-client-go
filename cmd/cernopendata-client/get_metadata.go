package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/cernopendata/cernopendata-client-go/internal/config"
	"github.com/cernopendata/cernopendata-client-go/internal/metadater"
	"github.com/cernopendata/cernopendata-client-go/internal/printer"
	"github.com/cernopendata/cernopendata-client-go/internal/searcher"
)

var getMetadataCmd = &cobra.Command{
	Use:   "get-metadata",
	Short: "Get metadata content of a record",
	Long: `Get metadata content of a record.

Select a CERN Open Data bibliographic record by a record ID, a
DOI, or a title and return its metadata in the JSON format.

Examples:

     $ cernopendata-client get-metadata --recid 1

     $ cernopendata-client get-metadata --recid 1 --output-value title

     $ cernopendata-client get-metadata --recid 329 --output-value authors.orcid --filter name="Rousseau, David"`,
	Run: func(cmd *cobra.Command, args []string) {
		recid, err := cmd.Flags().GetInt("recid")
		if err != nil {
			printer.DisplayMessage(printer.Error, fmt.Sprintf("Invalid recid: %v", err))
			os.Exit(1)
		}
		doi, _ := cmd.Flags().GetString("doi")
		title, _ := cmd.Flags().GetString("title")
		outputValue, _ := cmd.Flags().GetString("output-value")
		filterStr, _ := cmd.Flags().GetString("filter")
		outputFormat, _ := cmd.Flags().GetString("format")
		server, _ := cmd.Flags().GetString("server")

		if server == "" {
			server = config.ServerHTTPURI
		}

		if filterStr != "" && outputValue == "" {
			printer.DisplayMessage(printer.Error, "--filter can only be used with --output-value")
			os.Exit(1)
		}

		parsedRecid, err := searcher.GetRecid(server, doi, title, recid)
		if err != nil {
			printer.DisplayMessage(printer.Error, fmt.Sprintf("Failed to find record: %v", err))
			os.Exit(1)
		}

		client := searcher.NewClient(server)
		record, err := client.GetRecord(parsedRecid)
		if err != nil {
			printer.DisplayMessage(printer.Error, fmt.Sprintf("Failed to get metadata: %v", err))
			os.Exit(1)
		}

		var filters []string
		if filterStr != "" {
			filters = []string{filterStr}
		}

		if outputValue == "" {
			output, err := metadater.FormatOutput(record.Metadata, outputFormat)
			if err != nil {
				printer.DisplayMessage(printer.Error, fmt.Sprintf("Failed to format output: %v", err))
				os.Exit(1)
			}
			printer.DisplayOutput(output)
		} else {
			metadata, err := metadater.GetNestedField(record.Metadata, outputValue)
			if err != nil {
				printer.DisplayMessage(printer.Error, fmt.Sprintf("Field not found: %v", err))
				os.Exit(1)
			}

			if len(filters) > 0 {
				items, isArray := metadata.([]any)
				if !isArray {
					items = []any{metadata}
				}
				filtered, err := metadater.FilterArray(items, filters)
				if err != nil {
					printer.DisplayMessage(printer.Error, fmt.Sprintf("Filter error: %v", err))
					os.Exit(1)
				}
				if len(filtered) > 0 {
					output, err := metadater.FormatOutput(filtered[0], outputFormat)
					if err != nil {
						printer.DisplayMessage(printer.Error, fmt.Sprintf("Failed to format output: %v", err))
						os.Exit(1)
					}
					printer.DisplayOutput(output)
				}
			} else {
				output, err := metadater.FormatOutput(metadata, outputFormat)
				if err != nil {
					printer.DisplayMessage(printer.Error, fmt.Sprintf("Failed to format output: %v", err))
					os.Exit(1)
				}
				printer.DisplayOutput(output)
			}
		}
	},
}

func init() {
	getMetadataCmd.Flags().IntP("recid", "r", 0, "Record ID (exact match)")
	getMetadataCmd.Flags().StringP("doi", "d", "", "Digital Object Identifier (exact match)")
	getMetadataCmd.Flags().StringP("title", "t", "", "Record title (exact match, no wildcards)")
	getMetadataCmd.Flags().StringP("output-value", "v", "", "Output value of only desired metadata field [example=title]")
	getMetadataCmd.Flags().StringP("filter", "f", "", "Filter only certain output values matching filtering criteria. [Use --filter some_field_name=some_value]")
	getMetadataCmd.Flags().StringP("format", "m", "pretty", "Output format (pretty|json)")
	getMetadataCmd.Flags().StringP("server", "s", "", "Which CERN Open Data server to query? [default=http://opendata.cern.ch]")
}
