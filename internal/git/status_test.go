package git

import (
	"go/types"
	"testing"

	"golang.org/x/exp/apidiff"
	"golang.org/x/tools/go/packages"
)

// Note: These tests focus on the comparePackages and compareImports functions
// which handle API diff analysis. The tests validate that when internal packages
// are included in the analysis (via the --include-internal flag), the tool
// correctly detects breaking changes in their public APIs. By default, internal
// packages are excluded unless the includeInternal parameter is true.

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

// TestComparePackages_StructTypeChanges tests detection of struct type changes.
func TestComparePackages_StructTypeChanges(t *testing.T) {
	// Create old package with a struct type
	oldPkg := types.NewPackage("github.com/example/internal/models", "models")
	fields := []*types.Var{
		types.NewField(0, oldPkg, "Name", types.Typ[types.String], false),
		types.NewField(0, oldPkg, "Age", types.Typ[types.Int], false),
	}
	oldStruct := types.NewStruct(fields, nil)
	oldType := types.NewNamed(types.NewTypeName(0, oldPkg, "User", nil), oldStruct, nil)
	oldPkg.Scope().Insert(oldType.Obj())

	oldPkgs := map[string]*packages.Package{
		"github.com/example/internal/models": {
			PkgPath: "github.com/example/internal/models",
			Name:    "models",
			Types:   oldPkg,
		},
	}

	// Create new package with modified struct (removed Age field - breaking change)
	newPkg := types.NewPackage("github.com/example/internal/models", "models")
	newFields := []*types.Var{
		types.NewField(0, newPkg, "Name", types.Typ[types.String], false),
	}
	newStruct := types.NewStruct(newFields, nil)
	newType := types.NewNamed(types.NewTypeName(0, newPkg, "User", nil), newStruct, nil)
	newPkg.Scope().Insert(newType.Obj())

	newPkgs := map[string]*packages.Package{
		"github.com/example/internal/models": {
			PkgPath: "github.com/example/internal/models",
			Name:    "models",
			Types:   newPkg,
		},
	}

	reports, incompatible := compareImports(oldPkgs, newPkgs)

	if !incompatible {
		t.Error("Expected incompatible=true when struct field is removed")
	}

	if len(reports["github.com/example/internal/models"].Changes) == 0 {
		t.Error("Expected changes to be detected for struct modification")
	}
}

// TestComparePackages_InterfaceChanges tests detection of interface changes.
func TestComparePackages_InterfaceChanges(t *testing.T) {
	// Create old package with an interface
	oldPkg := types.NewPackage("github.com/example/internal/api", "api")
	sig1 := types.NewSignatureType(nil, nil, nil, nil, nil, false)
	method1 := types.NewFunc(0, oldPkg, "Method1", sig1)

	oldInterface := types.NewInterfaceType([]*types.Func{method1}, nil)
	oldType := types.NewNamed(types.NewTypeName(0, oldPkg, "Interface", nil), oldInterface, nil)
	oldPkg.Scope().Insert(oldType.Obj())

	oldPkgs := map[string]*packages.Package{
		"github.com/example/internal/api": {
			PkgPath: "github.com/example/internal/api",
			Name:    "api",
			Types:   oldPkg,
		},
	}

	// Create new package with additional method in interface (breaking change for implementers)
	newPkg := types.NewPackage("github.com/example/internal/api", "api")
	sig2 := types.NewSignatureType(nil, nil, nil, nil, nil, false)
	newMethod1 := types.NewFunc(0, newPkg, "Method1", sig2)
	newMethod2 := types.NewFunc(0, newPkg, "Method2", sig2)

	newInterface := types.NewInterfaceType([]*types.Func{newMethod1, newMethod2}, nil)
	newType := types.NewNamed(types.NewTypeName(0, newPkg, "Interface", nil), newInterface, nil)
	newPkg.Scope().Insert(newType.Obj())

	newPkgs := map[string]*packages.Package{
		"github.com/example/internal/api": {
			PkgPath: "github.com/example/internal/api",
			Name:    "api",
			Types:   newPkg,
		},
	}

	reports, incompatible := compareImports(oldPkgs, newPkgs)

	if !incompatible {
		t.Error("Expected incompatible=true when interface method is added")
	}

	if len(reports["github.com/example/internal/api"].Changes) == 0 {
		t.Error("Expected changes to be detected for interface modification")
	}
}

// TestComparePackages_ConstantAndVariableChanges tests detection of const/var changes.
func TestComparePackages_ConstantAndVariableChanges(t *testing.T) {
	// Create old package with exported constant
	oldPkg := types.NewPackage("github.com/example/internal/config", "config")
	oldConst := types.NewConst(0, oldPkg, "MaxRetries", types.Typ[types.Int], nil)
	oldPkg.Scope().Insert(oldConst)

	oldPkgs := map[string]*packages.Package{
		"github.com/example/internal/config": {
			PkgPath: "github.com/example/internal/config",
			Name:    "config",
			Types:   oldPkg,
		},
	}

	// Create new package without the constant (breaking change)
	newPkg := types.NewPackage("github.com/example/internal/config", "config")
	newPkgs := map[string]*packages.Package{
		"github.com/example/internal/config": {
			PkgPath: "github.com/example/internal/config",
			Name:    "config",
			Types:   newPkg,
		},
	}

	reports, incompatible := compareImports(oldPkgs, newPkgs)

	if !incompatible {
		t.Error("Expected incompatible=true when exported constant is removed")
	}

	if len(reports["github.com/example/internal/config"].Changes) == 0 {
		t.Error("Expected changes to be detected for constant removal")
	}
}

// TestComparePackages_MethodChanges tests detection of method changes on types.
func TestComparePackages_MethodChanges(t *testing.T) {
	// Create old package with a type and method
	oldPkg := types.NewPackage("github.com/example/internal/service", "service")
	oldStruct := types.NewStruct(nil, nil)
	oldType := types.NewNamed(types.NewTypeName(0, oldPkg, "Service", nil), oldStruct, nil)

	// Add method to the type
	sig := types.NewSignatureType(types.NewVar(0, oldPkg, "", oldType), nil, nil, nil, nil, false)
	method := types.NewFunc(0, oldPkg, "Start", sig)
	oldType.AddMethod(method)

	oldPkg.Scope().Insert(oldType.Obj())

	oldPkgs := map[string]*packages.Package{
		"github.com/example/internal/service": {
			PkgPath: "github.com/example/internal/service",
			Name:    "service",
			Types:   oldPkg,
		},
	}

	// Create new package without the method (breaking change)
	newPkg := types.NewPackage("github.com/example/internal/service", "service")
	newStruct := types.NewStruct(nil, nil)
	newType := types.NewNamed(types.NewTypeName(0, newPkg, "Service", nil), newStruct, nil)
	newPkg.Scope().Insert(newType.Obj())

	newPkgs := map[string]*packages.Package{
		"github.com/example/internal/service": {
			PkgPath: "github.com/example/internal/service",
			Name:    "service",
			Types:   newPkg,
		},
	}

	reports, incompatible := compareImports(oldPkgs, newPkgs)

	if !incompatible {
		t.Error("Expected incompatible=true when method is removed from type")
	}

	if len(reports["github.com/example/internal/service"].Changes) == 0 {
		t.Error("Expected changes to be detected for method removal")
	}
}

// TestComparePackages_NewPackageAdded tests adding a new package (compatible change).
func TestComparePackages_NewPackageAdded(t *testing.T) {
	// Old packages - empty
	oldPkgs := map[string]*packages.Package{}

	// New package added
	newPkg := types.NewPackage("github.com/example/internal/newpkg", "newpkg")
	sig := types.NewSignatureType(nil, nil, nil, nil, nil, false)
	newFunc := types.NewFunc(0, newPkg, "NewFunction", sig)
	newPkg.Scope().Insert(newFunc)

	newPkgs := map[string]*packages.Package{
		"github.com/example/internal/newpkg": {
			PkgPath: "github.com/example/internal/newpkg",
			Name:    "newpkg",
			Types:   newPkg,
		},
	}

	reports, incompatible := compareImports(oldPkgs, newPkgs)

	// Adding a new package is compatible
	if incompatible {
		t.Error("Expected incompatible=false when new package is added")
	}

	// Should have a report for the new package
	if _, ok := reports["github.com/example/internal/newpkg"]; !ok {
		t.Error("Expected report for new package")
	}
}

// TestComparePackages_MixedCompatibleAndIncompatible tests a scenario with
// both compatible and incompatible changes across multiple packages.
func TestComparePackages_MixedCompatibleAndIncompatible(t *testing.T) {
	// Create old packages
	oldPkg1 := types.NewPackage("github.com/example/internal/pkg1", "pkg1")
	sig := types.NewSignatureType(nil, nil, nil, nil, nil, false)
	oldFunc1 := types.NewFunc(0, oldPkg1, "Func1", sig)
	oldPkg1.Scope().Insert(oldFunc1)

	oldPkg2 := types.NewPackage("github.com/example/internal/pkg2", "pkg2")
	oldFunc2 := types.NewFunc(0, oldPkg2, "Func2", sig)
	oldPkg2.Scope().Insert(oldFunc2)

	oldPkgs := map[string]*packages.Package{
		"github.com/example/internal/pkg1": {
			PkgPath: "github.com/example/internal/pkg1",
			Name:    "pkg1",
			Types:   oldPkg1,
		},
		"github.com/example/internal/pkg2": {
			PkgPath: "github.com/example/internal/pkg2",
			Name:    "pkg2",
			Types:   oldPkg2,
		},
	}

	// Create new packages
	// pkg1: compatible change (add new function)
	newPkg1 := types.NewPackage("github.com/example/internal/pkg1", "pkg1")
	newFunc1 := types.NewFunc(0, newPkg1, "Func1", sig)
	newFunc1b := types.NewFunc(0, newPkg1, "NewFunc", sig)
	newPkg1.Scope().Insert(newFunc1)
	newPkg1.Scope().Insert(newFunc1b)

	// pkg2: incompatible change (remove function)
	newPkg2 := types.NewPackage("github.com/example/internal/pkg2", "pkg2")

	newPkgs := map[string]*packages.Package{
		"github.com/example/internal/pkg1": {
			PkgPath: "github.com/example/internal/pkg1",
			Name:    "pkg1",
			Types:   newPkg1,
		},
		"github.com/example/internal/pkg2": {
			PkgPath: "github.com/example/internal/pkg2",
			Name:    "pkg2",
			Types:   newPkg2,
		},
	}

	reports, incompatible := compareImports(oldPkgs, newPkgs)

	// Should be incompatible overall because pkg2 has breaking change
	if !incompatible {
		t.Error("Expected incompatible=true when one package has breaking changes")
	}

	// Should have reports for both packages
	if len(reports) != 2 {
		t.Errorf("Expected reports for 2 packages, got %d", len(reports))
	}

	// pkg1 should have changes but all compatible
	pkg1Report := reports["github.com/example/internal/pkg1"]
	for _, change := range pkg1Report.Changes {
		if !change.Compatible {
			t.Error("Expected all changes in pkg1 to be compatible")
		}
	}

	// pkg2 should have incompatible changes
	pkg2Report := reports["github.com/example/internal/pkg2"]
	foundIncompatible := false
	for _, change := range pkg2Report.Changes {
		if !change.Compatible {
			foundIncompatible = true
			break
		}
	}
	if !foundIncompatible {
		t.Error("Expected pkg2 to have incompatible changes")
	}
}

// TestComparePackages_TypeAliasChanges tests detection of type alias changes.
func TestComparePackages_TypeAliasChanges(t *testing.T) {
	// Create old package with type alias
	oldPkg := types.NewPackage("github.com/example/internal/types", "types")
	oldAlias := types.NewTypeName(0, oldPkg, "MyInt", types.Typ[types.Int])
	oldPkg.Scope().Insert(oldAlias)

	oldPkgs := map[string]*packages.Package{
		"github.com/example/internal/types": {
			PkgPath: "github.com/example/internal/types",
			Name:    "types",
			Types:   oldPkg,
		},
	}

	// Create new package without the type alias (breaking change)
	newPkg := types.NewPackage("github.com/example/internal/types", "types")
	newPkgs := map[string]*packages.Package{
		"github.com/example/internal/types": {
			PkgPath: "github.com/example/internal/types",
			Name:    "types",
			Types:   newPkg,
		},
	}

	reports, incompatible := compareImports(oldPkgs, newPkgs)

	if !incompatible {
		t.Error("Expected incompatible=true when type alias is removed")
	}

	if len(reports["github.com/example/internal/types"].Changes) == 0 {
		t.Error("Expected changes to be detected for type alias removal")
	}
}
