# CERN Open Data Client - Go Implementation

A command-line tool to download and verify files from CERN Open Data portal, written in Go.

## Overview

This is a Go implementation of the [cernopendata-client](https://github.com/cernopendata/cernopendata-client) Python client.

## Flag Reference

Each command uses unique flag shorthands to avoid conflicts:

**get-metadata**:

- `-r` `--recid` - Record ID
- `-d` `--doi` - DOI
- `-t` `--title` - Title
- `-v` `--output-value` - Specific field value
- `-f` `--filter` - Filter field=value
- `-m` `--format` - Output format (pretty|json)
- `-s` `--server` - Server URI

**get-file-locations**:

- `-i` `--recid` - Record ID
- `-D` `--doi` - DOI
- `-T` `--title` - Title
- `-p` `--protocol` - Protocol (http|https)
- `-e` `--expand` - Expand file indices
- `-V` `--verbose` - Verbose output (includes size, checksum, and availability)
- `--file-availability` - Filter by availability (online|all)
- `-m` `--format` - Output format (text|json, default: text)
- `-S` `--server` - Server URI

**download-files**:

- `-R` `--recid` - Record ID
- `-d` `--doi` - DOI
- `-t` `--title` - Title
- `-O` `--output-dir` - Output directory (defaults to recid/)
- `-n` `--filter-name` - Glob pattern filter (comma-separated for multiple)
- `-e` `--filter-regexp` - Regex pattern filter
- `-r` `--filter-range` - Range filter (e.g., 1-2,5-7)
- `-y` `--retry-limit` - Retry attempts (default: 10)
- `-Y` `--retry-sleep` - Sleep duration between retries in seconds (default: 5)
- `-v` `--verbose` - Verbose output
- `-N` `--dry-run` - Dry run
- `-V` `--verify` - Verify files after download
- `--download-engine` - Download engine (http|xrootd, default: http)
- `-x` `--expand` - Expand file indices
- `--no-expand` - Don't expand file indices
- `-P` `--progress` - Show progress indicators
- `-p` `--protocol` - Protocol (http|xrootd)
- `--file-availability` - Filter by availability (online|all, default: skip tape files with warning)
- `-s` `--server` - Server URI

**verify-files**:

- `-r` `--recid` - Record ID
- `-d` `--doi` - DOI
- `-t` `--title` - Title
- `-i` `--input-dir` - Input directory (defaults to recid/)
- `-n` `--filter-name` - Glob pattern filter
- `-e` `--filter-regexp` - Regex pattern filter
- `-s` `--server` - Server URI

**list-directory**:

- `path` - XRootD path (positional argument)
- `-v` `--verbose` - Verbose output
- `-r` `--recursive` - List directories recursively
- `-t` `--timeout` - Timeout in seconds (default: 300)
- `-m` `--format` - Output format (text|json, default: text)

**completion**:

- `bash` - Generate bash completion
- `zsh` - Generate zsh completion

**update**:

- `--check` - Only check for updates, don't install

**search**:

- `-q` `--query` - Full URL or query string from portal
- `--query-pattern` - Free text search pattern
- `-f` `--query-facet` - Facet filter (key=value, repeatable)
- `-o` `--output-value` - Extract specific metadata field
- `--filter` - Filter array results
- `-m` `--format` - Output format (pretty|json)
- `-s` `--server` - Server URI
- `-p` `--page` - Page number (default: 1)
- `--size` - Page size (default: 10, -1 for all)
- `--sort` - Sort order
- `--list-facets` - List available facets for filtering

## Installation

### Requirements

- Go 1.26 or later
- For XRootD support (optional): `go-hep.org/x/hep/xrootd` (automatically downloaded)

### Option 1: Homebrew (macOS/Linux)

The easiest installation method on macOS or Linux:

```bash
brew tap clelange/particle-physics
brew install cernopendata-client
```

### Option 2: One-line install script (Linux/macOS)

The fastest way to install on Linux or macOS:

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/cernopendata/cernopendata-client-go/main/scripts/install.sh)"
```

This script will:

- Detect your OS and architecture
- Download latest release binary
- Verify checksum
- Install to `/usr/local/bin`, `~/bin`, or `~/.local/bin` (in this order, based on permissions)
- Automatically configure your PATH if needed

### Option 3: Building from Source

For Linux, macOS, or Windows:

```bash
# Clone the repository
git clone https://github.com/cernopendata/cernopendata-client-go.git
cd cernopendata-client-go

# Build the binary
go build -o cernopendata-client ./cmd/cernopendata-client

# Optional: Install to system path
mv cernopendata-client /usr/local/bin/
```

> [!IMPORTANT]
> Starting with v0.10.0, the Go module path is
> `github.com/cernopendata/cernopendata-client-go`. Commands such as `go install`
> must use the canonical CERN path; the former `github.com/clelange/...` path is
> retained only by historical pre-v0.10.0 releases.

### Shell Completion

Shell completion is included automatically with the Homebrew installation. For other installation methods:

```bash
# Generate bash completion script
cernopendata-client completion bash > /tmp/cernopendata-client.bash
source /tmp/cernopendata-client.bash

# Generate zsh completion script
cernopendata-client completion zsh > ~/.zsh/completions/_cernopendata-client

# Add to .zshrc for automatic loading
echo "fpath=(~/.zsh/completions \$fpath)" >> ~/.zshrc
echo "autoload -U compinit && compinit" >> ~/.zshrc
```

### XRootD Support

XRootD downloads are fully supported using the `--download-engine xrootd` flag.
The Go XRootD client (`go-hep.org/x/hep/xrootd`) is automatically downloaded when building the binary.

No additional system-level XRootD libraries are required - the implementation uses a pure Go XRootD client.

## Usage

### Version

```bash
cernopendata-client version
```

### Update

```bash
# Check for updates
cernopendata-client update --check

# Install updates
cernopendata-client update
```

**Note:** If you installed via Homebrew, use `brew upgrade cernopendata-client-go` instead.

The update command will:

- Check for the latest version on GitHub
- Download the appropriate binary for your platform
- Verify the SHA256 checksum
- Replace the current binary

### Get Metadata

```bash
# Get full metadata
cernopendata-client get-metadata --recid 3005

# Get specific field
cernopendata-client get-metadata --recid 3005 --output-value title

# Get metadata in JSON format
cernopendata-client get-metadata --recid 3005 --format json

# Filter metadata
cernopendata-client get-metadata --recid 3005 --filter type=Primary

# By DOI or title
cernopendata-client get-metadata --doi "10.7483/record/5500"
cernopendata-client get-metadata --title "CMS Open Data"
```

### Get File Locations

```bash
# List files
cernopendata-client get-file-locations --recid 5500

# Verbose output (includes size, checksum, and availability)
cernopendata-client get-file-locations --recid 5500 --verbose

# Use HTTPS protocol
cernopendata-client get-file-locations --recid 5500 --protocol https

# Expand file indices
cernopendata-client get-file-locations --recid 5500 --expand

# JSON output format
cernopendata-client get-file-locations --recid 5500 --format json

# Filter by file availability (online files only)
cernopendata-client get-file-locations --recid 8886 --file-availability online

# Include all files (online and on tape)
cernopendata-client get-file-locations --recid 8886 --file-availability all
```

### Download Files

```bash
# Download all files
cernopendata-client download-files --recid 5500

# Download to specific directory
cernopendata-client download-files --recid 5500 --output-dir data

# Filter by name pattern (glob)
cernopendata-client download-files --recid 5500 --filter-name "*.root"

# Filter by regex pattern
cernopendata-client download-files --recid 5500 --filter-regexp ".*\\.root$"

# Multiple name filters (comma-separated)
cernopendata-client download-files --recid 5500 --filter-name "*.root,*.txt"

# Multiple range filters
cernopendata-client download-files --recid 5500 --filter-range 1-5,10-15

# Dry run (don't actually download)
cernopendata-client download-files --recid 5500 --dry-run

# Configure retry attempts and sleep duration
cernopendata-client download-files --recid 5500 --retry-limit 5 --retry-sleep 2

# Verify files after download
cernopendata-client download-files --recid 5500 --verify

# Download using XRootD protocol
cernopendata-client download-files --recid 5500 --download-engine xrootd

# Expand file indices
cernopendata-client download-files --recid 5500 --expand

# Don't expand file indices
cernopendata-client download-files --recid 5500 --no-expand

# Show progress
cernopendata-client download-files --recid 5500 --progress

# Download only online files (skip tape-based files)
cernopendata-client download-files --recid 8886 --file-availability online

# Force download all files including those on tape (may fail if not staged)
cernopendata-client download-files --recid 8886 --file-availability all
```

**Duplicate File Names Note**: If a record contains multiple files with the same basename, the client preserves the shortest unique trailing directory structure under the output directory to avoid overwriting files. For example, files may be stored as `data/0002/AO2D.root` and `data/0003/AO2D.root`.

**File Availability Note**: By default, the client will warn you about files stored on tape and skip them automatically. You'll see:

- A warning message with a link to request file staging at the CERN Open Data portal
- A summary showing files downloaded and files skipped (on tape)
- Total bytes downloaded out of total bytes

Use `--file-availability online` to explicitly filter to online files only, or `--file-availability all` to force attempting to download all files (not recommended unless files have been staged).

### Verify Files

```bash
# Verify downloaded files
cernopendata-client verify-files --recid 5500 --input-dir data

# Verify only specific files by glob pattern
cernopendata-client verify-files --recid 5500 --input-dir data --filter-name "*.root"

# Verify only specific files by regex pattern
cernopendata-client verify-files --recid 5500 --input-dir data --filter-regexp ".*\\.root$"
```

### List Directory (XRootD)

```bash
# List XRootD directory
cernopendata-client list-directory /eos/opendata/cms

# Verbose output (includes size and modification time)
cernopendata-client list-directory /eos/opendata/cms --verbose

# List directory recursively
cernopendata-client list-directory /eos/opendata/cms --recursive

# List directory with custom timeout (seconds)
cernopendata-client list-directory /eos/opendata/cms --timeout 60

# JSON output format
cernopendata-client list-directory /eos/opendata/cms --format json

# Full XRootD URLs are also supported
cernopendata-client list-directory root://eospublic.cern.ch//eos/opendata/cms
```

### Search Records

```bash
# Basic search
cernopendata-client search --query-pattern "Higgs"

# Search with facet filter
cernopendata-client search --query-pattern "muon" --query-facet experiment=CMS

# Multiple facets
cernopendata-client search --query-pattern "electron" --query-facet experiment=CMS --query-facet type=Dataset

# Copy-paste URL from portal
cernopendata-client search --query "q=online&f=experiment%3ACMS"

# Extract specific field from results
cernopendata-client search --query-pattern "Higgs" --output-value title

# JSON output format
cernopendata-client search --query-pattern "Higgs" --output-value title --format json

# Fetch all results (batched)
cernopendata-client search --query-pattern "/TT*" --query-facet experiment=CMS --size -1

# Custom page size
cernopendata-client search --query-pattern "muon" --size 50

# Advanced search syntax (see https://opendata.cern.ch/docs/cod-search-tips)
cernopendata-client search --query-pattern "title.tokens:*muon*"
cernopendata-client search --query-pattern "doi:10.7483*"
cernopendata-client search --query-pattern "heavy ion -electron"

# Discover available facets
cernopendata-client search --list-facets
```

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run all tests (using make)
make test

# Run tests with coverage
go test ./... -cover

# Run tests for a specific package
go test ./internal/verifier

# Run integration tests (requires network access)
make test-integration
```

### Building

```bash
# Build for current platform
make build

# Or using go directly
go build -o bin/cernopendata-client ./cmd/cernopendata-client

# Build for different platforms
make build-all

# Build for a specific platform
GOOS=linux GOARCH=amd64 go build -o bin/cernopendata-client-linux ./cmd/cernopendata-client
GOOS=darwin GOARCH=amd64 go build -o bin/cernopendata-client-macos ./cmd/cernopendata-client
GOOS=windows GOARCH=amd64 go build -o bin/cernopendata-client.exe ./cmd/cernopendata-client
```

### Linting & Code Quality

```bash
# Run pre-commit hooks (recommended before committing)
pre-commit run --all-files

# Or use prek (faster drop-in replacement for pre-commit)
prek run --all-files

# Run golangci-lint
make lint

# Or run gofmt and go vet directly
gofmt -w .
go vet ./...
```

### Installing Pre-commit

```bash
# Install pre-commit
pip install pre-commit

# Or install prek (faster Rust-based alternative)
cargo install prek

# Setup git hooks
pre-commit install
# or
prek install
```

## Package Structure

```text
internal/
├── config/          # Configuration constants
├── searcher/        # Record search and API client
├── metadater/      # Metadata field extraction and formatting
├── checksum/        # ADLER32 checksum calculation
├── downloader/     # HTTP download engine with resume/retry
├── xrootddownloader/ # XRootD download engine with resume/retry
├── verifier/       # File integrity verification
├── lister/         # XRootD directory listing
├── validator/      # Input validation functions
├── utils/          # Utility functions (parsing, etc.)
├── printer/        # Output formatting and display
├── progress/       # Real-time progress tracking for downloads
├── updater/        # Update checking functionality
└── version/        # Version info
```

## Dependencies

- github.com/spf13/cobra v1.10.2 - CLI framework
- go-hep.org/x/hep v0.38.1 - XRootD protocol support

## License

This project is licensed under the GPLv3 license.

## Contributing

Contributions are welcome! Please ensure all tests pass before submitting a pull request.

```bash
go test ./...
```
