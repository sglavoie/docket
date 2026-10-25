package tui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/coordinator"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// TestInboxAdvanceIgnoresFailedMutations records that a failed mutation cannot
// advance because withOutlinerRelocationCapture attaches the capture to
// successful results only; a TransactionErrorMsg passes through bare.
func TestInboxAdvanceIgnoresFailedMutations(t *testing.T) {
	one := primaryRowsTask("one", models.StateInbox)
	two := primaryRowsTask("two", models.StateInbox)
	alpha := primaryRowsProject("alpha", one, two)
	outlinerView := inboxModeOutliner(t, alpha)

	cursorToTaskRow(t, outlinerView, one.ID)
	before := outlinerView.Cursor
	capture := outlinerView.CaptureOutlinerRelocation([]taskref.TaskIdentity{taskref.For(one, alpha)})
	failure := coordinator.TransactionErrorMsg{Err: errors.New("commit refused")}
	wrapped := withOutlinerRelocationCapture(func() tea.Msg { return failure }, capture)
	if got, ok := wrapped().(coordinator.TransactionErrorMsg); !ok || got != failure {
		t.Fatalf("failed mutation = %#v, want the bare TransactionErrorMsg", wrapped())
	}
	if outlinerView.Cursor != before {
		t.Fatalf("cursor = %d after a failed mutation, want %d", outlinerView.Cursor, before)
	}
	requireCursorRow(t, outlinerView, one.ID)
}

func requireCursorTask(t *testing.T, outlinerView *outliner.OutlinerModel, want taskref.TaskIdentity) {
	t.Helper()
	if got := outlinerView.CurrentTaskIdentity(); got != want {
		t.Fatalf("cursor task = %+v, want %+v (cursor=%d)", got, want, outlinerView.Cursor)
	}
}

func requireCursorRow(t *testing.T, outlinerView *outliner.OutlinerModel, taskID string) {
	t.Helper()
	if outlinerView.Cursor < 0 || outlinerView.Cursor >= len(outlinerView.Rows) {
		t.Fatalf("cursor %d is outside %d rows", outlinerView.Cursor, len(outlinerView.Rows))
	}
	row := outlinerView.Rows[outlinerView.Cursor]
	if !row.IsTaskRow() || row.Task == nil {
		t.Fatalf("cursor landed on a non-task row %+v", row)
	}
	if row.Task.ID != taskID {
		t.Fatalf("cursor task = %q, want %q", row.Task.ID, taskID)
	}
}

func cursorToTaskRow(t *testing.T, outlinerView *outliner.OutlinerModel, taskID string) {
	t.Helper()
	for i, row := range outlinerView.Rows {
		if row.IsTaskRow() && row.Task != nil && row.Task.ID == taskID {
			outlinerView.Cursor = i
			return
		}
	}
	t.Fatalf("task %q has no task row", taskID)
}

// inboxModeOutliner builds the real Inbox row pipeline over the given
// projects. Every recovery test that wants the Inbox shape runs against these
// rows rather than a hand-rolled slice, so the [projectHeader][task...] group
// shape is the one the product renders.
func inboxModeOutliner(t *testing.T, projects ...*models.Project) *outliner.OutlinerModel {
	t.Helper()
	outlinerView, _ := primaryRowsOutliner(projects...)
	outlinerView.SelectOutlinerMode(chrome.ModeInbox)
	return outlinerView
}
