package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/flaticols/gobreaker/pkg/breaking"
)

// OpenRepo compares API differences between two commits in a Git repository.
// It returns a Diff report with details on compatibility and breaking changes.
func OpenRepo(repoPath, oldCommit, newCommit string) (*breaking.Diff, error) {
	// Ensure we're in a git repository
	if _, err := runGitCommand(repoPath, "rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("not a git repository: %s", repoPath)
	}

	// Check if working tree is clean
	status, err := runGitCommand(repoPath, "status", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("failed to get git status: %w", err)
	}
	if status != "" {
		return nil, &StatusError{Status: status, Err: fmt.Errorf("current git tree is dirty")}
	}

	// Get current HEAD ref
	origRef, err := runGitCommand(repoPath, "symbolic-ref", "HEAD")
	if err != nil {
		// If not on a branch, get the commit hash
		origRef, err = runGitCommand(repoPath, "rev-parse", "HEAD")
		if err != nil {
			return nil, fmt.Errorf("failed to get current HEAD reference: %w", err)
		}
	}
	origRef = strings.TrimSpace(origRef)

	// Resolve commit hashes
	oldHash, err := runGitCommand(repoPath, "rev-parse", oldCommit)
	if err != nil {
		return nil, fmt.Errorf("could not resolve commit %q: %w", oldCommit, err)
	}
	oldHash = strings.TrimSpace(oldHash)

	newHash, err := runGitCommand(repoPath, "rev-parse", newCommit)
	if err != nil {
		return nil, fmt.Errorf("could not resolve commit %q: %w", newCommit, err)
	}
	newHash = strings.TrimSpace(newHash)

	// Ensure we restore original state on exit
	defer func() {
		var checkoutErr error
		if strings.HasPrefix(origRef, "refs/") {
			// Checkout branch
			_, checkoutErr = runGitCommand(repoPath, "checkout", origRef)
		} else {
			// Checkout detached HEAD
			_, checkoutErr = runGitCommand(repoPath, "checkout", origRef)
		}
		if checkoutErr != nil {
			fmt.Printf("WARNING: failed to checkout your original working commit after diff: %v\n", checkoutErr)
		}
	}()

	// Get packages from old commit
	selfOld, importsOld, err := getPackages(repoPath, oldHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get packages from old commit %q (%s): %w", oldCommit, oldHash, err)
	}

	// Get packages from new commit
	selfNew, importsNew, err := getPackages(repoPath, newHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get packages from new commit %q (%s): %w", newCommit, newHash, err)
	}

	// Compare packages
	apiReports, incompatible := comparePackages(selfOld, selfNew)
	apiImports, breakingImports := compareImports(importsOld, importsNew)

	d := breaking.New(apiReports, apiImports)
	d.SetBreakingImports(breakingImports)
	d.SetIncompatible(incompatible)

	return d, nil
}

// runGitCommand executes a git command in the specified directory
func runGitCommand(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w\nstderr: %s", strings.Join(args, " "), err, stderr.String())
	}
	
	return stdout.String(), nil
}