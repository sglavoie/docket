// Package residue guards a test binary's working directory against workspace
// artifacts. A test whose runtime or project is rooted at an empty path resolves
// `.docket/`, the `.tt/` tombstone, `projects/`, and lock files beneath the
// package directory, which is the source tree; a TestMain snapshots that
// directory before the run and refuses whatever appeared during it.
package residue

import (
	"fmt"
	"os"
	"strings"
)

// Names are the workspace artifacts a stray write leaves in a working directory.
var Names = []string{".docket", ".tt", "projects", ".lock", "tasks.md.lock", ".tasks.md.lock"}

// Snapshot records which of Names already exist in the working directory and
// returns the check to run after m.Run: it reports every name that appeared
// since, or nil when none did.
func Snapshot() func() error {
	before := present()
	return func() error {
		var appeared []string
		for name := range present() {
			if !before[name] {
				appeared = append(appeared, name)
			}
		}
		if len(appeared) == 0 {
			return nil
		}
		wd, _ := os.Getwd()
		return fmt.Errorf("test run wrote workspace residue into %s: %s; root the runtime or project at t.TempDir()", wd, strings.Join(appeared, ", "))
	}
}

func present() map[string]bool {
	found := make(map[string]bool, len(Names))
	for _, name := range Names {
		if _, err := os.Lstat(name); err == nil {
			found[name] = true
		}
	}
	return found
}

// Run executes the test binary and fails it when the run left residue behind,
// so a TestMain's exit code carries both verdicts.
func Run(run func() int) int {
	check := Snapshot()
	code := run()
	if err := check(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		if code == 0 {
			code = 1
		}
	}
	return code
}
