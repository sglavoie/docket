package tui

import "testing"

// TestNoteIndexClusterHasOneHome pins the notes cluster to its single named
// AppModel field. The rows, the generation that discards a superseded read, and
// the owner fingerprint are one answer with one discipline; a second home is how
// a stale read would come to be adopted.
func TestNoteIndexClusterHasOneHome(t *testing.T) {
	t.Parallel()

	clustered := map[string]bool{
		"noteIndexRows":       true,
		"noteIndexGeneration": true,
		"noteIndexOwners":     true,
	}
	assertAppModelClusterHasOneHome(t, "notes", clustered)
}
