package tui

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// referencePickerFixture is one task that already names a target in another
// project, with a second open task beside that target so the picker has both
// halves to offer: a stored row and an addition candidate.
func referencePickerFixture(t *testing.T) (*AppModel, *models.Task, *models.Project, *models.Project) {
	t.Helper()
	model := newUITestApp()
	target := &models.Task{ID: "target", ShortID: 1, Title: "Ship the API", WorkflowState: models.StateBacklog}
	other := &models.Task{ID: "other", ShortID: 2, Title: "Write the guide", WorkflowState: models.StateBacklog}
	remote := &models.Project{ID: "remote", Name: "Remote", Slug: "remote", Tasks: []*models.Task{target, other}}
	remote.BuildTaskTree()

	subject := &models.Task{
		ID:            "subject",
		ShortID:       1,
		Title:         "Cut the release",
		WorkflowState: models.StateBacklog,
		Dependencies:  []models.TaskReference{{ProjectID: remote.ID, TaskID: target.ID}},
	}
	home := &models.Project{ID: "home", Name: "Home", Slug: "home", Tasks: []*models.Task{subject}}
	home.BuildTaskTree()

	setTestAppProjects(model, home, remote)
	model.crossOutliner.loadedProjects = []*models.Project{home, remote}
	model.crossOutliner.rows = []outlinerRow{{task: subject, ProjectID: home.ID}}
	return model, subject, home, remote
}

// TestReferencePickerOpensOnTildeWithTheStoredSetChecked is the session's
// behavioral assertion: `~` on a task row reaches the picker through the real
// key path, and what the task already stores arrives checked, so unchecking a
// row is the removal the sigil's `-~task` form spells.
func TestReferencePickerOpensOnTildeWithTheStoredSetChecked(t *testing.T) {
	model, _, _, remote := referencePickerFixture(t)

	handled, cmd := model.handleGlobalKeyPress(tea.KeyPressMsg{Code: '~', Text: "~"})
	if !handled {
		t.Fatal("~ was not handled on a task row")
	}
	if cmd != nil {
		model.handleOperationOverlayMessage(cmd())
	}
	picker := model.operationValuePicker
	if picker == nil || picker.provider != ValueProviderTaskReference {
		t.Fatalf("~ did not open the reference picker: %#v", picker)
	}

	storedRow := taskReferenceRowID(app.TaskLocator{ProjectID: remote.ID, TaskID: "target"})
	candidateRow := taskReferenceRowID(app.TaskLocator{ProjectID: remote.ID, TaskID: "other"})
	ids := make([]ListItemID, 0, len(picker.surface.Items()))
	for _, item := range picker.surface.Items() {
		ids = append(ids, item.ID)
	}
	if !slices.Contains(ids, storedRow) {
		t.Fatalf("the picker has no row for the stored reference; rows = %v", ids)
	}
	if !slices.Contains(ids, candidateRow) {
		t.Fatalf("the picker offers no addition candidate; rows = %v", ids)
	}
	if selected := picker.surface.SelectedIDs(); !slices.Equal(selected, []ListItemID{storedRow}) {
		t.Errorf("checked rows = %v, want the stored reference alone", selected)
	}
}

// TestReferencePickerCommitsNothingWhenTheCheckedSetIsUnchanged is the other
// half of the wiring: leaving the preselection alone must not open an empty
// transaction, because that would still write a ledger entry `u` could take
// back for an edit that changed nothing.
func TestReferencePickerCommitsNothingWhenTheCheckedSetIsUnchanged(t *testing.T) {
	model, subject, home, remote := referencePickerFixture(t)

	stored := app.TaskLocator{ProjectID: remote.ID, TaskID: "target"}
	cmd := model.setTaskReferencesCmd(taskIdentityFor(subject, home), []string{stored.Reference().Locator()})
	if cmd == nil {
		t.Fatal("an unchanged reference set produced no command at all")
	}
	if msg, ok := cmd().(InfoStatusMsg); !ok {
		t.Fatalf("an unchanged reference set = %T, want a status notice and no edit", msg)
	}
}
