package workspace

import (
	"os"
	"testing"

	"github.com/sglavoie/tt/internal/testutil/nofsync"
)

// TestMain skips fsync for this package's tests, which write their workspaces
// into throwaway temporary directories whose durability nothing depends on.
func TestMain(m *testing.M) {
	nofsync.Enable()
	os.Exit(m.Run())
}
