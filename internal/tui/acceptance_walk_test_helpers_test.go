package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/tui/outliner"
)

// The two helpers below are what every driver-backed acceptance walk in this
// package is built out of: seat the cursor on a named row, then replay a list of
// actions through the production driver. They started beside the checklist
// acceptance tests and kept their names when the comment surfaces began reading
// them, so they live here rather than behind a test file whose move would
// orphan five readers.

// seatCursorOnTaskTitled parks the Outliner cursor on the task row naming title.
// Walking the real projection rather than counting key presses is what keeps the
// assertion about the row kind instead of about how many `j`s the fixture
// happens to need.
func seatCursorOnTaskTitled(t *testing.T, view *outliner.OutlinerModel, title string) {
	t.Helper()
	for index, row := range view.Rows {
		if row.IsTaskRow() && row.Task != nil && row.Task.Title == title {
			view.Cursor = index
			return
		}
	}
	t.Fatalf("no task row titled %q in the projection", title)
}

// runChecklistActions replays actions through the production driver and fails
// the test on the first one the app refuses to accept.
func runChecklistActions(t *testing.T, driver *UITestDriver, actions []UITestAction) {
	t.Helper()
	for index, action := range actions {
		if _, err := driver.ApplyAction(action, ObserveOptions{}); err != nil {
			t.Fatalf("action %d (%s %q): %v", index, action.Type, action.Key+action.Text, err)
		}
	}
}
