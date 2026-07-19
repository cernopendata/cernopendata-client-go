package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cernopendata/cernopendata-client-go/internal/printer"
	"github.com/cernopendata/cernopendata-client-go/internal/updater"
	"github.com/cernopendata/cernopendata-client-go/internal/version"
)

var checkOnly bool

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check for and install updates",
	Long: `Check for available updates and optionally install them.

By default, this command will download and install the latest version.
Use --check to only check for updates without installing.

If the binary was installed via Homebrew, the command will suggest using
'brew upgrade' instead of performing a self-update.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runUpdate()
	},
}

func init() {
	updateCmd.Flags().BoolVar(&checkOnly, "check", false, "Only check for updates, don't install")
}

func runUpdate() error {
	currentVersion := version.Version

	printer.DisplayMessage(printer.Info, fmt.Sprintf("Current version: %s", currentVersion))
	printer.DisplayMessage(printer.Info, "Checking for updates...")

	release, err := updater.CheckForUpdate()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	latestVersion := release.TagName
	cmp := updater.CompareVersions(currentVersion, latestVersion)

	if cmp >= 0 {
		printer.DisplayMessage(printer.Info, fmt.Sprintf("You are running the latest version (%s)", currentVersion))
		return nil
	}

	printer.DisplayMessage(printer.Info, fmt.Sprintf("New version available: %s", latestVersion))

	if checkOnly {
		printer.DisplayMessage(printer.Note, "Run 'cernopendata-client update' to install the update.")
		return nil
	}

	if updater.IsHomebrewInstall() {
		printer.DisplayMessage(printer.Info, "")
		printer.DisplayMessage(printer.Info, "This binary appears to be installed via Homebrew.")
		printer.DisplayMessage(printer.Note, "Please update using: brew upgrade cernopendata-client-go")
		return nil
	}

	binaryURL, checksumURL, err := updater.GetAssetForCurrentPlatform(release)
	if err != nil {
		return fmt.Errorf("failed to find binary for your platform: %w", err)
	}

	urlParts := strings.Split(binaryURL, "/")
	assetName := urlParts[len(urlParts)-1]

	printer.DisplayMessage(printer.Info, fmt.Sprintf("Downloading %s...", assetName))
	printer.DisplayMessage(printer.Info, "Verifying checksum...")

	binary, err := updater.DownloadVerifiedBinary(binaryURL, checksumURL, assetName, func(downloaded, total int64) {
		percent := float64(downloaded) / float64(total) * 100
		fmt.Fprintf(os.Stderr, "\rDownloading... %.1f%%", percent)
	})
	if err != nil {
		return fmt.Errorf("failed to download and verify update: %w", err)
	}
	fmt.Fprintln(os.Stderr)
	printer.DisplayMessage(printer.Info, "Checksum verified.")

	printer.DisplayMessage(printer.Info, "Installing update...")
	if err := updater.ReplaceBinary(binary); err != nil {
		return fmt.Errorf("failed to install update: %w", err)
	}

	printer.DisplayMessage(printer.Info, fmt.Sprintf("Successfully updated to %s", latestVersion))
	return nil
}
