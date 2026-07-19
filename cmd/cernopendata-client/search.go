package main

import (
	"fmt"
	"maps"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cernopendata/cernopendata-client-go/internal/config"
	"github.com/cernopendata/cernopendata-client-go/internal/metadater"
	"github.com/cernopendata/cernopendata-client-go/internal/printer"
	"github.com/cernopendata/cernopendata-client-go/internal/searcher"
	"github.com/cernopendata/cernopendata-client-go/internal/utils"
)

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search CERN Open Data records",
	Long: `Search CERN Open Data records with flexible query options.

Search the CERN Open Data portal using free text queries, facet filters,
or by copying URLs directly from the web portal.

Search patterns support AND, OR operators and field-specific queries.
See https://opendata.cern.ch/docs/cod-search-tips for syntax details.

Examples:

     $ cernopendata-client search --query-pattern "Higgs"

     $ cernopendata-client search --query-pattern "muon" --query-facet experiment=CMS

     $ cernopendata-client search --query "q=online&f=experiment%3ACMS"

     $ cernopendata-client search --query-pattern "title.tokens:*muon*" --output-value title

     $ cernopendata-client search --query-pattern "Higgs" --size -1`,
	Run: func(cmd *cobra.Command, args []string) {
		query, _ := cmd.Flags().GetString("query")
		queryPattern, _ := cmd.Flags().GetString("query-pattern")
		queryFacets, _ := cmd.Flags().GetStringArray("query-facet")
		outputValue, _ := cmd.Flags().GetString("output-value")
		filterStr, _ := cmd.Flags().GetString("filter")
		outputFormat, _ := cmd.Flags().GetString("format")
		server, _ := cmd.Flags().GetString("server")
		page, _ := cmd.Flags().GetInt("page")
		size, _ := cmd.Flags().GetInt("size")
		sort, _ := cmd.Flags().GetString("sort")
		listFacets, _ := cmd.Flags().GetBool("list-facets")

		if server == "" {
			server = config.ServerHTTPURI
		}

		client := searcher.NewClient(server)

		// Handle --list-facets
		if listFacets {
			facets, err := client.GetFacets()
			if err != nil {
				printer.DisplayMessage(printer.Error, fmt.Sprintf("Failed to fetch facets: %v", err))
				os.Exit(1)
			}

			printer.DisplayMessage(printer.Info, "Available facets for --query-facet:\n")
			for name, agg := range facets {
				if len(agg.Buckets) == 0 {
					continue
				}
				printer.DisplayOutput(fmt.Sprintf("%s:", name))
				for _, bucket := range agg.Buckets {
					keyStr := fmt.Sprintf("%v", bucket.Key)
					printer.DisplayOutput(fmt.Sprintf("  - %s (%d)", keyStr, bucket.DocCount))
				}
				printer.DisplayOutput("")
			}
			return
		}

		if filterStr != "" && outputValue == "" {
			printer.DisplayMessage(printer.Error, "--filter can only be used with --output-value")
			os.Exit(1)
		}

		// Build query from parameters
		facetsMap := make(map[string]string)

		// Parse --query URL/query string if provided
		if query != "" {
			parsedQuery, err := utils.ParseQueryFromURL(query)
			if err != nil {
				printer.DisplayMessage(printer.Error, fmt.Sprintf("Failed to parse query: %v", err))
				os.Exit(1)
			}
			// Use parsed values as defaults
			if queryPattern == "" && parsedQuery.Q != "" {
				queryPattern = parsedQuery.Q
			}
			maps.Copy(facetsMap, parsedQuery.Facets)
			if parsedQuery.Page != nil && !cmd.Flags().Changed("page") {
				page = *parsedQuery.Page
			}
			if parsedQuery.Size != nil && !cmd.Flags().Changed("size") {
				size = *parsedQuery.Size
			}
			if parsedQuery.Sort != "" && sort == "" {
				sort = parsedQuery.Sort
			}
		}

		// Parse --query-facet flags (override parsed facets)
		for _, qf := range queryFacets {
			parts := strings.SplitN(qf, "=", 2)
			if len(parts) != 2 {
				printer.DisplayMessage(printer.Error, fmt.Sprintf("Invalid facet format: %s (expected key=value)", qf))
				os.Exit(1)
			}
			facetsMap[parts[0]] = parts[1]
		}

		var searchResp *searcher.SearchResponse
		var err error

		// Handle --size -1 for fetching all results
		if size == -1 {
			searchResp, err = client.SearchAllRecords(queryPattern, facetsMap, sort)
		} else {
			searchResp, err = client.SearchRecords(queryPattern, facetsMap, page, size, sort)
		}

		if err != nil {
			printer.DisplayMessage(printer.Error, fmt.Sprintf("Search failed: %v", err))
			os.Exit(1)
		}

		if searchResp.Hits.Total == 0 {
			printer.DisplayMessage(printer.Info, "No records found.")
			return
		}

		var filters []string
		if filterStr != "" {
			filters = []string{filterStr}
		}

		// Output handling
		if outputValue == "" {
			// Default: print record titles
			for _, hit := range searchResp.Hits.Hits {
				if title, ok := hit.Metadata["title"].(string); ok {
					printer.DisplayOutput(title)
				} else {
					printer.DisplayOutput(fmt.Sprintf("Record %s", hit.ID))
				}
			}
			// Show total count if there are more results
			displayed := len(searchResp.Hits.Hits)
			if searchResp.Hits.Total > displayed {
				printer.DisplayMessage(printer.Info, fmt.Sprintf("\nShowing %d of %d total records. Use --size -1 to fetch all.", displayed, searchResp.Hits.Total))
			} else {
				printer.DisplayMessage(printer.Info, fmt.Sprintf("\nTotal: %d records", searchResp.Hits.Total))
			}
		} else {
			// Extract specific field from each record
			var results []any
			for _, hit := range searchResp.Hits.Hits {
				value, err := metadater.ExtractNestedField(hit.Metadata, outputValue)
				if err == nil && value != nil {
					results = append(results, value)
				}
			}

			if len(filters) > 0 {
				filtered, err := metadater.FilterArray(results, filters)
				if err != nil {
					printer.DisplayMessage(printer.Error, fmt.Sprintf("Filter error: %v", err))
					os.Exit(1)
				}
				results = filtered
			}

			if outputFormat == "json" {
				output, err := metadater.FormatOutput(results, outputFormat)
				if err != nil {
					printer.DisplayMessage(printer.Error, fmt.Sprintf("Failed to format output: %v", err))
					os.Exit(1)
				}
				printer.DisplayOutput(output)
			} else {
				// Pretty format: print each result on a line
				for _, result := range results {
					printer.DisplayOutput(fmt.Sprintf("%v", result))
				}
			}
		}
	},
}

func init() {
	searchCmd.Flags().StringP("query", "q", "", "Full URL or query string from CERN Open Data portal")
	searchCmd.Flags().String("query-pattern", "", "Free text search pattern (see https://opendata.cern.ch/docs/cod-search-tips)")
	searchCmd.Flags().StringArrayP("query-facet", "f", []string{}, "Facet filter in key=value format (can be repeated)")
	searchCmd.Flags().StringP("output-value", "o", "", "Extract specific metadata field from results")
	searchCmd.Flags().String("filter", "", "Filter array results (requires --output-value)")
	searchCmd.Flags().StringP("format", "m", "pretty", "Output format (pretty|json)")
	searchCmd.Flags().StringP("server", "s", "", "CERN Open Data server URL [default=http://opendata.cern.ch]")
	searchCmd.Flags().IntP("page", "p", 1, "Page number")
	searchCmd.Flags().Int("size", 10, "Page size (-1 for all results)")
	searchCmd.Flags().String("sort", "", "Sort order")
	searchCmd.Flags().Bool("list-facets", false, "List available facets for filtering")
}
