package outliner

import (
	"os"
	"testing"

	"github.com/sglavoie/docket/internal/testutil/nofsync"

	"github.com/sglavoie/docket/internal/testutil/residue"
)

// TestMain isolates $HOME for every test in the outliner package. TUI tests that
// build a config.Manager and persist through it resolve machine-local state
// under ~/.tt/runtime/<workspace-id>/machine.json relative to $HOME.
// Isolating it here keeps those paths from writing into the developer's real
// home directory once that tier becomes part of Manager persistence. Tests that
// override $HOME via t.Setenv restore to this throwaway directory rather than
// the real home.
//
// residue.Run also fails the run when it leaves .tt/, projects/, or a lock file
// in the package directory, which is what an unrooted runtime or project writes.
func TestMain(m *testing.M) {
	nofsync.Enable()
	tmp, err := os.MkdirTemp("", "tt-outliner-home-*")
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
