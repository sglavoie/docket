package listsurface

import (
	"os"
	"testing"

	"github.com/sglavoie/docket/internal/testutil/nofsync"
)

// TestMain isolates $HOME for every test in the listsurface package. The list
// surfaces themselves persist nothing, but the guard is unconditional across
// internal/tui: the tag and parent selectors are seeded from real
// models.Project and models.Task values here, and the moment one of those
// fixtures reaches the storage stack the developer's own ~/.tt is what it
// would touch.
func TestMain(m *testing.M) {
	nofsync.Enable()
	tmp, err := os.MkdirTemp("", "tt-listsurface-home-*")
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
