package tui

import (
	"testing"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// expandChecklistOf opens one task's checklist and returns its owner identity,
// which is the setup every promotion needs: `S` answers from an item row, and an
// item row exists only while its owner's checklist is expanded.
func expandChecklistOf(t *testing.T, view *outliner.OutlinerModel, title string) taskref.TaskIdentity {
	t.Helper()
	seatCursorOnTaskTitled(t, view, title)
	identity := view.TaskIdentityForRow(view.Rows[view.Cursor])
	view.Folds.SetChecklist(identity, true)
	view.RebuildRows()
	return identity
}

// seatCursorOnChecklistItem parks the cursor on the item row carrying text.
// Walking the projection rather than counting rows keeps the assertion about the
// item the user is looking at instead of about how the fixture happens to sort.
func seatCursorOnChecklistItem(t *testing.T, view *outliner.OutlinerModel, text string) {
	t.Helper()
	for index, row := range view.Rows {
		if !row.IsChecklistItem || row.Task == nil {
			continue
		}
		if row.Task.Checklist[row.ChecklistIndex].Text == text {
			view.Cursor = index
			return
		}
	}
	t.Fatalf("no checklist item row carrying %q in the projection", text)
}

// workspaceTaskTitled is the committed child itself, read out of the reloaded
// workspace rather than out of the projection. A Done child is hidden by the
// default view, so a test that could only see rows could not tell a promotion
// that wrote nothing from one whose result the view declines to draw.
func workspaceTaskTitled(t *testing.T, app *AppModel, title string) (*models.Task, string) {
	t.Helper()
	for _, project := range app.domain.projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task != nil && task.Title == title {
				return task, project.ID
			}
		}
	}
	t.Fatalf("no task titled %q in the loaded workspace", title)
	return nil, ""
}
