package tui

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The package-wide audits below must keep meaning the same thing after
// internal/tui is split into sub-packages. A walk rooted at "." is really a
// walk of whatever directory the test binary happens to run in, so a file that
// moves one package down falls out of scope and the audit keeps reporting
// success. Every audit anchors on its own compiled-in `runtime.Caller(0)` path
// instead and hands that path in here, which is correct at any package depth
// and independent of the working directory. The anchor stays in the calling
// file on purpose: it is the one part that must move with the file, so it may
// not be delegated to a helper whose own path would answer instead.

// tuiRepositoryRootFrom resolves the repository root by walking up from the
// caller's own source path until it finds the module's go.mod. Finding go.mod
// rather than counting directories upward is what survives a new package depth.
func tuiRepositoryRootFrom(t *testing.T, sourceFile string) string {
	t.Helper()
	dir := filepath.Dir(sourceFile)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate the repository root above %s", sourceFile)
		}
		dir = parent
	}
}

// tuiSourceRootFrom is the internal/tui subtree: the whole TUI, including every
// sub-package, whichever one the calling test file itself lives in.
func tuiSourceRootFrom(t *testing.T, sourceFile string) string {
	t.Helper()
	return filepath.Join(tuiRepositoryRootFrom(t, sourceFile), "internal", "tui")
}

// walkTUIProductionSourcesFrom visits every non-test .go file in the
// internal/tui subtree resolved from the caller's own source path. relative is
// the slash-separated path below that subtree root and is what diagnostics
// should name; path is absolute and is what a parser or a reader should open. A
// visit error stops the walk and is returned, so each audit keeps its own
// failure wording.
func walkTUIProductionSourcesFrom(t *testing.T, sourceFile string, visit func(path, relative string, entry fs.DirEntry) error) error {
	t.Helper()
	root := tuiSourceRootFrom(t, sourceFile)
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
