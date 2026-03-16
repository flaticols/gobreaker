// Package gobreaker detects breaking changes in Go APIs.
//
// It wraps [golang.org/x/exp/apidiff] to compare two versions of Go packages
// and report incompatible (breaking) and compatible API changes. Versions can
// be specified as git refs or as filesystem paths.
//
// # Comparison modes
//
// Git ref mode compares commits, tags, or branches within a repository.
// Temporary worktrees are created for the requested refs so the working
// directory is never modified. When newRef is empty the current working
// directory (including uncommitted changes) is used as the target:
//
//	// Compare a tag against the working directory.
//	report, err := gobreaker.CompareRefs("/path/to/repo", "v1.2.0", "", false)
//
//	// Compare two tags.
//	report, err := gobreaker.CompareRefs("/path/to/repo", "v1.2.0", "v1.3.0", false)
//
// Filesystem mode compares two directories that each contain Go packages.
// No git operations are performed:
//
//	report, err := gobreaker.CompareFilesystems("/old/pkg", "/new/pkg", false)
//
// # Interpreting results
//
// [Report.IsCompatible] returns false when any breaking change is found.
// [Report.HasChanges] returns true when any change (breaking or not) is found.
// Together they cover the three semver cases:
//
//	report, err := gobreaker.CompareRefs(repo, oldTag, newTag, false)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	switch {
//	case !report.IsCompatible():
//	    // Breaking change → bump major.
//	case report.HasChanges():
//	    // Backward-compatible change → bump minor.
//	default:
//	    // No API change → bump patch.
//	}
//
// # Internal packages
//
// By default internal packages (paths containing "/internal/") are excluded
// from analysis because they are not part of the public API. Pass
// includeInternal=true to include them.
//
// # Human-readable output
//
// [Report.WriteText] writes the report to any [io.Writer] and [Report.String]
// returns it as a string:
//
//	fmt.Print(report)           // uses String() via the fmt.Stringer interface
//	report.WriteText(os.Stderr) // write to a specific writer
package gobreaker

import (
	"bytes"
	"io"

	"github.com/flaticols/gobreaker/internal/git"
	"github.com/flaticols/gobreaker/pkg/breaking"
)

// Report holds the results of an API compatibility comparison.
//
// A Report is returned by [CompareRefs] and [CompareFilesystems].
// Use [Report.IsCompatible] and [Report.HasChanges] to inspect the
// outcome programmatically, or [Report.WriteText] / [Report.String]
// for human-readable output.
type Report struct {
	diff *breaking.Diff
}

// IsCompatible returns true when no breaking (incompatible) API changes
// were detected. A compatible addition of new exported symbols does not
// affect this result.
func (r *Report) IsCompatible() bool {
	return r.diff.IsCompatible()
}

// HasChanges returns true when any API change was detected, whether
// compatible or incompatible. When both IsCompatible and HasChanges
// return true the change set contains only backward-compatible additions.
func (r *Report) HasChanges() bool {
	return r.diff.HasChanges()
}

// WriteText writes the human-readable diff report to w.
// Each package with changes is listed with its incompatible and compatible
// changes indented below.
func (r *Report) WriteText(w io.Writer) error {
	return r.diff.WriteText(w)
}

// String returns the human-readable diff report as a string.
// It is equivalent to calling [Report.WriteText] into a buffer.
func (r *Report) String() string {
	var buf bytes.Buffer
	_ = r.diff.WriteText(&buf)
	return buf.String()
}

// CompareFilesystems compares the public Go API of packages rooted at
// oldPath against those rooted at newPath. Both paths must be directories
// containing Go source files. No git operations are performed.
//
// Set includeInternal to true to include packages whose import path
// contains "/internal/".
func CompareFilesystems(oldPath, newPath string, includeInternal bool) (*Report, error) {
	diff, err := git.CompareFilesystems(oldPath, newPath, includeInternal)
	if err != nil {
		return nil, err
	}
	return &Report{diff: diff}, nil
}

// CompareRefs compares the public Go API at two git refs (branches, tags,
// or commit hashes) within the repository at repoPath.
//
// Each ref is checked out into a temporary worktree; the caller's working
// directory is never modified. If newRef is the empty string the comparison
// target is the current working directory, which may include uncommitted
// changes.
//
// Set includeInternal to true to include packages whose import path
// contains "/internal/".
func CompareRefs(repoPath, oldRef, newRef string, includeInternal bool) (*Report, error) {
	diff, err := git.OpenRepo(repoPath, oldRef, newRef, includeInternal)
	if err != nil {
		return nil, err
	}
	return &Report{diff: diff}, nil
}

// DetectDefaultBranch returns the default branch name for the git
// repository at repoPath. It checks, in order:
//
//  1. The symbolic ref refs/remotes/origin/HEAD (set by git clone).
//  2. A local branch named "main".
//  3. A local branch named "master".
//
// An error is returned when none of the above resolve.
func DetectDefaultBranch(repoPath string) (string, error) {
	return git.DetectDefaultBranch(repoPath)
}
