package git

import (
	"fmt"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"golang.org/x/exp/apidiff"
	"golang.org/x/tools/go/packages"
)

type StatusError struct {
	Stat git.Status
	Err  error
}

func (err *StatusError) Error() string {
	return fmt.Sprintf("%v\n%v", err.Err, err.Stat)
}

func comparePackages(oldPkgs, newPkgs map[string]*packages.Package) (map[string]apidiff.Report, bool) {
	reports, incompatible := compareImports(oldPkgs, newPkgs)
	for k := range newPkgs {
		delete(oldPkgs, k)
	}

	for k, oldPackage := range oldPkgs {
		report := apidiff.Changes(oldPackage.Types, types.NewPackage(k, oldPackage.Name))
		for _, c := range report.Changes {
			if !c.Compatible {
				incompatible = true
			}
		}
		reports[k] = report
	}
	return reports, incompatible
}

func compareImports(oldPkgs, newPkgs map[string]*packages.Package) (map[string]apidiff.Report, bool) {
	reports := map[string]apidiff.Report{}
	incompatible := false
	for k, newPackage := range newPkgs {
		oldPackage, ok := oldPkgs[k]
		if !ok {
			oldPackage = &packages.Package{Types: types.NewPackage(newPackage.PkgPath, newPackage.Name)}
		}

		report := apidiff.Changes(oldPackage.Types, newPackage.Types)
		for _, c := range report.Changes {
			if !c.Compatible {
				incompatible = true
			}
		}
		reports[k] = report
	}
	return reports, incompatible
}

func getHashes(repo *git.Repository, oldRev, newRev plumbing.Revision) (*plumbing.Hash, *plumbing.Hash, error) {
	oldCommitHash, err := repo.ResolveRevision(oldRev)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get hash for %q: %v", oldRev, err)
	}

	newCommitHash, err := repo.ResolveRevision(newRev)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get hash for %q: %v", newRev, err)
	}

	return oldCommitHash, newCommitHash, nil
}

func getPackages(wt git.Worktree, hash plumbing.Hash) (map[string]*packages.Package, map[string]*packages.Package, error) {
	if err := wt.Checkout(&git.CheckoutOptions{Hash: hash, Force: true}); err != nil {
		return nil, nil, err
	}
	if err := wt.Clean(&git.CleanOptions{Dir: true}); err != nil {
		return nil, nil, err
	}
	if err := wt.Reset(&git.ResetOptions{Commit: hash, Mode: git.HardReset}); err != nil {
		return nil, nil, err
	}

	goFlags := "-mod=readonly"
	if st, err := os.Stat(filepath.Join(wt.Filesystem.Root(), "vendor")); err == nil && st.IsDir() {
		goFlags = "-mod=vendor"
	}
	cfg := packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedTypes | packages.NeedTypesSizes,
		Tests:      false,
		BuildFlags: []string{goFlags},
	}
	pkgs, err := packages.Load(&cfg, "./...")
	if err != nil {
		return nil, nil, err
	}

	selfPkgs := make(map[string]*packages.Package)
	importPkgs := make(map[string]*packages.Package)
	for _, pkg := range pkgs {
		// skip internal packages since they do not contain public APIs
		if strings.HasSuffix(pkg.PkgPath, "/internal") || strings.Contains(pkg.PkgPath, "/internal/") {
			continue
		}
		selfPkgs[pkg.PkgPath] = pkg
	}
	for _, pkg := range pkgs {
		for _, ipkg := range pkg.Imports {
			if _, ok := selfPkgs[ipkg.PkgPath]; !ok {
				importPkgs[ipkg.PkgPath] = ipkg
			}
		}
	}

	if err := wt.Reset(&git.ResetOptions{
		Mode:   git.HardReset,
		Commit: hash,
	}); err != nil {
		return nil, nil, fmt.Errorf("failed to hard reset to %v: %w", hash, err)
	}

	return selfPkgs, importPkgs, nil
}

func checkoutRef(wt git.Worktree, ref plumbing.Reference) (err error) {
	if ref.Name() == "HEAD" {
		return wt.Checkout(&git.CheckoutOptions{Hash: ref.Hash()})
	}
	return wt.Checkout(&git.CheckoutOptions{Branch: ref.Name()})
}
