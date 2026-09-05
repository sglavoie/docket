package notesops

import (
	"os"
	"testing"
)

// TestMain isolates $HOME for every test in the notesops package. The note
// mutations here travel through the shell port rather than the filesystem, but
// every test-bearing package under internal/tui carries the same guard so a
// test that later reaches ~/.tt/runtime/<workspace-id>/machine.json cannot
// write into the developer's real home directory.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "tt-notesops-home-*")
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
