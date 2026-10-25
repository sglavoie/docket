package tui

import (
	"testing"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/outliner"
)

// The menu bar is a projection of the operation catalog, so these are the
// parity assertions that keep it one: the bar may only name operations the
// registry holds, it may never spell a key the strategy owns, the modes it
// offers are the modes the catalog declares, and an operation the bar is the
// only route to is never an entity operation. A menu that drifts from the
// grammar fails here rather than in front of a user.
//
// This file holds the resolution's own unit tests; the fixtures and driver
// helpers the suite shares are in menu_bar_test_helpers_test.go. The parity
// rules are in menu_bar_parity_test.go and the open-menu sub-state in
// menu_bar_navigation_test.go.

// The specs carry live state, so a resolution that cached would show a stale
// checkmark the moment the mode changed under it.
func TestResolveMenuEntriesRebuildsOnEveryCall(t *testing.T) {
	app := parityAuditApp(t)

	inbox := menuWithID(t, resolveOutlinerMenus(t, app, chrome.ModeInbox), outliner.MenuIDView)
	recent := menuWithID(t, resolveOutlinerMenus(t, app, chrome.ModeRecent), outliner.MenuIDView)
	if len(inbox.Entries) != len(recent.Entries) {
		t.Fatalf("view menu changed shape between resolutions: %d then %d", len(inbox.Entries), len(recent.Entries))
	}
	for index, entry := range inbox.Entries {
		mode, ok := operation.GotoModeOperations[entry.OperationID]
		if !ok {
			continue
		}
		if got, want := *entry.Checked, mode == chrome.ModeInbox; got != want {
			t.Errorf("entry %d for mode %s checked=%v under Inbox, want %v", index, mode, got, want)
		}
		if got, want := *recent.Entries[index].Checked, mode == chrome.ModeRecent; got != want {
			t.Errorf("entry %d for mode %s checked=%v under Recent, want %v", index, mode, got, want)
		}
	}
}
