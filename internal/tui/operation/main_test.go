package operation

import (
	"os"
	"testing"

	"github.com/sglavoie/docket/internal/testutil/nofsync"
)

// TestMain isolates $HOME for every test in the operation package. Nothing here
// writes durable state on purpose, but the guard is unconditional across
// internal/tui: a descriptor manifest reads whatever registry a test hands it,
// and the moment one of those fixtures reaches the real storage stack the
// developer's own ~/.tt is what it would touch.
func TestMain(m *testing.M) {
	nofsync.Enable()
	tmp, err := os.MkdirTemp("", "tt-operation-home-*")
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
