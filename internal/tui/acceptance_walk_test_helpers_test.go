package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/outliner"
)

// The helpers below are what every driver-backed acceptance walk in this
// package is built out of: launch the fixture, seat the cursor on a named row,
// replay a list of actions through the production driver, then read back what
// the canonical writer persisted. They started beside the checklist and comment
// acceptance tests and kept their names when the other comment surfaces began
// reading them, so they live here rather than behind a test file whose move
// would orphan their readers.

// The checklist fixture is the right one for `C` on a project: it carries a
// project group header, task rows, checklist item rows, and section headers, and
// it holds exactly one project, so the sole-scope fallback that
// outlinerProjectOperationTargetsHeader narrows has something to fall back onto
// and a leak would be visible rather than latent. It sits beside the helpers
// that spell it because every one of their readers names it too.
const projectCommentFixtureSlug = "checklist-delivery"

// addCommentFixtureApp launches the checklist fixture, which is the one that
// carries every row kind `C` has to answer from or refuse: task rows with and
// without children, checklist item rows, project group headers, and section
// headers. Every comment surface reached through the shell starts here, which is
// why it sits beside the walk helpers rather than beside the `C` tests it was
// written for.
func addCommentFixtureApp(t *testing.T) (*UITestDriver, *uitestWorkspace, *outliner.OutlinerModel) {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(FixtureChecklist, 100, 35)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	view := driver.App().crossOutliner
	if view == nil {
		t.Fatal("fixture app has no Outliner")
	}
	return driver, workspace, view
}

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

// seatCursorOnProjectHeader parks the cursor on the fixture's project header,
// which is the only row `C` may resolve a project from.
func seatCursorOnProjectHeader(t *testing.T, view *outliner.OutlinerModel) {
	t.Helper()
	seatCursorOnRow(t, view, "project header", func(row outliner.OutlinerRow) bool {
		return row.IsProjectHeader
	})
}

// projectCommentsOnDisk reads the comments the canonical writer actually
// persisted, rather than the ones the shell happens to be holding. A comment
// that never reached tasks.md is not a comment.
func projectCommentsOnDisk(t *testing.T, workspace *uitestWorkspace) []*models.ProjectComment {
	t.Helper()
	path := filepath.Join(workspace.DataDir, "projects", projectCommentFixtureSlug, "tasks.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	document, err := storage.ParseProjectDocument(string(content))
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return document.Comments
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
