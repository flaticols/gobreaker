# gobreaker

A command-line tool that detects breaking changes in Go APIs by comparing different versions of your code.

## Overview

gobreaker analyzes your Go packages and reports incompatible API changes between versions, helping you maintain backward compatibility and follow semantic versioning principles. It uses `golang.org/x/exp/apidiff` under the hood to perform accurate API comparisons.

## Installation

```bash
go install github.com/flaticols/gobreaker/cmd/gobreaker@latest
```

Or build from source:

```bash
git clone https://github.com/flaticols/gobreaker.git
cd gobreaker
go build -o gobreaker ./cmd/gobreaker
```

## Usage

```bash
gobreaker [OPTIONS] <old-ref> [new-ref]
```

### Arguments

- `old-ref` (required): Old reference (branch, tag, or commit) to compare from
- `new-ref` (optional): New reference (branch, tag, or commit) to compare to (default: HEAD)

### Options

- `-r, --repo <path>`: Path to git repository (default: current directory)
- `-f, --format <format>`: Output format - `text` (default), `json`, or `markdown`
- `-i, --include-internal`: Include internal packages in API analysis
- `-q, --quiet`: Suppress output
- `-v, --version`: Print version information and exit
- `-h, --help`: Show help message

### Examples

```bash
# Compare HEAD against main branch (skips internal packages by default)
gobreaker main

# Compare main branch against develop branch
gobreaker main develop

# Compare HEAD against main and include internal packages
gobreaker main --include-internal

# Compare specific commits
gobreaker abc123 def456

# Compare in a different repository
gobreaker main --repo /path/to/repo

# Compare with a different repository and specific commits
gobreaker abc123 def456 --repo /path/to/repo

# Output results as JSON
gobreaker main --format json

# Output results as Markdown (useful for PR comments)
gobreaker main --format markdown

# Check version
gobreaker --version
```

## Output Formats

### Text (default)
Shows a human-readable report of breaking changes with proper indentation.

### JSON
Outputs structured JSON data for programmatic consumption.

### Markdown
Generates a Markdown-formatted report suitable for documentation or pull request comments.

## What It Detects

gobreaker identifies various types of breaking changes including:

- Removed or renamed exported functions, types, methods, or variables
- Changed function signatures
- Removed or changed struct fields
- Interface method changes
- Type definition changes

**Note on Internal Packages:** By default, gobreaker skips internal packages (those with `/internal/` in their path). This is because internal packages are implementation details not meant to be used outside the module. However, if you want to track breaking changes in internal package public APIs (useful for maintaining internal API stability), use the `--include-internal` flag. When this flag is enabled, gobreaker analyzes **only the exported (public) APIs** of internal packages, just as it does for regular packages.

## Development

### Building

```bash
go build -o gobreaker ./cmd/gobreaker
```

### Running Tests

```bash
go test ./...
```

## License

See [LICENSE](LICENSE) file for details.
