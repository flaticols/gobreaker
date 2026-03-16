package git

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/flaticols/gobreaker/pkg/breaking"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// CompareFilesystems compares API differences between two filesystem directories.
// No git operations are performed — pure filesystem comparison.
func CompareFilesystems(oldPath, newPath string, includeInternal bool) (*breaking.Diff, error) {
	selfOld, importsOld, err := getPackagesFromPath(oldPath, includeInternal)
	if err != nil {
		return nil, fmt.Errorf("failed to get packages from %s: %w", oldPath, err)
	}

	selfNew, importsNew, err := getPackagesFromPath(newPath, includeInternal)
	if err != nil {
		return nil, fmt.Errorf("failed to get packages from %s: %w", newPath, err)
	}

	apiReports, incompatible := comparePackages(selfOld, selfNew)
	apiImports, breakingImports := compareImports(importsOld, importsNew)

	d := breaking.New(apiReports, apiImports)
	d.SetBreakingImports(breakingImports)
	d.SetIncompatible(incompatible)

	return d, nil
}

// DetectDefaultBranch attempts to detect the default branch of a git repository.
func DetectDefaultBranch(repoPath string) (string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to open repository at %s: %w", repoPath, err)
	}

	// Try refs/remotes/origin/HEAD
	ref, err := repo.Reference(plumbing.ReferenceName("refs/remotes/origin/HEAD"), false)
	if err == nil && ref.Type() == plumbing.SymbolicReference {
		target := ref.Target().Short()
		if branch, ok := strings.CutPrefix(target, "origin/"); ok {
			return branch, nil
		}
		return target, nil
	}

	// Fallback: try main, then master
	for _, branch := range []string{"main", "master"} {
		_, err := repo.Reference(plumbing.ReferenceName("refs/heads/"+branch), false)
		if err == nil {
			return branch, nil
		}
	}

	return "", fmt.Errorf("cannot detect default branch")
}

// createTempWorktree creates a temporary git worktree checked out at the given ref.
func createTempWorktree(repoPath, ref string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "gobreaker-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	cmd := exec.Command("git", "-C", repoPath, "worktree", "add", "--detach", tmpDir, ref)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("failed to create worktree for %q: %s: %w", ref, strings.TrimSpace(string(out)), err)
	}

	return tmpDir, nil
}

// removeTempWorktree removes a temporary git worktree.
func removeTempWorktree(repoPath, worktreePath string) {
	exec.Command("git", "-C", repoPath, "worktree", "remove", "--force", worktreePath).Run()
	os.RemoveAll(worktreePath)
}

// OpenRepo compares API differences between git refs in a repository.
// If newRef is empty, compares oldRef against the current working directory.
func OpenRepo(repoPath, oldRef, newRef string, includeInternal bool) (*breaking.Diff, error) {
	oldDir, err := createTempWorktree(repoPath, oldRef)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare old ref: %w", err)
	}
	defer removeTempWorktree(repoPath, oldDir)

	newDir := repoPath
	if newRef != "" {
		newDir, err = createTempWorktree(repoPath, newRef)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare new ref: %w", err)
		}
		defer removeTempWorktree(repoPath, newDir)
	}

	return CompareFilesystems(oldDir, newDir, includeInternal)
}
