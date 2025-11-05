package git

import (
	"fmt"

	"github.com/flaticols/gobreaker/pkg/breaking"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

// OpenRepo compares API differences between two commits in a Git repository.
// It returns a Diff report with details on compatibility and breaking changes.
// If includeInternal is false, internal packages are excluded from analysis.
func OpenRepo(repoPath, oldCommit, newCommit string, includeInternal bool) (*breaking.Diff, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository at %s: %w", repoPath, err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree: %w", err)
	}

	wt.Filesystem = osfs.New(repoPath)
	rootFS := osfs.New("/")

	globalIgnoreFile, err := gitignore.LoadGlobalPatterns(rootFS)
	if err != nil {
		return nil, fmt.Errorf("failed to load gitignore: %v", err)
	}
	wt.Excludes = append(wt.Excludes, globalIgnoreFile...)

	sysIgnoreFile, err := gitignore.LoadSystemPatterns(rootFS)
	if err != nil {
		return nil, fmt.Errorf("failed to load system gitignore: %v", err)
	}
	wt.Excludes = append(wt.Excludes, sysIgnoreFile...)

	if stat, err := wt.Status(); err != nil {
		return nil, fmt.Errorf("failed to get git status: %w", err)
	} else if !stat.IsClean() {
		return nil, &StatusError{stat, fmt.Errorf("current git tree is dirty")}
	}

	origRef, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get current HEAD reference: %w", err)
	}

	oldHash, newHash, err := getHashes(repo, plumbing.Revision(oldCommit), plumbing.Revision(newCommit))
	if err != nil {
		return nil, fmt.Errorf("failed to lookup git commit hashes: %w", err)
	}

	defer func() {
		if err := checkoutRef(*wt, *origRef); err != nil {
			fmt.Printf("WARNING: failed to checkout your original working commit after diff: %v\n", err)
		}
	}()

	selfOld, importsOld, err := getPackages(*wt, *oldHash, includeInternal)
	if err != nil {
		return nil, fmt.Errorf("failed to get packages from old commit %q (%s): %w", oldCommit, oldHash, err)
	}

	selfNew, importsNew, err := getPackages(*wt, *newHash, includeInternal)
	if err != nil {
		return nil, fmt.Errorf("failed to get packages from new commit %q (%s): %w", newCommit, newHash, err)
	}

	apiReports, incompatible := comparePackages(selfOld, selfNew)
	apiImports, breakingImports := compareImports(importsOld, importsNew)

	d := breaking.New(apiReports, apiImports)
	d.SetBreakingImports(breakingImports)
	d.SetIncompatible(incompatible)

	return d, nil
}
