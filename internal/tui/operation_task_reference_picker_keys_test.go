package tui

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// This file covers the `~` key path itself: which cursors it refuses, what it
// opens over a visual selection, and what an unchanged commit does. The
// difference a commit spells is in
// operation_task_reference_picker_diff_test.go.

// TestReferencePickerRefusesOffATaskRow keeps `~` inside the task grammar: a
// project header names no task, so it refuses rather than falling back to
// whatever the cursor happens to be on. A visual selection is the other case and
// no longer a refusal — it edits every selected task, which
// TestReferencePickerEditsAWholeSelectionTriState holds.
func TestReferencePickerRefusesOffATaskRow(t *testing.T) {
	descriptorFor := func(t *testing.T, model *AppModel) OperationDescriptor {
		t.Helper()
		descriptor, ok := model.operationRegistry.Descriptor(operationTaskReferences)
		if !ok {
			t.Fatal("tui.task.references is not registered")
		}
		return descriptor
	}

	t.Run("a project header row resolves no target", func(t *testing.T) {
		model, _, home, _ := referencePickerFixture(t)
		model.crossOutliner.rows = append(
			[]outlinerRow{{IsProjectHeader: true, ProjectID: home.ID, SectionName: home.Name}},
			model.crossOutliner.rows...,
		)
		model.crossOutliner.cursor = 0

		ctx := model.operationCommandContext()
		if !ctx.Entity.HasProjectHeaderCursor || ctx.Entity.TaskID != "" {
			t.Fatalf("the fixture cursor is not on a project header row: %+v", ctx.Entity)
		}
		descriptor := descriptorFor(t, model)
		if _, err := (OperationTargetResolver{}).Resolve(descriptor, ctx); err == nil {
			t.Fatal("~ resolved a target from a project header cursor")
		} else if err.Error() != operationTargetRejectionMessage(OperationCategoryTask) {
			t.Errorf("rejection = %q, want the shared sentence", err)
		}

		handled, cmd := model.handleGlobalKeyPress(tea.KeyPressMsg{Code: '~', Text: "~"})
		if cmd != nil {
			model.handleOperationOverlayMessage(cmd())
		}
		if model.operationValuePicker != nil {
			t.Fatalf("~ opened a picker from a project header row (handled = %v)", handled)
		}
	})

	t.Run("a visual selection is the selection, never the cursor", func(t *testing.T) {
		model, subject, home, _ := referencePickerFixture(t)
		model.crossOutliner.ReplaceSelection([]TaskIdentity{taskIdentityFor(subject, home)})

		ctx := model.operationCommandContext()
		if !ctx.Selection.Visual {
			t.Fatal("the fixture did not enter a visual selection")
		}
		if !descriptorFor(t, model).Applicable(ctx) {
			t.Fatal("~ stopped applying under a visual selection")
		}
		targets, err := (OperationTargetResolver{}).Resolve(descriptorFor(t, model), ctx)
		if err != nil {
			t.Fatalf("~ resolved no target under a visual selection: %v", err)
		}
		if len(targets.Targets) != 1 || targets.Targets[0].Task.TaskID != subject.ID {
			t.Fatalf("targets = %+v, want the selected task alone", targets.Targets)
		}
	})
}

// TestReferencePickerEditsAWholeSelectionTriState is the bulk contract. The
// documented objection to bulk here — N tasks carrying N different stored sets —
// is an objection to a replacement picker, so the answer is the tri-state one
// tags already use: a reference everybody stores opens checked, one only part of
// the selection stores opens indeterminate and belongs to neither half of the
// emitted patch, and a selected task is never offered as a target of another.
func TestReferencePickerEditsAWholeSelectionTriState(t *testing.T) {
	model, subject, home, remote := referencePickerFixture(t)
	second := &models.Task{
		ID: "second", ShortID: 2, Title: "Draft the notes", WorkflowState: models.StateBacklog,
		Dependencies: []models.TaskReference{
			{ProjectID: remote.ID, TaskID: "target"},
			{ProjectID: remote.ID, TaskID: "other"},
		},
	}
	home.Tasks = append(home.Tasks, second)
	home.BuildTaskTree()
	setTestAppProjects(model, home, remote)
	model.crossOutliner.loadedProjects = []*models.Project{home, remote}
	model.crossOutliner.rows = []outlinerRow{
		{task: subject, ProjectID: home.ID},
		{task: second, ProjectID: home.ID},
	}
	model.crossOutliner.ReplaceSelection([]TaskIdentity{
		taskIdentityFor(subject, home), taskIdentityFor(second, home),
	})

	_, cmd := model.handleGlobalKeyPress(tea.KeyPressMsg{Code: '~', Text: "~"})
	if cmd != nil {
		model.handleOperationOverlayMessage(cmd())
	}
	picker := model.operationValuePicker
	if picker == nil || picker.provider != ValueProviderTaskReference {
		t.Fatalf("~ did not open the reference picker over a selection: %#v", picker)
	}

	shared := taskReferenceRowID(app.TaskLocator{ProjectID: remote.ID, TaskID: "target"})
	partial := taskReferenceRowID(app.TaskLocator{ProjectID: remote.ID, TaskID: "other"})
	if got := picker.membershipInitial[shared]; got != operationValueAll {
		t.Errorf("the reference both tasks store opened at %v, want checked", got)
	}
	if got := picker.membershipInitial[partial]; got != operationValueSome {
		t.Errorf("the reference one task stores opened at %v, want indeterminate", got)
	}
	for _, item := range picker.surface.Items() {
		if item.ID == partial && !item.Indeterminate {
			t.Error("the row only part of the selection stores is not drawn indeterminate")
		}
		if locator, ok := taskReferenceRowLocator(item.ID); ok && locator.TaskID == second.ID {
			t.Errorf("the picker offered %s, a task inside the selection", item.ID)
		}
	}

	// Touching nothing but the shared row leaves the row the tasks disagree about
	// out of both halves, which is what keeps them disagreeing.
	if !picker.surface.SetCursorToID(shared) {
		t.Fatalf("the picker has no row for %s", shared)
	}
	if err := picker.toggleCurrentMultiValue(); err != nil {
		t.Fatalf("toggling the shared row: %v", err)
	}
	value, err := picker.selectedValue()
	if err != nil {
		t.Fatalf("selectedValue: %v", err)
	}
	selection, ok := value.(OperationTaskReferenceValue)
	if !ok {
		t.Fatalf("the picker emitted %T, want a reference difference", value)
	}
	if len(selection.Add) != 0 {
		t.Errorf("Add = %v, want nothing added", selection.Add)
	}
	if !slices.Equal(selection.Remove, []string{"remote:target"}) {
		t.Errorf("Remove = %v, want the unchecked shared reference alone", selection.Remove)
	}
	patch := taskReferenceBulkPatch(selection)
	if len(patch.Add) != 0 || len(patch.Remove) != 1 || patch.Clear {
		t.Fatalf("bulk patch = %+v, want one removal and nothing else", patch)
	}
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
	initial := taskReferenceMembership([]*models.Task{subject})
	value := taskReferenceSelection(
		initial, referenceMembershipAfter(initial, []string{stored.Reference().Locator()}),
	)
	if value.Add != nil || value.Remove != nil {
		t.Fatalf("leaving the preselection alone emitted %+v, want no intent at all", value)
	}
	cmd := model.setTaskReferencesCmd(taskIdentityFor(subject, home), value)
	if cmd == nil {
		t.Fatal("an unchanged reference set produced no command at all")
	}
	if msg, ok := cmd().(InfoStatusMsg); !ok {
		t.Fatalf("an unchanged reference set = %T, want a status notice and no edit", msg)
	}
}
