package taskform

import (
	"os"
	"testing"
)

// TestMain isolates $HOME for every test in the taskform package. The form's
// save path commits through the real storage stack against models.Project
// values, and the machine-local runtime state resolves from $HOME, so without
// this guard a save test would reach the developer's own ~/.tt.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "tt-taskform-home-*")
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
