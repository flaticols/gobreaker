package gitrepo

import (
	"fmt"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/exp/apidiff"
	"golang.org/x/tools/go/packages"
)

type StatusError struct {
	Err    error
	Status string
}

func (err *StatusError) Error() string {
	return fmt.Sprintf("%v\ngit status:\n%v", err.Err, err.Status)
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

func getPackages(repoPath string, hash string, includeInternal bool) (map[string]*packages.Package, map[string]*packages.Package, error) {
	// Checkout the specific commit
	if _, err := runGitCommand(repoPath, "checkout", "--force", hash); err != nil {
		return nil, nil, fmt.Errorf("failed to checkout %s: %w", hash, err)
	}

	// Clean untracked files
	if _, err := runGitCommand(repoPath, "clean", "-fd"); err != nil {
		return nil, nil, fmt.Errorf("failed to clean working directory: %w", err)
	}

	// Hard reset to ensure clean state
	if _, err := runGitCommand(repoPath, "reset", "--hard", hash); err != nil {
		return nil, nil, fmt.Errorf("failed to reset to %s: %w", hash, err)
	}

	// Determine go module flags
	goFlags := "-mod=readonly"
	if st, err := os.Stat(filepath.Join(repoPath, "vendor")); err == nil && st.IsDir() {
		goFlags = "-mod=vendor"
	}

	// Load packages
	cfg := packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedTypes | packages.NeedTypesSizes,
		Tests:      false,
		BuildFlags: []string{goFlags},
		Dir:        repoPath,
	}
	pkgs, err := packages.Load(&cfg, "./...")
	if err != nil {
		return nil, nil, err
	}

	selfPkgs := make(map[string]*packages.Package)
	importPkgs := make(map[string]*packages.Package)
	for _, pkg := range pkgs {
		// by default and in must-cases, we must skip internal packages; however, in same cases it might be useful
		if !includeInternal && (strings.HasSuffix(pkg.PkgPath, "/internal") || strings.Contains(pkg.PkgPath, "/internal/")) {
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

	// Ensure we're still at the right commit after package loading
	if _, err := runGitCommand(repoPath, "reset", "--hard", hash); err != nil {
		return nil, nil, fmt.Errorf("failed to hard reset to %v: %w", hash, err)
	}

	return selfPkgs, importPkgs, nil
}
