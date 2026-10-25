package chrome

import (
	"os"
	"testing"

	"github.com/sglavoie/docket/internal/testutil/nofsync"
)

// TestMain isolates $HOME for every test in the chrome package. TUI tests that
// build a config.Manager and persist through it resolve machine-local state
// under ~/.docket/runtime/<workspace-id>/machine.json relative to $HOME.
// Isolating it here keeps those paths from writing into the developer's real
// home directory once that tier becomes part of Manager persistence. Tests that
// override $HOME via t.Setenv restore to this throwaway directory rather than
// the real home.
func TestMain(m *testing.M) {
	nofsync.Enable()
	tmp, err := os.MkdirTemp("", "tt-chrome-home-*")
	if err != nil {
		panic(err)
	}
	if err := os.Setenv("HOME", tmp); err != nil {
		panic(err)
	}
	code := m.Run()
	_ = os.RemoveAll(tmp)
	os.Exit(code)
}
