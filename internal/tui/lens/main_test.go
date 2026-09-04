package lens

import (
	"os"
	"testing"
)

// TestMain isolates $HOME for every test in the lens package. Nothing here
// resolves a workspace path today — the picker is fed a snapshot and the
// semantic resolver reads a definition alone — but the guard is uniform on
// purpose, because narrowing it to the packages that need it has to guess and a
// guess that guesses wrong writes into the developer's own ~/.tt.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "tt-lens-home-*")
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
