package taskdetail

import (
	"os"
	"testing"
)

// TestMain isolates $HOME for every test in the taskdetail package. The comment
// and attachment panels commit through the storage stack against real
// models.Project values, and the machine-local runtime state resolves from
// $HOME, so without this guard a mutation test would reach the developer's own
// ~/.tt.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "tt-taskdetail-home-*")
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
