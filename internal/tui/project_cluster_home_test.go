package tui

import "testing"

// TestProjectClusterHasOneHome pins the cluster to its single named AppModel
// field. Every pending here is an edit a prompt is standing in front of; a
// second home on the god struct is how a prompt and the commit it gates would
// come to read different values.
func TestProjectClusterHasOneHome(t *testing.T) {
	t.Parallel()

	clustered := map[string]bool{
		"pendingProjectArchive":        true,
		"pendingProjectArchiveArchive": true,
		"projectArchiveConfirm":        true,
		"pendingProjectDelete":         true,
		"projectDeleteConfirm":         true,
		"pendingProjectTags":           true,
		"pendingProjectTagsSet":        true,
		"pendingProjectBulkTags":       true,
		"pendingProjectEditorSave":     true,
		"projectParkConfirm":           true,
		"projectTagActionSubject":      true,
	}
	assertAppModelClusterHasOneHome(t, "project", clustered)
}
