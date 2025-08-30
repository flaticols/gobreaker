package gitrepo

import (
	"fmt"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

type RefComparator struct {
	repo *git.Repository
}

func NewRefComparator(repoPath string) (*RefComparator, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, err
	}

	return &RefComparator{repo: repo}, nil
}

func (rc *RefComparator) GetRefWorktree(ref string) (*git.Worktree, error) {
	hash, err := rc.repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return nil, err
	}

	fs := memfs.New()

	// Actually create a proper worktree
	wt := git.Worktree{
		Filesystem: fs,
		Repository: rc.repo,
	}

	err = wt.Checkout(&git.CheckoutOptions{
		Hash: *hash,
	})

	return &wt, err
}
