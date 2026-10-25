package taskdetail

import (
	"os"
	"testing"

	"github.com/sglavoie/docket/internal/testutil/nofsync"

	"github.com/sglavoie/docket/internal/testutil/residue"
)

// TestMain isolates $HOME for every test in the taskdetail package. The comment
// and attachment panels commit through the storage stack against real
// models.Project values, and the machine-local runtime state resolves from
// $HOME, so without this guard a mutation test would reach the developer's own
// ~/.tt.
//
// residue.Run also fails the run when it leaves .docket/, projects/, or a lock file
// in the package directory, which is what an unrooted runtime or project writes.
func TestMain(m *testing.M) {
	nofsync.Enable()
	tmp, err := os.MkdirTemp("", "tt-taskdetail-home-*")
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
