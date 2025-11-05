package git

import (
	"go/types"
	"testing"

	"golang.org/x/exp/apidiff"
	"golang.org/x/tools/go/packages"
)

// TestComparePackages_InternalPackageWithPublicAPI tests that internal packages
// with exported (public) APIs are correctly analyzed for breaking changes.
func TestComparePackages_InternalPackageWithPublicAPI(t *testing.T) {
	// Create an old version of an internal package with a public function
	oldPkg := types.NewPackage("github.com/example/internal/utils", "utils")
	oldScope := oldPkg.Scope()

	// Add an exported function to the old package
	sig := types.NewSignatureType(nil, nil, nil, nil, nil, false)
	oldFunc := types.NewFunc(0, oldPkg, "PublicFunc", sig)
	oldScope.Insert(oldFunc)

	oldPkgs := map[string]*packages.Package{
		"github.com/example/internal/utils": {
			PkgPath: "github.com/example/internal/utils",
			Name:    "utils",
			Types:   oldPkg,
		},
	}

	// Create a new version where the public function is removed (breaking change)
	newPkgs := map[string]*packages.Package{}

	reports, incompatible := comparePackages(oldPkgs, newPkgs)

	if !incompatible {
		t.Error("Expected incompatible=true when public API is removed from internal package")
	}

	report, ok := reports["github.com/example/internal/utils"]
	if !ok {
		t.Fatal("Expected report for internal/utils package")
	}

	if len(report.Changes) == 0 {
		t.Error("Expected changes to be detected for internal package with public API")
	}

	// Verify that at least one change is incompatible
	foundIncompatible := false
	for _, change := range report.Changes {
		if !change.Compatible {
			foundIncompatible = true
			break
		}
	}

	if !foundIncompatible {
		t.Error("Expected at least one incompatible change when removing public API")
	}
}

// TestComparePackages_InternalPackagePathDetection tests that packages with
// "/internal/" in their path are included in the analysis.
func TestComparePackages_InternalPackagePathDetection(t *testing.T) {
	testCases := []struct {
		name    string
		pkgPath string
	}{
		{
			name:    "internal at end",
			pkgPath: "github.com/example/internal",
		},
		{
			name:    "internal in middle",
			pkgPath: "github.com/example/internal/utils",
		},
		{
			name:    "internal with subpackage",
			pkgPath: "github.com/example/internal/deep/nested/pkg",
		},
		{
			name:    "regular package",
			pkgPath: "github.com/example/pkg",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create old package with exported function
			oldPkg := types.NewPackage(tc.pkgPath, "testpkg")
			sig := types.NewSignatureType(nil, nil, nil, nil, nil, false)
			oldFunc := types.NewFunc(0, oldPkg, "ExportedFunc", sig)
			oldPkg.Scope().Insert(oldFunc)

			oldPkgs := map[string]*packages.Package{
				tc.pkgPath: {
					PkgPath: tc.pkgPath,
					Name:    "testpkg",
					Types:   oldPkg,
				},
			}

			// Create new package without the function (breaking change)
			newPkgs := map[string]*packages.Package{}

			reports, incompatible := comparePackages(oldPkgs, newPkgs)

			if !incompatible {
				t.Errorf("Expected incompatible=true for package %s", tc.pkgPath)
			}

			if _, ok := reports[tc.pkgPath]; !ok {
				t.Errorf("Expected report for package %s to be included", tc.pkgPath)
			}
		})
	}
}

// TestCompareImports_InternalPackageChanges tests that changes to internal
// packages that are imported are detected.
func TestCompareImports_InternalPackageChanges(t *testing.T) {
	// Create old version of internal package
	oldPkg := types.NewPackage("github.com/example/internal/api", "api")
	sig := types.NewSignatureType(nil, nil, nil, nil, nil, false)
	oldFunc := types.NewFunc(0, oldPkg, "DoSomething", sig)
	oldPkg.Scope().Insert(oldFunc)

	oldPkgs := map[string]*packages.Package{
		"github.com/example/internal/api": {
			PkgPath: "github.com/example/internal/api",
			Name:    "api",
			Types:   oldPkg,
		},
	}

	// Create new version with modified signature (breaking change)
	newPkg := types.NewPackage("github.com/example/internal/api", "api")
	// Create a different signature (e.g., adds a parameter)
	params := types.NewTuple(types.NewVar(0, newPkg, "x", types.Typ[types.Int]))
	newSig := types.NewSignatureType(nil, nil, nil, params, nil, false)
	newFunc := types.NewFunc(0, newPkg, "DoSomething", newSig)
	newPkg.Scope().Insert(newFunc)

	newPkgs := map[string]*packages.Package{
		"github.com/example/internal/api": {
			PkgPath: "github.com/example/internal/api",
			Name:    "api",
			Types:   newPkg,
		},
	}

	reports, incompatible := compareImports(oldPkgs, newPkgs)

	if !incompatible {
		t.Error("Expected incompatible=true when internal package API changes")
	}

	report, ok := reports["github.com/example/internal/api"]
	if !ok {
		t.Fatal("Expected report for internal package")
	}

	if len(report.Changes) == 0 {
		t.Error("Expected changes to be detected in internal package")
	}
}

// TestComparePackages_PrivateAPINotDetected tests that unexported (private)
// identifiers are not included in the diff reports.
func TestComparePackages_PrivateAPINotDetected(t *testing.T) {
	// Create old package with private function
	oldPkg := types.NewPackage("github.com/example/pkg", "pkg")
	sig := types.NewSignatureType(nil, nil, nil, nil, nil, false)
	// Note: lowercase 'privateFunc' means it's unexported
	oldFunc := types.NewFunc(0, oldPkg, "privateFunc", sig)
	oldPkg.Scope().Insert(oldFunc)

	oldPkgs := map[string]*packages.Package{
		"github.com/example/pkg": {
			PkgPath: "github.com/example/pkg",
			Name:    "pkg",
			Types:   oldPkg,
		},
	}

	// Create new package without the private function
	newPkg := types.NewPackage("github.com/example/pkg", "pkg")
	newPkgs := map[string]*packages.Package{
		"github.com/example/pkg": {
			PkgPath: "github.com/example/pkg",
			Name:    "pkg",
			Types:   newPkg,
		},
	}

	reports, incompatible := compareImports(oldPkgs, newPkgs)

	// Removing a private function should NOT be considered incompatible
	// because apidiff only looks at exported APIs
	if incompatible {
		t.Error("Expected incompatible=false when only private API is removed")
	}

	report := reports["github.com/example/pkg"]
	if len(report.Changes) > 0 {
		t.Errorf("Expected no changes for private API, got %d changes", len(report.Changes))
	}
}

// TestComparePackages_CompatibleChanges tests that compatible changes are
// correctly identified.
func TestComparePackages_CompatibleChanges(t *testing.T) {
	// Create old package with one exported function
	oldPkg := types.NewPackage("github.com/example/pkg", "pkg")
	sig := types.NewSignatureType(nil, nil, nil, nil, nil, false)
	oldFunc := types.NewFunc(0, oldPkg, "ExistingFunc", sig)
	oldPkg.Scope().Insert(oldFunc)

	oldPkgs := map[string]*packages.Package{
		"github.com/example/pkg": {
			PkgPath: "github.com/example/pkg",
			Name:    "pkg",
			Types:   oldPkg,
		},
	}

	// Create new package with the old function plus a new one (compatible change)
	newPkg := types.NewPackage("github.com/example/pkg", "pkg")
	newFunc1 := types.NewFunc(0, newPkg, "ExistingFunc", sig)
	newFunc2 := types.NewFunc(0, newPkg, "NewFunc", sig)
	newPkg.Scope().Insert(newFunc1)
	newPkg.Scope().Insert(newFunc2)

	newPkgs := map[string]*packages.Package{
		"github.com/example/pkg": {
			PkgPath: "github.com/example/pkg",
			Name:    "pkg",
			Types:   newPkg,
		},
	}

	_, incompatible := compareImports(oldPkgs, newPkgs)

	// Adding a new function is compatible
	if incompatible {
		t.Error("Expected incompatible=false when adding new exported function (compatible change)")
	}
}

// TestComparePackages_EmptyPackages tests the edge case of empty packages.
func TestComparePackages_EmptyPackages(t *testing.T) {
	oldPkgs := map[string]*packages.Package{}
	newPkgs := map[string]*packages.Package{}

	reports, incompatible := comparePackages(oldPkgs, newPkgs)

	if incompatible {
		t.Error("Expected incompatible=false for empty packages")
	}

	if len(reports) != 0 {
		t.Errorf("Expected no reports for empty packages, got %d", len(reports))
	}
}

// TestApidiffOnlyAnalyzesExportedIdentifiers is a documentation test that
// verifies apidiff behavior with exported vs unexported identifiers.
func TestApidiffOnlyAnalyzesExportedIdentifiers(t *testing.T) {
	// Create package with both exported and unexported identifiers
	oldPkg := types.NewPackage("test/pkg", "pkg")
	sig := types.NewSignatureType(nil, nil, nil, nil, nil, false)

	// Add exported function
	exportedFunc := types.NewFunc(0, oldPkg, "ExportedFunc", sig)
	oldPkg.Scope().Insert(exportedFunc)

	// Add unexported function
	unexportedFunc := types.NewFunc(0, oldPkg, "unexportedFunc", sig)
	oldPkg.Scope().Insert(unexportedFunc)

	// Create new package with only the unexported function removed
	newPkg := types.NewPackage("test/pkg", "pkg")
	newExportedFunc := types.NewFunc(0, newPkg, "ExportedFunc", sig)
	newPkg.Scope().Insert(newExportedFunc)
	// Note: unexportedFunc is not added to newPkg

	// Use apidiff directly to verify it only reports on exported identifiers
	report := apidiff.Changes(oldPkg, newPkg)

	// Should have no changes because only unexported identifier was removed
	if len(report.Changes) > 0 {
		t.Errorf("Expected apidiff to ignore unexported identifiers, but got %d changes", len(report.Changes))
		for _, change := range report.Changes {
			t.Logf("Unexpected change: %s", change.Message)
		}
	}
}
