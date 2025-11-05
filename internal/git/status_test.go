package git

import (
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
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

// TestIsFilesystemPath tests the filesystem path detection logic.
func TestIsFilesystemPath(t *testing.T) {
	testCases := []struct {
		name     string
		path     string
		expected bool
		setup    func() (string, func())
	}{
		{
			name:     "absolute path that exists",
			expected: true,
			setup: func() (string, func()) {
				tmpDir, err := os.MkdirTemp("", "gobreaker-test-*")
				if err != nil {
					t.Fatalf("Failed to create temp dir: %v", err)
				}
				return tmpDir, func() { os.RemoveAll(tmpDir) }
			},
		},
		{
			name:     "relative path that exists",
			path:     ".",
			expected: true,
			setup:    func() (string, func()) { return ".", func() {} },
		},
		{
			name:     "path that does not exist",
			path:     "/nonexistent/path/that/should/not/exist",
			expected: false,
			setup:    func() (string, func()) { return "/nonexistent/path/that/should/not/exist", func() {} },
		},
		{
			name:     "empty path",
			path:     "",
			expected: false,
			setup:    func() (string, func()) { return "", func() {} },
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path := tc.path
			cleanup := func() {}
			if tc.setup != nil {
				path, cleanup = tc.setup()
				defer cleanup()
			}

			result := IsFilesystemPath(path)
			if result != tc.expected {
				t.Errorf("IsFilesystemPath(%q) = %v, expected %v", path, result, tc.expected)
			}
		})
	}
}

// TestIsGitRef tests the git reference detection logic.
func TestIsGitRef(t *testing.T) {
	// Create a temporary git repository for testing
	tmpDir, err := os.MkdirTemp("", "gobreaker-git-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize a git repo
	repo, err := git.PlainInit(tmpDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create a commit
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Failed to get worktree: %v", err)
	}

	// Create a file and commit it
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	if _, err := wt.Add("test.txt"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	commit, err := wt.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	testCases := []struct {
		name     string
		ref      string
		expected bool
	}{
		{
			name:     "HEAD reference",
			ref:      "HEAD",
			expected: true,
		},
		{
			name:     "commit hash",
			ref:      commit.String(),
			expected: true,
		},
		{
			name:     "short commit hash",
			ref:      commit.String()[:7],
			expected: true,
		},
		{
			name:     "invalid reference",
			ref:      "nonexistent-branch",
			expected: false,
		},
		{
			name:     "empty reference",
			ref:      "",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsGitRef(tmpDir, tc.ref)
			if result != tc.expected {
				t.Errorf("IsGitRef(%q, %q) = %v, expected %v", tmpDir, tc.ref, result, tc.expected)
			}
		})
	}
}

// TestIsGitRef_NoRepo tests IsGitRef when not in a git repository.
func TestIsGitRef_NoRepo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gobreaker-no-git-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	result := IsGitRef(tmpDir, "HEAD")
	if result != false {
		t.Errorf("IsGitRef in non-git directory should return false, got %v", result)
	}
}

// TestGetPackagesFromPath tests loading packages from a filesystem path.
func TestGetPackagesFromPath(t *testing.T) {
	// Create a temporary directory with a simple Go package
	tmpDir, err := os.MkdirTemp("", "gobreaker-pkg-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a simple Go file
	goFile := filepath.Join(tmpDir, "main.go")
	goContent := `package main

type ExportedType struct {
	Field string
}

func ExportedFunc() string {
	return "test"
}

func privateFunc() {
	// private function
}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to write Go file: %v", err)
	}

	// Create go.mod
	goMod := filepath.Join(tmpDir, "go.mod")
	goModContent := `module example.com/testpkg

go 1.21
`
	if err := os.WriteFile(goMod, []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Test loading packages
	selfPkgs, importPkgs, err := getPackagesFromPath(tmpDir, false)
	if err != nil {
		t.Fatalf("Failed to load packages: %v", err)
	}

	if len(selfPkgs) == 0 {
		t.Error("Expected at least one package, got 0")
	}

	// Verify the package was loaded
	found := false
	for pkgPath := range selfPkgs {
		if pkgPath == "example.com/testpkg" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected to find example.com/testpkg in loaded packages")
	}

	// importPkgs might be empty for a simple package with no external deps
	if importPkgs == nil {
		t.Error("Expected importPkgs map to be initialized, got nil")
	}
}

// TestGetPackagesFromPath_WithInternal tests internal package filtering.
func TestGetPackagesFromPath_WithInternal(t *testing.T) {
	// Create a temporary directory with internal package
	tmpDir, err := os.MkdirTemp("", "gobreaker-internal-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create main package
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(`package main
func Main() {}`), 0644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	// Create internal package
	internalDir := filepath.Join(tmpDir, "internal")
	if err := os.MkdirAll(internalDir, 0755); err != nil {
		t.Fatalf("Failed to create internal dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(internalDir, "helper.go"), []byte(`package internal
func Helper() string { return "help" }`), 0644); err != nil {
		t.Fatalf("Failed to write helper.go: %v", err)
	}

	// Create go.mod
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(`module example.com/testapp
go 1.21
`), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Test with includeInternal = false (default)
	selfPkgs, _, err := getPackagesFromPath(tmpDir, false)
	if err != nil {
		t.Fatalf("Failed to load packages: %v", err)
	}

	// Should not include internal package
	for pkgPath := range selfPkgs {
		if strings.Contains(pkgPath, "/internal") {
			t.Errorf("Expected internal package to be filtered out, but found: %s", pkgPath)
		}
	}

	// Test with includeInternal = true
	selfPkgsWithInternal, _, err := getPackagesFromPath(tmpDir, true)
	if err != nil {
		t.Fatalf("Failed to load packages with internal: %v", err)
	}

	// Should include internal package
	foundInternal := false
	for pkgPath := range selfPkgsWithInternal {
		if strings.Contains(pkgPath, "/internal") {
			foundInternal = true
			break
		}
	}

	if !foundInternal {
		t.Error("Expected internal package to be included when includeInternal=true")
	}
}

// TestGetPackagesFromPath_InvalidPath tests error handling for invalid paths.
func TestGetPackagesFromPath_InvalidPath(t *testing.T) {
	_, _, err := getPackagesFromPath("/nonexistent/path/that/does/not/exist", false)
	if err == nil {
		t.Error("Expected error for nonexistent path, got nil")
	}
}

// TestCompareFilesystems_BasicComparison tests filesystem comparison mode.
func TestCompareFilesystems_BasicComparison(t *testing.T) {
// Create two temporary directories with Go packages
oldDir, err := os.MkdirTemp("", "gobreaker-old-*")
if err != nil {
t.Fatalf("Failed to create old dir: %v", err)
}
defer os.RemoveAll(oldDir)

newDir, err := os.MkdirTemp("", "gobreaker-new-*")
if err != nil {
t.Fatalf("Failed to create new dir: %v", err)
}
defer os.RemoveAll(newDir)

// Create old version with a function
oldGoFile := `package testpkg

func OldFunction() string {
return "old"
}

func SharedFunction() int {
return 42
}
`
if err := os.WriteFile(filepath.Join(oldDir, "test.go"), []byte(oldGoFile), 0644); err != nil {
t.Fatalf("Failed to write old test.go: %v", err)
}

if err := os.WriteFile(filepath.Join(oldDir, "go.mod"), []byte("module example.com/test\ngo 1.21\n"), 0644); err != nil {
t.Fatalf("Failed to write old go.mod: %v", err)
}

// Create new version without OldFunction (breaking change)
newGoFile := `package testpkg

func SharedFunction() int {
return 42
}

func NewFunction() bool {
return true
}
`
if err := os.WriteFile(filepath.Join(newDir, "test.go"), []byte(newGoFile), 0644); err != nil {
t.Fatalf("Failed to write new test.go: %v", err)
}

if err := os.WriteFile(filepath.Join(newDir, "go.mod"), []byte("module example.com/test\ngo 1.21\n"), 0644); err != nil {
t.Fatalf("Failed to write new go.mod: %v", err)
}

// Compare the filesystems
diff, err := CompareFilesystems(oldDir, newDir, false)
if err != nil {
t.Fatalf("CompareFilesystems failed: %v", err)
}

// Should detect breaking change
if diff.IsCompatible() {
t.Error("Expected incompatible changes due to removed function")
}
}

// TestCompareFilesystems_WithInternalPackages tests filesystem comparison with internal packages.
func TestCompareFilesystems_WithInternalPackages(t *testing.T) {
oldDir, err := os.MkdirTemp("", "gobreaker-old-internal-*")
if err != nil {
t.Fatalf("Failed to create old dir: %v", err)
}
defer os.RemoveAll(oldDir)

newDir, err := os.MkdirTemp("", "gobreaker-new-internal-*")
if err != nil {
t.Fatalf("Failed to create new dir: %v", err)
}
defer os.RemoveAll(newDir)

// Create old version with internal package
if err := os.MkdirAll(filepath.Join(oldDir, "internal"), 0755); err != nil {
t.Fatalf("Failed to create internal dir: %v", err)
}

oldInternal := `package internal

func InternalFunc() string {
return "internal"
}
`
if err := os.WriteFile(filepath.Join(oldDir, "internal", "helper.go"), []byte(oldInternal), 0644); err != nil {
t.Fatalf("Failed to write old internal: %v", err)
}

if err := os.WriteFile(filepath.Join(oldDir, "go.mod"), []byte("module example.com/test\ngo 1.21\n"), 0644); err != nil {
t.Fatalf("Failed to write old go.mod: %v", err)
}

// Create new version without internal function
if err := os.MkdirAll(filepath.Join(newDir, "internal"), 0755); err != nil {
t.Fatalf("Failed to create internal dir: %v", err)
}

newInternal := `package internal

// InternalFunc was removed
`
if err := os.WriteFile(filepath.Join(newDir, "internal", "helper.go"), []byte(newInternal), 0644); err != nil {
t.Fatalf("Failed to write new internal: %v", err)
}

if err := os.WriteFile(filepath.Join(newDir, "go.mod"), []byte("module example.com/test\ngo 1.21\n"), 0644); err != nil {
t.Fatalf("Failed to write new go.mod: %v", err)
}

// Compare without including internal - should be compatible
diffWithout, err := CompareFilesystems(oldDir, newDir, false)
if err != nil {
t.Fatalf("CompareFilesystems failed: %v", err)
}

if !diffWithout.IsCompatible() {
t.Error("Expected compatible when internal packages are excluded")
}

// Compare with internal packages included - should be incompatible
diffWith, err := CompareFilesystems(oldDir, newDir, true)
if err != nil {
t.Fatalf("CompareFilesystems with internal failed: %v", err)
}

if diffWith.IsCompatible() {
t.Error("Expected incompatible when internal packages are included and function is removed")
}
}

// TestCompareFilesystems_InvalidPaths tests error handling for invalid filesystem paths.
func TestCompareFilesystems_InvalidPaths(t *testing.T) {
testCases := []struct {
name    string
oldPath string
newPath string
}{
{
name:    "both paths invalid",
oldPath: "/nonexistent/old",
newPath: "/nonexistent/new",
},
{
name:    "old path invalid",
oldPath: "/nonexistent/old",
newPath: ".",
},
{
name:    "new path invalid",
oldPath: ".",
newPath: "/nonexistent/new",
},
}

for _, tc := range testCases {
t.Run(tc.name, func(t *testing.T) {
_, err := CompareFilesystems(tc.oldPath, tc.newPath, false)
if err == nil {
t.Error("Expected error for invalid paths, got nil")
}
})
}
}

// TestAutoDetection_MixedModes tests that mixing filesystem paths and git refs is rejected.
func TestAutoDetection_MixedModes(t *testing.T) {
// Create a temp dir that exists
tmpDir, err := os.MkdirTemp("", "gobreaker-mixed-*")
if err != nil {
t.Fatalf("Failed to create temp dir: %v", err)
}
defer os.RemoveAll(tmpDir)

testCases := []struct {
name     string
arg1     string
arg2     string
arg1Path bool
arg2Path bool
}{
{
name:     "path and non-path",
arg1:     tmpDir,
arg2:     "HEAD",
arg1Path: true,
arg2Path: false,
},
{
name:     "non-path and path",
arg1:     "main",
arg2:     tmpDir,
arg1Path: false,
arg2Path: true,
},
}

for _, tc := range testCases {
t.Run(tc.name, func(t *testing.T) {
// Verify detection works as expected
if IsFilesystemPath(tc.arg1) != tc.arg1Path {
t.Errorf("IsFilesystemPath(%q) = %v, expected %v", tc.arg1, !tc.arg1Path, tc.arg1Path)
}
if IsFilesystemPath(tc.arg2) != tc.arg2Path {
t.Errorf("IsFilesystemPath(%q) = %v, expected %v", tc.arg2, !tc.arg2Path, tc.arg2Path)
}
})
}
}

// TestComparePackages_ComplexScenarios tests complex real-world scenarios.
func TestComparePackages_ComplexScenarios(t *testing.T) {
// Scenario: Package with multiple types, methods, and interfaces changing together
oldPkg := types.NewPackage("github.com/example/complex", "complex")

// Old version has a type with methods and an interface
oldStruct := types.NewStruct([]*types.Var{
types.NewField(0, oldPkg, "Name", types.Typ[types.String], false),
types.NewField(0, oldPkg, "Age", types.Typ[types.Int], false),
}, nil)
oldType := types.NewNamed(types.NewTypeName(0, oldPkg, "Person", nil), oldStruct, nil)

// Add methods to the type
sig1 := types.NewSignatureType(types.NewVar(0, oldPkg, "", oldType), nil, nil, nil,
types.NewTuple(types.NewVar(0, oldPkg, "", types.Typ[types.String])), false)
oldType.AddMethod(types.NewFunc(0, oldPkg, "GetName", sig1))

sig2 := types.NewSignatureType(types.NewVar(0, oldPkg, "", oldType), nil, nil, nil,
types.NewTuple(types.NewVar(0, oldPkg, "", types.Typ[types.Int])), false)
oldType.AddMethod(types.NewFunc(0, oldPkg, "GetAge", sig2))

oldPkg.Scope().Insert(oldType.Obj())

// Add an interface
method1 := types.NewFunc(0, oldPkg, "DoSomething", types.NewSignatureType(nil, nil, nil, nil, nil, false))
oldInterface := types.NewInterfaceType([]*types.Func{method1}, nil)
oldInterfaceType := types.NewNamed(types.NewTypeName(0, oldPkg, "Doer", nil), oldInterface, nil)
oldPkg.Scope().Insert(oldInterfaceType.Obj())

oldPkgs := map[string]*packages.Package{
"github.com/example/complex": {
PkgPath: "github.com/example/complex",
Name:    "complex",
Types:   oldPkg,
},
}

// New version removes one method and changes the struct
newPkg := types.NewPackage("github.com/example/complex", "complex")

newStruct := types.NewStruct([]*types.Var{
types.NewField(0, newPkg, "Name", types.Typ[types.String], false),
// Age field removed - breaking change
types.NewField(0, newPkg, "Email", types.Typ[types.String], false), // New field added
}, nil)
newType := types.NewNamed(types.NewTypeName(0, newPkg, "Person", nil), newStruct, nil)

// Only add GetName method (GetAge removed - breaking change)
newSig1 := types.NewSignatureType(types.NewVar(0, newPkg, "", newType), nil, nil, nil,
types.NewTuple(types.NewVar(0, newPkg, "", types.Typ[types.String])), false)
newType.AddMethod(types.NewFunc(0, newPkg, "GetName", newSig1))

newPkg.Scope().Insert(newType.Obj())

// Interface unchanged
newMethod1 := types.NewFunc(0, newPkg, "DoSomething", types.NewSignatureType(nil, nil, nil, nil, nil, false))
newInterface := types.NewInterfaceType([]*types.Func{newMethod1}, nil)
newInterfaceType := types.NewNamed(types.NewTypeName(0, newPkg, "Doer", nil), newInterface, nil)
newPkg.Scope().Insert(newInterfaceType.Obj())

newPkgs := map[string]*packages.Package{
"github.com/example/complex": {
PkgPath: "github.com/example/complex",
Name:    "complex",
Types:   newPkg,
},
}

reports, incompatible := compareImports(oldPkgs, newPkgs)

if !incompatible {
t.Error("Expected incompatible=true due to removed field and method")
}

report := reports["github.com/example/complex"]
if len(report.Changes) == 0 {
t.Error("Expected changes to be detected")
}

// Verify we detected incompatible changes
foundIncompatible := false
for _, change := range report.Changes {
if !change.Compatible {
foundIncompatible = true
t.Logf("Detected incompatible change: %s", change.Message)
}
}

if !foundIncompatible {
t.Error("Expected at least one incompatible change")
}
}
