package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/testutil/gotree"
)

// tuiSubtree is the directory the package split fans out under. Every package
// below it is in scope for this guard, including the root shell itself.
const tuiSubtree = "internal/tui"

// tuiTestMainFloor is the number of test-bearing packages that exist under
// internal/tui today: the shell, keymap, and presentation. The split only ever
// adds packages, so a run that finds fewer has stopped walking the subtree and
// says so instead of passing vacuously.
const tuiTestMainFloor = 3

// homeIsolation records which parts of the shared TestMain shape one package's
// TestMain actually has. The shape is the contract, not the symbol: a TestMain
// that runs the suite without swapping $HOME satisfies a symbol check and still
// lets a test resolve ~/.tt/runtime/<workspace-id>/machine.json into the
// developer's real home directory, which is how internal/tui/.tt/ and
// internal/tui/projects/ became git-tracked residue.
type homeIsolation struct {
	file           string
	makesTempDir   bool
	setsHomeEnv    bool
	runsSuite      bool
	removesTempDir bool
	exitsWithCode  bool
}

func (isolation homeIsolation) missing() []string {
	var gaps []string
	if !isolation.makesTempDir {
		gaps = append(gaps, "os.MkdirTemp")
	}
	if !isolation.setsHomeEnv {
		gaps = append(gaps, `os.Setenv("HOME", ...)`)
	}
	if !isolation.runsSuite {
		gaps = append(gaps, "m.Run()")
	}
	if !isolation.removesTempDir {
		gaps = append(gaps, "os.RemoveAll")
	}
	if !isolation.exitsWithCode {
		gaps = append(gaps, "os.Exit")
	}
	return gaps
}

// TestEveryTestBearingTUIPackageIsolatesHome is the TestMain-per-package guard.
// The split multiplies packages under internal/tui, and a new one whose tests
// build a config.Manager resolves machine-local state relative to $HOME. One
// package without the swap writes into the developer's real home directory, and
// nothing else in the suite notices.
//
// The guard is uniform rather than narrowed to packages whose tests resolve a
// workspace path, because that narrowing cannot be decided statically: it would
// have to guess, and a guess that guesses wrong fails open. Session 1.3 already
// set the uniform shape by giving internal/tui/presentation a TestMain it does
// not need yet.
func TestEveryTestBearingTUIPackageIsolatesHome(t *testing.T) {
	t.Parallel()
	root := repositoryRootDir(t)
	packages := testBearingPackageDirs(t, filepath.Join(root, filepath.FromSlash(tuiSubtree)))
	if len(packages) < tuiTestMainFloor {
		t.Fatalf("found %d test-bearing packages under %s, want at least %d; the walk is not reaching the subtree",
			len(packages), tuiSubtree, tuiTestMainFloor)
	}

	for _, dir := range packages {
		relative := subtreePath(t, root, dir)
		isolation, found := findHomeIsolation(t, dir)
		if !found {
			t.Errorf("%s has tests but no TestMain; copy the $HOME swap from %s/main_test.go",
				relative, tuiSubtree)
			continue
		}
		if gaps := isolation.missing(); len(gaps) > 0 {
			t.Errorf("%s/%s declares TestMain without %s; it does not isolate $HOME",
				relative, isolation.file, strings.Join(gaps, ", "))
		}
	}
}

// testBearingPackageDirs returns every directory below root that holds at least
// one _test.go file, root included.
func testBearingPackageDirs(t *testing.T, root string) []string {
	t.Helper()

	bearing := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if gotree.Ignored(entry.Name()) && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(entry.Name(), "_test.go") {
			bearing[filepath.Dir(path)] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}

	dirs := make([]string, 0, len(bearing))
	for dir := range bearing {
		dirs = append(dirs, dir)
	}
	slices.Sort(dirs)
	return dirs
}

// findHomeIsolation parses every test file in one directory looking for
// TestMain, and reports which parts of the $HOME swap its body performs.
func findHomeIsolation(t *testing.T, dir string) (homeIsolation, bool) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), "_test.go") {
			names = append(names, entry.Name())
		}
	}
	slices.Sort(names)

	fileSet := token.NewFileSet()
	for _, name := range names {
		parsed, parseErr := parser.ParseFile(fileSet, filepath.Join(dir, name), nil, 0)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", filepath.Join(dir, name), parseErr)
		}
		for _, decl := range parsed.Decls {
			function, ok := decl.(*ast.FuncDecl)
			if !ok || function.Recv != nil || function.Name.Name != "TestMain" {
				continue
			}
			isolation := homeIsolation{file: name}
			inspectHomeIsolation(function, &isolation)
			return isolation, true
		}
	}
	return homeIsolation{}, false
}

func inspectHomeIsolation(function *ast.FuncDecl, isolation *homeIsolation) {
	ast.Inspect(function, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		receiver, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		switch {
		case receiver.Name == "os" && selector.Sel.Name == "MkdirTemp":
			isolation.makesTempDir = true
		case receiver.Name == "os" && selector.Sel.Name == "Setenv":
			isolation.setsHomeEnv = isolation.setsHomeEnv || firstArgumentIs(call, "HOME")
		case receiver.Name == "os" && selector.Sel.Name == "RemoveAll":
			isolation.removesTempDir = true
		case receiver.Name == "os" && selector.Sel.Name == "Exit":
			isolation.exitsWithCode = true
		case selector.Sel.Name == "Run":
			isolation.runsSuite = true
		}
		return true
	})
}

// firstArgumentIs reads the environment variable name off the call so a
// TestMain that isolates some other variable is not mistaken for one that
// isolates $HOME.
func firstArgumentIs(call *ast.CallExpr, want string) bool {
	if len(call.Args) == 0 {
		return false
	}
	literal, ok := call.Args[0].(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return false
	}
	value, err := strconv.Unquote(literal.Value)
	if err != nil {
		return false
	}
	return value == want
}

// subtreePath names a directory the way the failure message should read it:
// relative to the repository root, in slash form.
func subtreePath(t *testing.T, root, dir string) string {
	t.Helper()

	relative, err := filepath.Rel(root, dir)
	if err != nil {
		t.Fatalf("relativize %s: %v", dir, err)
	}
	return filepath.ToSlash(relative)
}
