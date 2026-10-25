package projecthub

import (
	"os"
	"testing"

	"github.com/sglavoie/docket/internal/testutil/nofsync"

	"github.com/sglavoie/docket/internal/testutil/residue"
)

// TestMain isolates $HOME for every test in the projecthub package. The Hub's
// count fixtures write task documents under a temporary data directory and the
// project editor commits through a real config manager, so a stray absolute
// path would otherwise land in the developer's own ~/.tt.
//
// residue.Run also fails the run when it leaves .docket/, projects/, or a lock file
// in the package directory, which is what an unrooted runtime or project writes.
func TestMain(m *testing.M) {
	nofsync.Enable()
	tmp, err := os.MkdirTemp("", "tt-projecthub-home-*")
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
