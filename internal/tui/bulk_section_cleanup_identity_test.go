package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestBulkSectionMoveCleanupKeepsCollidingCrossProjectSelection(t *testing.T) {
	sourceTask := newBulkPickerTestTask(t, 1, "Source task")
	otherTask := newBulkPickerTestTask(t, 1, "Other project task")
	otherTask.ID = sourceTask.ID
	env := newAppBulkPickerEnv(t, []*models.Task{sourceTask}, []*models.Task{otherTask})
	shell := env.appModel()
	sourceProject := env.projects[0]
	otherProject := env.projects[1]

	shell.pushView(ViewSectionPicker)
	shell.pendingBulkSectionTasks = []*models.Task{sourceProject.Tasks[0]}
	shell.pendingBulkSectionProject = sourceProject

	handled, cmd := shell.handleSectionMessages(SectionPickerDoneMsg{Section: models.SectionNext})
	if !handled {
		t.Fatal("bulk SectionPickerDoneMsg should be handled")
	}
	msg := runBatchResult(t, cmd)
	if len(msg.ClearSelectionTaskLocators) != 1 {
		t.Fatalf("clear-selection locators = %+v, want one", msg.ClearSelectionTaskLocators)
	}
	locator := msg.ClearSelectionTaskLocators[0]
	if locator.ProjectID != sourceProject.ID || locator.TaskID != sourceTask.ID {
		t.Fatalf("clear-selection locator = %+v, want source project/task", locator)
	}

	shell.crossOutliner = NewOutlinerModel(nil, shell.configManager, shell.runtime)
	shell.crossOutliner.loadedProjects = env.projects
	shell.crossOutliner.multiSelectMode = true
	sourceIdentity := taskIdentityFor(sourceTask, sourceProject)
	otherIdentity := taskIdentityFor(otherTask, otherProject)
	shell.crossOutliner.SelectTaskIdentity(sourceIdentity)
	shell.crossOutliner.SelectTaskIdentity(otherIdentity)

	shell.handleBatchTransactionResult(msg)

	if shell.crossOutliner.IsTaskIdentitySelected(sourceIdentity) {
		t.Fatal("moved source task should be removed from the cross-project selection")
	}
	if !shell.crossOutliner.IsTaskIdentitySelected(otherIdentity) {
		t.Fatal("same-UUID task in the other project should remain selected")
	}
	if !shell.crossOutliner.multiSelectMode || shell.crossOutliner.SelectedCount() != 1 {
		t.Fatalf("remaining selection state = active %t count %d, want active with one task",
			shell.crossOutliner.multiSelectMode, shell.crossOutliner.SelectedCount())
	}
}
