package config

import (
	"os"
	"testing"

	"github.com/sglavoie/tt/internal/testutil/nofsync"

	"github.com/sglavoie/tt/internal/testutil/residue"
)

// TestMain isolates $HOME for every test in the config package. Manager
// persistence resolves machine-local state under
// ~/.tt/runtime/<workspace-id>/machine.json relative to $HOME, so without isolation a test
// that exercises Save/LoadOrCreate would write into the developer's real home
// directory once that tier becomes part of Manager persistence. Individual
// tests that need a specific $HOME still override it via t.Setenv; those
// overrides restore to this throwaway directory rather than the real home.
//
// residue.Run also fails the run when it leaves .tt/, projects/, or a lock file
// in the package directory, which is what an unrooted runtime or project writes.
func TestMain(m *testing.M) {
	nofsync.Enable()
	tmp, err := os.MkdirTemp("", "tt-config-home-*")
	if err != nil {
		panic(err)
	}
	if err := os.Setenv("HOME", tmp); err != nil {
		panic(err)
	}
	code := residue.Run(m.Run)
	_ = os.RemoveAll(tmp)
	os.Exit(code)
}
