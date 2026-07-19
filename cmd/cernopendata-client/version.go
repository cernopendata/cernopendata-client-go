package main

import (
	"github.com/spf13/cobra"

	"github.com/cernopendata/cernopendata-client-go/internal/printer"
	"github.com/cernopendata/cernopendata-client-go/internal/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Return version",
	Run: func(cmd *cobra.Command, args []string) {
		printer.DisplayOutput(version.Version)
	},
}
