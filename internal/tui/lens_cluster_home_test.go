package tui

import "testing"

// TestLensClusterHasOneHome pins the named-lens cluster to its single named
// AppModel field. The picker and the launch pointer are the two pieces of lens
// state the shell holds at all; a second home is how a drained pointer would
// come to be applied twice.
func TestLensClusterHasOneHome(t *testing.T) {
	t.Parallel()

	clustered := map[string]bool{
		"lensPicker":        true,
		"pendingLaunchLens": true,
	}
	assertAppModelClusterHasOneHome(t, "lenses", clustered)
}
