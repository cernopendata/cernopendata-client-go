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
	if exitCode := execute(newRootCommand()); exitCode != 0 {
		os.Exit(exitCode)
	}
}

func execute(rootCmd *cobra.Command) int {
	if err := rootCmd.Execute(); err != nil {
		printer.DisplayMessage(printer.Error, fmt.Sprintf("Error: %v", err))
		return 1
	}
	return 0
}

func commandErrorf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

func newRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "cernopendata-client",
		Short:         "CLI for CERN Open Data portal",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return fmt.Errorf("unknown command: %s", args[0])
		},
	}

	rootCmd.AddCommand(newVersionCommand())
	rootCmd.AddCommand(newUpdateCommand())
	rootCmd.AddCommand(newGetMetadataCommand())
	rootCmd.AddCommand(newGetFileLocationsCommand())
	rootCmd.AddCommand(newDownloadFilesCommand())
	rootCmd.AddCommand(newVerifyFilesCommand())
	rootCmd.AddCommand(newListDirectoryCommand())
	rootCmd.AddCommand(newSearchCommand())
	rootCmd.AddCommand(newCompletionCommand(rootCmd))

	return rootCmd
}

func newCompletionCommand(rootCmd *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "completion",
		Short: "Generate shell completion script",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return commandErrorf("Please specify bash or zsh")
			}

			shell := args[0]
			switch shell {
			case "bash":
				if err := rootCmd.GenBashCompletion(cmd.OutOrStdout()); err != nil {
					return commandErrorf("Failed to generate bash completion: %w", err)
				}
			case "zsh":
				if err := rootCmd.GenZshCompletion(cmd.OutOrStdout()); err != nil {
					return commandErrorf("Failed to generate zsh completion: %w", err)
				}
			default:
				return commandErrorf("Unsupported shell: %s (supported: bash, zsh)", shell)
			}
			return nil
		},
	}
}
