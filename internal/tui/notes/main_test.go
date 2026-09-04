package notes

import (
	"os"
	"testing"
)

// TestMain isolates $HOME for every test in the notes package. The manager
// resolves note directories from real models.Project values and writes through
// the storage stack, so without this guard a mutation test would reach the
// developer's own ~/.tt.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "tt-notes-home-*")
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
