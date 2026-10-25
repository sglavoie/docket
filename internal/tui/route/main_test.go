package route

import (
	"os"
	"testing"

	"github.com/sglavoie/docket/internal/testutil/nofsync"
)

// TestMain isolates $HOME for every test in the route package. Nothing here
// writes durable state on purpose, but the guard is unconditional across
// internal/tui: a route fixture carries whatever params a test hands it, and
// the moment one of those reaches the real storage stack the developer's own
// ~/.docket is what it would touch.
func TestMain(m *testing.M) {
	nofsync.Enable()
	tmp, err := os.MkdirTemp("", "tt-route-home-*")
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
