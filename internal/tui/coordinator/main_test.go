package coordinator

import (
	"os"
	"testing"
)

// TestMain isolates $HOME for every test in the coordinator package. The
// scenario tests here drive the real transactional storage stack and the
// workspace undo ledger, so this guard is what keeps a committed mutation from
// reaching ~/.tt/runtime/<workspace-id>/machine.json in the developer's real
// home directory.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "tt-coordinator-home-*")
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
