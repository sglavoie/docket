package taskform

import (
	"os"
	"testing"

	"github.com/sglavoie/tt/internal/testutil/nofsync"

	"github.com/sglavoie/tt/internal/testutil/residue"
)

// TestMain isolates $HOME for every test in the taskform package. The form's
// save path commits through the real storage stack against models.Project
// values, and the machine-local runtime state resolves from $HOME, so without
// this guard a save test would reach the developer's own ~/.tt.
//
// residue.Run also fails the run when it leaves .tt/, projects/, or a lock file
// in the package directory, which is what an unrooted runtime or project writes.
func TestMain(m *testing.M) {
	nofsync.Enable()
	tmp, err := os.MkdirTemp("", "tt-taskform-home-*")
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
