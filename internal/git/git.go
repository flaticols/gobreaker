package git

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/flaticols/gobreaker/pkg/breaking"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// CompareFilesystems compares API differences between two filesystem directories.
// It returns a Diff report with details on compatibility and breaking changes.
// If includeInternal is false, internal packages are excluded from analysis.
func CompareFilesystems(oldPath, newPath string, includeInternal bool) (*breaking.Diff, error) {
	// Verify paths exist
	if _, err := os.Stat(oldPath); err != nil {
		return nil, fmt.Errorf("old path %q does not exist: %w", oldPath, err)
	}
	if _, err := os.Stat(newPath); err != nil {
		return nil, fmt.Errorf("new path %q does not exist: %w", newPath, err)
	}

	selfOld, importsOld, err := getPackagesFromPath(oldPath, includeInternal)
	if err != nil {
		return nil, fmt.Errorf("failed to get packages from old path %q: %w", oldPath, err)
	}

	selfNew, importsNew, err := getPackagesFromPath(newPath, includeInternal)
	if err != nil {
		return nil, fmt.Errorf("failed to get packages from new path %q: %w", newPath, err)
	}

	apiReports, incompatible := comparePackages(selfOld, selfNew)
	apiImports, breakingImports := compareImports(importsOld, importsNew)

	d := breaking.New(apiReports, apiImports)
	d.SetBreakingImports(breakingImports)
	d.SetIncompatible(incompatible)

	return d, nil
}

// OpenRepo compares API differences between two commits in a Git repository.
// It uses temporary clones to avoid modifying the current branch.
// It returns a Diff report with details on compatibility and breaking changes.
// If includeInternal is false, internal packages are excluded from analysis.
func OpenRepo(repoPath, oldCommit, newCommit string, includeInternal bool) (*breaking.Diff, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository at %s: %w", repoPath, err)
	}

	oldHash, newHash, err := getHashes(repo, plumbing.Revision(oldCommit), plumbing.Revision(newCommit))
	if err != nil {
		return nil, fmt.Errorf("failed to lookup git commit hashes: %w", err)
	}

	// Create temporary directories for analysis
	oldWorktreePath, err := os.MkdirTemp("", "gobreaker-old-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory for old worktree: %w", err)
	}
	defer os.RemoveAll(oldWorktreePath)

	newWorktreePath, err := os.MkdirTemp("", "gobreaker-new-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory for new worktree: %w", err)
	}
	defer os.RemoveAll(newWorktreePath)

	// Clone repository to temp directories and checkout specific commits
	oldRepo, err := git.PlainClone(oldWorktreePath, false, &git.CloneOptions{
		URL: repoPath,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to clone for old commit: %w", err)
	}

	oldWt, err := oldRepo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get old worktree: %w", err)
	}

	if err := oldWt.Checkout(&git.CheckoutOptions{Hash: *oldHash}); err != nil {
		return nil, fmt.Errorf("failed to checkout old commit %s: %w", oldHash, err)
	}

	newRepo, err := git.PlainClone(newWorktreePath, false, &git.CloneOptions{
		URL: repoPath,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to clone for new commit: %w", err)
	}

	newWt, err := newRepo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get new worktree: %w", err)
	}

	if err := newWt.Checkout(&git.CheckoutOptions{Hash: *newHash}); err != nil {
		return nil, fmt.Errorf("failed to checkout new commit %s: %w", newHash, err)
	}

	selfOld, importsOld, err := getPackagesFromPath(oldWorktreePath, includeInternal)
	if err != nil {
		return nil, fmt.Errorf("failed to get packages from old commit %q (%s): %w", oldCommit, oldHash, err)
	}

	selfNew, importsNew, err := getPackagesFromPath(newWorktreePath, includeInternal)
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

// IsGitRef checks if a string is a git reference by attempting to resolve it.
func IsGitRef(repoPath, ref string) bool {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return false
	}

	_, err = repo.ResolveRevision(plumbing.Revision(ref))
	return err == nil
}

// IsFilesystemPath checks if a string is a valid filesystem path.
func IsFilesystemPath(path string) bool {
	// Check if it's an absolute path or if it exists relative to current directory
	if filepath.IsAbs(path) {
		_, err := os.Stat(path)
		return err == nil
	}

	// Check relative path
	_, err := os.Stat(path)
	return err == nil
}
