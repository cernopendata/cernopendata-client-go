package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cernopendata/cernopendata-client-go/internal/config"
	"github.com/cernopendata/cernopendata-client-go/internal/metadater"
	"github.com/cernopendata/cernopendata-client-go/internal/printer"
	"github.com/cernopendata/cernopendata-client-go/internal/searcher"
)

func newGetMetadataCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-metadata",
		Short: "Get metadata content of a record",
		Long: `Get metadata content of a record.

Select a CERN Open Data bibliographic record by a record ID, a
DOI, or a title and return its metadata in the JSON format.

Examples:

     $ cernopendata-client get-metadata --recid 1

     $ cernopendata-client get-metadata --recid 1 --output-value title

     $ cernopendata-client get-metadata --recid 329 --output-value authors.orcid --filter name="Rousseau, David"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			recid, err := cmd.Flags().GetInt("recid")
			if err != nil {
				return commandErrorf("Invalid recid: %w", err)
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
				return fmt.Errorf("--filter can only be used with --output-value")
			}

			parsedRecid, err := searcher.GetRecid(server, doi, title, recid)
			if err != nil {
				return commandErrorf("Failed to find record: %w", err)
			}

			client := searcher.NewClient(server)
			record, err := client.GetRecord(parsedRecid)
			if err != nil {
				return commandErrorf("Failed to get metadata: %w", err)
			}

			var filters []string
			if filterStr != "" {
				filters = []string{filterStr}
			}

			if outputValue == "" {
				output, err := metadater.FormatOutput(record.Metadata, outputFormat)
				if err != nil {
					return commandErrorf("Failed to format output: %w", err)
				}
				printer.DisplayOutput(output)
			} else {
				metadata, err := metadater.GetNestedField(record.Metadata, outputValue)
				if err != nil {
					return commandErrorf("Field not found: %w", err)
				}

				if len(filters) > 0 {
					items, isArray := metadata.([]any)
					if !isArray {
						items = []any{metadata}
					}
					filtered, err := metadater.FilterArray(items, filters)
					if err != nil {
						return commandErrorf("Filter error: %w", err)
					}
					if len(filtered) > 0 {
						output, err := metadater.FormatOutput(filtered[0], outputFormat)
						if err != nil {
							return commandErrorf("Failed to format output: %w", err)
						}
						printer.DisplayOutput(output)
					}
				} else {
					output, err := metadater.FormatOutput(metadata, outputFormat)
					if err != nil {
						return commandErrorf("Failed to format output: %w", err)
					}
					printer.DisplayOutput(output)
				}
			}
			return nil
		},
	}
	cmd.Flags().IntP("recid", "r", 0, "Record ID (exact match)")
	cmd.Flags().StringP("doi", "d", "", "Digital Object Identifier (exact match)")
	cmd.Flags().StringP("title", "t", "", "Record title (exact match, no wildcards)")
	cmd.Flags().StringP("output-value", "v", "", "Output value of only desired metadata field [example=title]")
	cmd.Flags().StringP("filter", "f", "", "Filter only certain output values matching filtering criteria. [Use --filter some_field_name=some_value]")
	cmd.Flags().StringP("format", "m", "pretty", "Output format (pretty|json)")
	cmd.Flags().StringP("server", "s", "", "Which CERN Open Data server to query? [default=http://opendata.cern.ch]")
	return cmd
}
