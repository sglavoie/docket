package projecthub

import (
	"os"
	"testing"
)

// TestMain isolates $HOME for every test in the projecthub package. The Hub's
// count fixtures write task documents under a temporary data directory and the
// project editor commits through a real config manager, so a stray absolute
// path would otherwise land in the developer's own ~/.tt.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "tt-projecthub-home-*")
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
