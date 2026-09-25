package query

import (
	"os"
	"testing"

	"github.com/sglavoie/tt/internal/testutil/nofsync"
)

// TestMain isolates $HOME for every test in the query package. Nothing here
// writes durable state on purpose, but the guard is unconditional across
// internal/tui: a compiled predicate reads whatever population a test hands it,
// and the moment one of those fixtures reaches the real storage stack the
// developer's own ~/.tt is what it would touch.
func TestMain(m *testing.M) {
	nofsync.Enable()
	tmp, err := os.MkdirTemp("", "tt-query-home-*")
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
