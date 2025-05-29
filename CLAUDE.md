# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

gobreaker is a Go tool that detects breaking changes in Go APIs using `golang.org/x/exp/apidiff`. It's designed to analyze Go packages and report incompatible changes between versions.

## Key Architecture

The project uses `golang.org/x/exp/apidiff` as its core dependency for API comparison. The codebase is structured as:

- **CLI Entry Point**: `cmd/gobreaker/main.go` uses `jessevdk/go-flags` for command parsing
- **Core Logic**: `pkg/breaking/` contains the breaking change detection logic
  - `diff.go`: Manages API diff reports and determines compatibility
  - `report.go`: Formats and outputs diff reports with proper indentation
- **Git Integration**: `internal/git/` (stub implementation) intended for repository operations using `go-git`

## Common Commands

```bash
# Build the tool
go build -o gobreaker ./cmd/gobreaker

# Run the tool
./gobreaker

# Install dependencies
go mod download

# Update dependencies
go mod tidy
```

## Development Guidelines

When implementing features:
1. The main CLI logic should be added to `cmd/gobreaker/main.go` using the existing `go-flags` parser
2. Breaking change detection logic belongs in `pkg/breaking/`
3. Git operations should be implemented in `internal/git/` using the already-imported `go-git` library
4. The tool should leverage `apidiff.Report` for compatibility analysis

The current implementation has:
- Basic diff reporting structure with support for both compatible and incompatible changes
- A custom writer (`diffReport`) that adds proper indentation to output
- Placeholder for git repository operations