# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

gobreaker is a Go tool that detects breaking changes in Go APIs using `golang.org/x/exp/apidiff`. It compares different versions of Go packages and reports incompatible API changes to help maintain backward compatibility and follow semantic versioning.

## Key Architecture

The project uses `golang.org/x/exp/apidiff` as its core dependency for API comparison. The codebase follows a clean architecture pattern:

- **CLI Entry Point**: `cmd/gobreaker/main.go` uses `jessevdk/go-flags` for command parsing
  - Accepts old/new git references, repository path, and output format options
  - Returns exit code 1 when breaking changes are detected
- **Breaking Change Detection**: `pkg/breaking/` contains the core logic
  - `diff.go`: Manages API diff reports, tracks changes, and determines compatibility
  - `report.go`: Formats and outputs diff reports with proper indentation
- **Git Integration**: `internal/git/` handles repository operations
  - `git.go`: Opens repositories and compares two git references
  - `status.go`: Ensures clean working tree before operations
  - Uses `go-git` library for all git operations

## Common Commands

```bash
# Build the tool
go build -o gobreaker ./cmd/gobreaker

# Run tests
go test ./...

# Basic usage - compare current against main branch
./gobreaker main

# Compare specific versions with JSON output
./gobreaker -o json v1.0.0 HEAD

# Compare in a different repository
./gobreaker main /path/to/repo

# Install dependencies
go mod download

# Update dependencies
go mod tidy
```

## Development Guidelines

When implementing features:
1. CLI logic and command parsing should be added to `cmd/gobreaker/main.go` using the existing `go-flags` parser
2. Breaking change detection logic belongs in `pkg/breaking/`
3. Git operations should be implemented in `internal/git/` using the `go-git` library
4. The tool leverages `apidiff.Report` for compatibility analysis and distinguishes between self packages and imported packages

The implementation includes:
- Custom `TextDiffReport` writer that adds proper indentation to apidiff output
- Error wrapping with context for better debugging
- Deferred cleanup to restore original git state after operations
- Support for text, JSON, and markdown output formats
- Skips internal packages from breaking change analysis