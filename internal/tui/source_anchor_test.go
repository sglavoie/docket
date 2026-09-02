package tui

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The package-wide audits below must keep meaning the same thing after
// internal/tui is split into sub-packages. A walk rooted at "." is really a
// walk of whatever directory the test binary happens to run in, so a file that
// moves one package down falls out of scope and the audit keeps reporting
// success. Everything here anchors on this file's own compiled-in path
// instead, which is correct at any package depth and independent of the
// working directory.

// tuiRepositoryRoot resolves the repository root by walking up from this
// file's own path until it finds the module's go.mod.
func tuiRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate the tui source anchor")
	}
	dir := filepath.Dir(thisFile)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate the repository root above %s", thisFile)
		}
		dir = parent
	}
}

// tuiSourceRoot is the internal/tui subtree: the whole TUI, including every
// sub-package, whichever one this test file itself lives in.
func tuiSourceRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(tuiRepositoryRoot(t), "internal", "tui")
}

// walkTUIProductionSources visits every non-test .go file in the internal/tui
// subtree. relative is the slash-separated path below that subtree root and is
// what diagnostics should name; path is absolute and is what a parser or a
// reader should open. A visit error stops the walk and is returned, so each
// audit keeps its own failure wording.
func walkTUIProductionSources(t *testing.T, visit func(path, relative string, entry fs.DirEntry) error) error {
	t.Helper()
	root := tuiSourceRoot(t)
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && skipsTUISourceDir(entry.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		name := entry.Name()
		if filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		return visit(path, filepath.ToSlash(relative), entry)
	})
}

// skipsTUISourceDir reports directories the Go tool itself never compiles, so
// no production source can hide in one: dot and underscore directories, plus
// testdata. That is also what keeps the walk out of the git-tracked .tt/ test
// residue under internal/tui. Never skip an ordinary directory name here — a
// sub-package must not be able to opt out of an audit by being called
// something.
func skipsTUISourceDir(name string) bool {
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "testdata"
}
