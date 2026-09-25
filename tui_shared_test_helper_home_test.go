package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// sharedTestHelperHomeThreshold is the number of *other* test files in the same
// package that make a helper shared rather than local. Below it a helper is one
// file's own scaffolding and belongs beside the tests that read it; at or above
// it the helper is infrastructure, and hiding infrastructure in a file that also
// declares tests is what makes a filename-based file move orphan its readers —
// the hazard this whole split runs into every time a surface is extracted.
//
// Five is the bar because it is where the shell's own distribution separates:
// the helpers Phase 1.5 homed run from 5 to 50 readers, and everything left
// below the bar is read by four files or fewer.
const sharedTestHelperHomeThreshold = 5

// sharedTestHelperFloor is a conservative floor under the 76 shared helpers the
// subtree holds today. The guard is only as good as its reach, so a run finding
// far fewer has stopped parsing rather than found a tidier tree. Keep it well
// below the real count: it exists to catch a broken walk, not to track churn.
const sharedTestHelperFloor = 20

// testHelperDecl is one non-test function declared in a test file, kept with the
// file that spells it so a failure names something editable.
type testHelperDecl struct {
	pkg  string
	file string
	name string
}

// TestSharedTestHelpersLiveInFilesThatDeclareNoTest is the helper-home guard.
//
// Go resolves a helper by package, not by file, so a shared helper is invisible
// in the file that declares it: nothing about `outliner_primary_rows_test.go`
// said that 50 other files read `primaryRowsProject`. A session moving that file
// into a sub-package breaks all 50 with no warning ahead of the compile, which
// is the single most expensive surprise in the split. Requiring every shared
// helper to sit in a file that declares no test makes the fan-out legible from
// the filename and gives the move an obvious unit.
//
// The rule is deliberately one-directional. A helper-only file may hold a helper
// nobody else reads yet; what it may not do is hide behind a test.
func TestSharedTestHelpersLiveInFilesThatDeclareNoTest(t *testing.T) {
	t.Parallel()
	root := repositoryRootDir(t)
	packages := testBearingPackageDirs(t, filepath.Join(root, filepath.FromSlash(tuiSubtree)))
	if len(packages) < tuiTestMainFloor {
		t.Fatalf("found %d test-bearing packages under %s, want at least %d; the guard is not reaching them",
			len(packages), tuiSubtree, tuiTestMainFloor)
	}

	shared := 0
	for _, dir := range packages {
		helpers, readers, testBearing := readPackageTestHelpers(t, root, dir)
		for _, helper := range helpers {
			if !testBearing[helper.file] {
				continue
			}
			var elsewhere []string
			for file, names := range readers {
				if file != helper.file && names[helper.name] {
					elsewhere = append(elsewhere, file)
				}
			}
			if len(elsewhere) < sharedTestHelperHomeThreshold {
				continue
			}
			shared++
			slices.Sort(elsewhere)
			t.Errorf("%s/%s declares tests and the helper %s that %d other test files read (%s, ...); "+
				"move %s into a file that declares no test, or a move of its file orphans them",
				helper.pkg, helper.file, helper.name, len(elsewhere), strings.Join(elsewhere[:2], ", "), helper.name)
		}
		shared += countHelperOnlyDecls(helpers, readers, testBearing)
	}

	if shared < sharedTestHelperFloor {
		t.Fatalf("measured %d shared test helpers under %s, want at least %d; the guard would pass vacuously",
			shared, tuiSubtree, sharedTestHelperFloor)
	}
}

// countHelperOnlyDecls counts the shared helpers that already sit in a
// helper-only file. They are the guard's evidence that it is measuring the right
// thing: without them a subtree whose parse silently returned nothing would look
// identical to a clean one.
func countHelperOnlyDecls(helpers []testHelperDecl, readers map[string]map[string]bool, testBearing map[string]bool) int {
	counted := 0
	for _, helper := range helpers {
		if testBearing[helper.file] {
			continue
		}
		elsewhere := 0
		for file, names := range readers {
			if file != helper.file && names[helper.name] {
				elsewhere++
			}
		}
		if elsewhere >= sharedTestHelperHomeThreshold {
			counted++
		}
	}
	return counted
}

// readPackageTestHelpers parses one package's test files and returns every
// non-test function they declare, the identifiers each file reads, and which
// files declare a test.
func readPackageTestHelpers(t *testing.T, root, dir string) ([]testHelperDecl, map[string]map[string]bool, map[string]bool) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	pkg := subtreePath(t, root, dir)

	var helpers []testHelperDecl
	readers := map[string]map[string]bool{}
	testBearing := map[string]bool{}
	fileSet := token.NewFileSet()

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, "_test.go") {
			continue
		}
		parsed, parseErr := parser.ParseFile(fileSet, filepath.Join(dir, name), nil, 0)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", filepath.Join(dir, name), parseErr)
		}
		for _, decl := range parsed.Decls {
			function, ok := decl.(*ast.FuncDecl)
			if !ok || function.Recv != nil {
				continue
			}
			if isGoTestEntryPoint(function.Name.Name) {
				testBearing[name] = true
				continue
			}
			helpers = append(helpers, testHelperDecl{pkg: pkg, file: name, name: function.Name.Name})
		}
		readers[name] = readIdentifiers(parsed)
	}

	slices.SortFunc(helpers, func(left, right testHelperDecl) int {
		if left.file != right.file {
			return strings.Compare(left.file, right.file)
		}
		return strings.Compare(left.name, right.name)
	})
	return helpers, readers, testBearing
}

// isGoTestEntryPoint reports whether a name is one the go tool runs itself.
// TestMain is included: a file holding only TestMain still runs, so a helper
// beside it is as hidden as one beside an ordinary test.
func isGoTestEntryPoint(name string) bool {
	for _, prefix := range []string{"Test", "Benchmark", "Fuzz", "Example"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// readIdentifiers collects the plain identifiers one file reads. A selector's
// member and a composite-literal field name are skipped, because both are
// resolved against a type rather than the package scope and would let an
// unrelated field name inflate a helper's reader count.
func readIdentifiers(file *ast.File) map[string]bool {
	names := map[string]bool{}
	skipped := map[*ast.Ident]bool{}

	ast.Inspect(file, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.SelectorExpr:
			skipped[typed.Sel] = true
		case *ast.CompositeLit:
			for _, element := range typed.Elts {
				pair, ok := element.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				if key, ok := pair.Key.(*ast.Ident); ok {
					skipped[key] = true
				}
			}
		}
		return true
	})

	ast.Inspect(file, func(node ast.Node) bool {
		identifier, ok := node.(*ast.Ident)
		if ok && !skipped[identifier] {
			names[identifier.Name] = true
		}
		return true
	})
	return names
}
