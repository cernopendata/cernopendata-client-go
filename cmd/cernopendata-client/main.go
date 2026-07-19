package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/cernopendata/cernopendata-client-go/internal/printer"
	"github.com/cernopendata/cernopendata-client-go/internal/version"
)

var buildVersion = "dev"

func init() {
	if buildVersion != "dev" {
		version.Version = buildVersion
	}
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "cernopendata-client",
		Short: "CLI for CERN Open Data portal",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				_ = cmd.Help()
				return nil
			}
			return fmt.Errorf("unknown command: %s", args[0])
		},
	}

	var completionCmd = &cobra.Command{
		Use:   "completion",
		Short: "Generate shell completion script",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				printer.DisplayMessage(printer.Error, "Please specify bash or zsh")
				os.Exit(1)
			}

			shell := args[0]
			switch shell {
			case "bash":
				_ = rootCmd.GenBashCompletion(os.Stdout)
			case "zsh":
				_ = rootCmd.GenZshCompletion(os.Stdout)
			default:
				printer.DisplayMessage(printer.Error, fmt.Sprintf("Unsupported shell: %s (supported: bash, zsh)", shell))
				os.Exit(1)
			}
		},
	}

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(getMetadataCmd)
	rootCmd.AddCommand(getFileLocationsCmd)
	rootCmd.AddCommand(downloadFilesCmd)
	rootCmd.AddCommand(verifyFilesCmd)
	rootCmd.AddCommand(listDirectoryCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(completionCmd)

	if err := rootCmd.Execute(); err != nil {
		printer.DisplayMessage(printer.Error, fmt.Sprintf("Error: %v", err))
		os.Exit(1)
	}
}
