package tui

import (
	"testing"
)

func TestChecklistReorderEdge(t *testing.T) {
	driver, ws, err := NewFixtureDriver(FixtureChecklist, 100, 35)
	if err != nil {
		t.Fatalf("NewFixtureDriver(checklist) error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	outliner := driver.App().crossOutliner
	focusChecklistOwner(t, driver, checklistFixtureFoldTitle)
	if !outliner.expandAtCursor() {
		t.Fatal("expandAtCursor did not expand the checklist")
	}
	outliner.Cursor++
	if !outliner.rows[outliner.Cursor].IsChecklistItem {
		t.Fatal("the cursor is not on the first checklist row")
	}

	pressChecklistEdgeKey(t, driver, "ctrl+j")
	assertOrder(t, "checklist after Ctrl+J", checklistRowTexts(outliner), []string{
		checklistFixtureSecondItem, checklistFixtureThirdItem, checklistFixtureFirstItem,
	})
	if !outliner.rows[outliner.Cursor].IsChecklistItem ||
		outliner.rows[outliner.Cursor].ChecklistIndex != 2 {
		t.Fatalf("cursor did not follow the moved item: %+v", outliner.rows[outliner.Cursor])
	}

	// The item is already last, so a second Ctrl+J changes nothing.
	pressChecklistEdgeKey(t, driver, "ctrl+j")
	assertOrder(t, "checklist after the no-op Ctrl+J", checklistRowTexts(outliner), []string{
		checklistFixtureSecondItem, checklistFixtureThirdItem, checklistFixtureFirstItem,
	})

	pressChecklistEdgeKey(t, driver, "ctrl+k")
	assertOrder(t, "checklist after Ctrl+K", checklistRowTexts(outliner), []string{
		checklistFixtureFirstItem, checklistFixtureSecondItem, checklistFixtureThirdItem,
	})
	if outliner.rows[outliner.Cursor].ChecklistIndex != 0 {
		t.Fatalf("cursor did not follow the item back to the top: %+v", outliner.rows[outliner.Cursor])
	}
}

// pressChecklistEdgeKey presses one key and settles the transaction it starts.
// A checklist move writes the project document, which routinely outlasts the
// driver's per-command drain budget and is abandoned to the pending set.
func pressChecklistEdgeKey(t *testing.T, driver *UITestDriver, key string) {
	t.Helper()
	sortPressKeys(t, driver, key)
	driver.Wait(defaultWaitTimeout)
}
