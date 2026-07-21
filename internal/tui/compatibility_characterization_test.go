package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestCompatibilityCharacterizationStackReturnBehavior(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.pushView(ViewOutliner)
	app.lastNavigatedTaskID = project.Tasks[0].ID
	app.pushView(ViewTaskDetail)

	if got := app.currentView(); got != ViewTaskDetail {
		t.Fatalf("current view = %s, want TaskDetail", got)
	}
	app.popView()
	if got := app.currentView(); got != ViewOutliner {
		t.Fatalf("return from TaskDetail = %s, want Outliner", got)
	}
	app.popView()
	if got := app.currentView(); got != ViewDashboard {
		t.Fatalf("return from Outliner = %s, want Dashboard", got)
	}
}

func TestCompatibilityCharacterizationSelectionIdentity(t *testing.T) {
	app, project := newTestAppModelWithProject()
	outliner := NewOutlinerModel(project, app.configManager, app.runtime)
	outliner.rebuildRows()
	target := project.Tasks[len(project.Tasks)-1]
	outliner.moveCursorToTask(target.ID)

	saved := outliner.SaveToggleState()
	if saved.CursorTaskID != target.ID {
		t.Fatalf("saved cursor identity = %q, want %q", saved.CursorTaskID, target.ID)
	}
	outliner.cursor = 0
	outliner.RestoreToggleState(saved)
	if selected := outliner.getCursorTask(); selected == nil || selected.ID != target.ID {
		got := "<nil>"
		if selected != nil {
			got = selected.ID
		}
		t.Fatalf("restored cursor identity = %q, want %q", got, target.ID)
	}
}

func TestCompatibilityCharacterizationPlanningMutationUndoRedo(t *testing.T) {
	app, _, task, _ := newPlanningLedgerApp(t, models.StateNext)
	app.seedDiskFocusQueue(t, task.ID, app.currentProject.ID)

	if command := app.toggleDailyPlan(); command == nil {
		t.Fatal("Next-to-Today planning mutation returned nil command")
	}
	assertExclusiveMembership(t, app, task.ID, false, true)
	if command := app.handleUndoToggle(); command == nil {
		t.Fatal("planning undo returned nil command")
	}
	assertExclusiveMembership(t, app, task.ID, true, false)
	if command := app.handleRedoToggle(); command == nil {
		t.Fatal("planning redo returned nil command")
	}
	assertExclusiveMembership(t, app, task.ID, false, true)
}
