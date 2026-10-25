package textfit

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/testutil/gotree"
)

// This is a deliberate copy of internal/tui/source_anchor_test.go, made when
// the first subtree-wide audit moved out of the shell. Session 1.2 left the
// choice open between duplicating these ~20 lines per package and promoting
// them to a non-test package shared by every audit; duplication wins, because a
// production package that exists only for tests is one `deadcode` would then
// police, and the helper is small, stable and reads better beside the audit it
// serves. What must never be shared is the `runtime.Caller(0)` anchor itself:
// it has to stay in the audit's own file so it keeps describing that file's
// location rather than this helper's.

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
			if path != root && gotree.Ignored(entry.Name()) {
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
