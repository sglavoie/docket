package operationhost

import (
	"os"
	"testing"

	"github.com/sglavoie/tt/internal/testutil/nofsync"
)

// TestMain isolates $HOME for every test in the operationhost package. The
// wiring here touches no filesystem, but every test-bearing package under
// internal/tui carries the same guard so a test that later reaches
// ~/.tt/runtime/<workspace-id>/machine.json cannot write into the developer's
// real home directory.
func TestMain(m *testing.M) {
	nofsync.Enable()
	tmp, err := os.MkdirTemp("", "tt-operationhost-home-*")
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
