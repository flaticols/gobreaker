package git

import (
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/exp/apidiff"
	"golang.org/x/tools/go/packages"
)

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

// getPackagesFromPath loads Go packages from a filesystem directory.
func getPackagesFromPath(dir string, includeInternal bool) (selfPkgs, importPkgs map[string]*packages.Package, err error) {
	goFlags := "-mod=readonly"
	if st, err := os.Stat(filepath.Join(dir, "vendor")); err == nil && st.IsDir() {
		goFlags = "-mod=vendor"
	}

	cfg := packages.Config{
		Dir: dir,
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedTypes | packages.NeedTypesSizes,
		Tests:      false,
		BuildFlags: []string{goFlags},
	}
	pkgs, err := packages.Load(&cfg, "./...")
	if err != nil {
		return nil, nil, err
	}

	selfPkgs = make(map[string]*packages.Package)
	importPkgs = make(map[string]*packages.Package)
	for _, pkg := range pkgs {
		if !includeInternal {
			if strings.HasSuffix(pkg.PkgPath, "/internal") || strings.Contains(pkg.PkgPath, "/internal/") {
				continue
			}
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

	return selfPkgs, importPkgs, nil
}
