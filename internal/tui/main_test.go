package tui

import (
	"os"
	"testing"
)

// TestMain isolates $HOME for every test in the tui package. TUI tests that
// build a config.Manager and persist through it resolve machine-local runtime
// state (machine.json under LocalRuntimeDirForDataDir) relative to $HOME.
// Isolating it here keeps those paths from writing into the developer's real
// home directory once that tier becomes part of Manager persistence. Tests that
// override $HOME via t.Setenv restore to this throwaway directory rather than
// the real home.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "tt-tui-home-*")
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
