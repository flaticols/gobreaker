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
gobreaker [OPTIONS] <base-ref> [repo-path]
```

### Arguments

- `base-ref` (required): The base reference to compare against (branch, tag, or commit SHA)
- `repo-path` (optional): Path to the git repository (defaults to current directory)

### Options

- `-o, --output <format>`: Output format - `text` (default), `json`, or `markdown`
- `-v, --version`: Print version information and exit
- `-h, --help`: Show help message

### Examples

```bash
# Compare current branch against main in current directory
gobreaker main

# Compare against a specific tag in a different repository
gobreaker v1.0.0 /path/to/repo

# Output results as JSON
gobreaker -o json main

# Output results as Markdown (useful for PR comments)
gobreaker --output markdown main

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
