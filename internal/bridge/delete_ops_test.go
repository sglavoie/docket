package bridge

import (
	"slices"
	"testing"
)

// TestBridgeDeletesATaskSubtreeOnlyAgainstWhatItRead holds command.delete to
// its guard: a delete naming no read revision, or one read before the subtree
// grew, removes nothing; a confirmed delete removes the task and every
// descendant in one transaction its receipt undoes.
func TestBridgeDeletesATaskSubtreeOnlyAgainstWhatItRead(t *testing.T) {
	h := newHarness(t, representativeFixture)
	h.open(false)
	parent := task(atlasProject, darkIconsTask)
	h.ok(h.call("command.hierarchy", map[string]any{"task": parent, "action": "add_child", "title": "Draw the moon icon"}), nil)
	moon := h.childNamed(parent, "Draw the moon icon")
	read := h.detail(atlasProject, darkIconsTask)

	h.unchanged("a delete naming no read revision", func() {
		h.rejects(h.call("command.delete", map[string]any{"task": parent}), CodeInvalidRequest)
	})

	h.ok(h.call("command.hierarchy", map[string]any{"task": parent, "action": "add_child", "title": "Draw the sun icon"}), nil)
	sun := h.childNamed(parent, "Draw the sun icon")
	h.unchanged("a delete confirmed before the subtree grew", func() {
		h.refused(h.call("command.delete", map[string]any{"task": parent},
			withExpect(Expect{Projects: map[string]string{atlasProject: read.Revision}})), CodeStaleRevision)
	})

	current := h.detail(atlasProject, darkIconsTask)
	var deleted deleteResult
	h.ok(h.call("command.delete", map[string]any{"task": parent},
		withExpect(Expect{Projects: map[string]string{atlasProject: current.Revision}})), &deleted)
	want := []locator{parent, moon, sun}
	if len(deleted.Removed) != len(want) || deleted.Committed.TransactionID == "" || deleted.Committed.Projects[atlasProject] == "" {
		t.Fatalf("delete result %+v", deleted)
	}
	for _, target := range want {
		if !slices.Contains(deleted.Removed, target) {
			t.Fatalf("delete did not report removing %+v: %+v", target, deleted.Removed)
		}
		h.rejects(h.call("query.task", map[string]any{"task": target}), CodeNotFound)
	}

	h.ok(h.call("command.undo", map[string]any{"transaction_id": deleted.Committed.TransactionID}), nil)
	if got := relationTitles(h.detail(atlasProject, darkIconsTask).Relations.Children); len(got) != 2 {
		t.Fatalf("undo restored children %v", got)
	}
}
