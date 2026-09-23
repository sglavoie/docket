package notesops

import (
	"os"
	"testing"

	"github.com/sglavoie/tt/internal/testutil/residue"
)

// TestMain isolates $HOME for every test in the notesops package. The note
// mutations here travel through the shell port rather than the filesystem, but
// every test-bearing package under internal/tui carries the same guard so a
// test that later reaches ~/.tt/runtime/<workspace-id>/machine.json cannot
// write into the developer's real home directory.
//
// residue.Run also fails the run when it leaves .tt/, projects/, or a lock file
// in the package directory, which is what an unrooted runtime or project writes.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "tt-notesops-home-*")
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
