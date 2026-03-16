---
name: check
description: >
  Detect breaking changes in a Go project's public API.
  Use when: checking API compatibility before release, reviewing PRs for breaking changes,
  comparing two git refs or tags, or auditing Go API modifications.
  Triggers on: "check breaking changes", "API compatibility", "is this a breaking change",
  "compare API", "gobreaker".
allowed-tools: Bash(gobreaker *), Bash(go install *), Bash(which gobreaker), Read, Grep, Glob
argument-hint: "[old-ref] [new-ref]  — defaults to default-branch vs working dir"
---

# Check Go API for Breaking Changes

Detect breaking changes in the current Go project using `gobreaker`.

## Prerequisites

Ensure gobreaker is installed:

```bash
which gobreaker || go install github.com/flaticols/gobreaker/cmd/gobreaker@latest
```

## Analysis

Run gobreaker from the repository root. Use `$ARGUMENTS` if the user provided refs,
otherwise default to comparing the default branch against the working directory:

```bash
cd $(git rev-parse --show-toplevel)
gobreaker $ARGUMENTS
```

If the user specified `--path` or `-p`, use filesystem mode:
```bash
gobreaker -p $ARGUMENTS
```

## Interpreting results

Based on the gobreaker output, provide a structured summary:

1. **Status**: COMPATIBLE or BREAKING
2. **Breaking changes** (if any): list each with the affected symbol and what changed
3. **Compatible changes** (if any): new additions that don't break existing code
4. **Semver recommendation**: major/minor/patch bump based on the changes
5. **Migration hints**: for each breaking change, suggest how callers should adapt

If gobreaker exits with code 1, breaking changes were found — emphasize this clearly.
If no changes are detected, confirm the API is stable.
