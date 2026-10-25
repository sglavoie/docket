package help

import (
	"os"
	"testing"

	"github.com/sglavoie/docket/internal/testutil/nofsync"
)

// TestMain isolates $HOME for every test in the help package. The `?` viewer
// persists nothing, but the guard is unconditional across internal/tui: the
// grammar section is projected from the shared registry and the render tests
// build the real simplified strategy, so the moment either reaches config or
// storage the developer's own ~/.docket is what it would touch.
func TestMain(m *testing.M) {
	nofsync.Enable()
	tmp, err := os.MkdirTemp("", "tt-help-home-*")
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
