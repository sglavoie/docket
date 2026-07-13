package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// These tests cover the AppModel single-task mutation handlers migrated onto the
// application service (session 33.7.3.5.3): task content edit, description edit,
// dependency selection, single-task priority, and single-task section. Each now
// commits ONE ledger-undoable ChangeSet through app.Service rather than the
// legacy per-project .undo snapshot. The fixture drives the real transactional
// storage stack + workspace undo ledger, mirroring the migrated bulk/picker
// tests, and asserts the mutation→reload→undo→redo cycle against committed disk
// state.

// requireDirectMutationCommitted verifies the migrated handler committed: it was
// handled, returned a follow-up command (refresh batch, not a raw SaveCompleteMsg
// — the handler already processed the save synchronously), and left no error on
// the model.
func requireDirectMutationCommitted(t *testing.T, app *AppModel, handled bool, cmd interface{}) {
	t.Helper()
	if !handled {
		t.Fatal("handler did not report the message as handled")
	}
	if cmd == nil {
		t.Fatal("handler returned no follow-up command")
	}
	if app.err != nil {
		t.Fatalf("handler set an error: %v", app.err)
	}
}

func TestHandleTaskContentEditedCommitsAndUndoesTitle(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "Old title")
	env := newAppBulkPickerEnv(t, []*models.Task{task})
	app := env.appModel()
	project := env.projects[0]

	handled, cmd := app.handleTaskContentEdited(TaskContentEditedMsg{
		Task:           project.Tasks[0],
		Project:        project,
		NewTitle:       "New title",
		NewDescription: project.Tasks[0].Description,
	})
	requireDirectMutationCommitted(t, app, handled, cmd)

	if got := env.reload(t, 0).GetTaskByID(task.ID); got == nil || got.Title != "New title" {
		t.Fatalf("committed title = %+v, want %q", got, "New title")
	}

	env.undo(t)
	if got := env.reload(t, 0).GetTaskByID(task.ID); got == nil || got.Title != "Old title" {
		t.Fatalf("after undo title = %+v, want %q", got, "Old title")
	}

	env.redo(t)
	if got := env.reload(t, 0).GetTaskByID(task.ID); got == nil || got.Title != "New title" {
		t.Fatalf("after redo title = %+v, want %q", got, "New title")
	}
}

func TestHandleDescriptionEditorDoneCommitsAndUndoes(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "Task with description")
	task.Description = "old description"
	env := newAppBulkPickerEnv(t, []*models.Task{task})
	app := env.appModel()
	project := env.projects[0]

	handled, cmd := app.handleDescriptionEditorDone(DescriptionEditorDoneMsg{
		Task:           project.Tasks[0],
		Project:        project,
		NewDescription: "new description",
	})
	requireDirectMutationCommitted(t, app, handled, cmd)

	if got := env.reload(t, 0).GetTaskByID(task.ID); got == nil || got.Description != "new description" {
		t.Fatalf("committed description = %+v, want %q", got, "new description")
	}

	env.undo(t)
	if got := env.reload(t, 0).GetTaskByID(task.ID); got == nil || got.Description != "old description" {
		t.Fatalf("after undo description = %+v, want %q", got, "old description")
	}
}

func TestHandleDependencySelectorDoneCommitsAndUndoes(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "Blocked task")
	dependency := newBulkPickerTestTask(t, 2, "Dependency")
	env := newAppBulkPickerEnv(t, []*models.Task{task, dependency})
	app := env.appModel()
	app.pushView(ViewDependencySelector)
	project := env.projects[0]

	handled, cmd := app.handleDependencyMessages(DependencySelectorDoneMsg{
		Task:       project.Tasks[0],
		Project:    project,
		NewDepsOn:  []string{dependency.ID},
		HasChanges: true,
	})
	requireDirectMutationCommitted(t, app, handled, cmd)

	got := env.reload(t, 0).GetTaskByID(task.ID)
	if got == nil || len(got.DependsOn) != 1 || got.DependsOn[0] != dependency.ID {
		t.Fatalf("committed DependsOn = %+v, want [%q]", got, dependency.ID)
	}

	env.undo(t)
	if got := env.reload(t, 0).GetTaskByID(task.ID); got == nil || len(got.DependsOn) != 0 {
		t.Fatalf("after undo DependsOn = %+v, want empty", got)
	}

	env.redo(t)
	if got := env.reload(t, 0).GetTaskByID(task.ID); got == nil || len(got.DependsOn) != 1 {
		t.Fatalf("after redo DependsOn = %+v, want one dep", got)
	}
}

func TestHandlePriorityPickerDoneCommitsAndUndoes(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "Prioritized task")
	task.Priority = 4
	env := newAppBulkPickerEnv(t, []*models.Task{task})
	app := env.appModel()
	app.pushView(ViewPriorityPicker)
	project := env.projects[0]

	handled, cmd := app.handleSectionMessages(PriorityPickerDoneMsg{
		Task:     project.Tasks[0],
		Project:  project,
		Priority: 1,
	})
	requireDirectMutationCommitted(t, app, handled, cmd)

	if got := env.reload(t, 0).GetTaskByID(task.ID); got == nil || got.Priority != 1 {
		t.Fatalf("committed priority = %+v, want 1", got)
	}

	env.undo(t)
	if got := env.reload(t, 0).GetTaskByID(task.ID); got == nil || got.Priority != 4 {
		t.Fatalf("after undo priority = %+v, want 4", got)
	}
}

func TestHandleCommandPaletteTaskActionCommitsTransition(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "Palette action task")
	env := newAppBulkPickerEnv(t, []*models.Task{task})
	app := env.appModel()
	project := env.projects[0]

	handled, cmd := app.handleCommandPaletteTaskAction(CommandPaletteTaskActionMsg{
		Task:    project.Tasks[0],
		Project: project,
		Action:  "start",
	})
	requireDirectMutationCommitted(t, app, handled, cmd)

	if got := env.reload(t, 0).GetTaskByID(task.ID); got == nil || !got.IsInState(models.StateInProgress) {
		t.Fatalf("committed state = %+v, want in_progress", got)
	}

	env.undo(t)
	if got := env.reload(t, 0).GetTaskByID(task.ID); got == nil || got.IsInState(models.StateInProgress) {
		t.Fatalf("after undo state = %+v, want not in_progress", got)
	}
}

// TestHandleSectionPickerDoneBulkCommitsAsyncAndUndoes verifies the bulk
// section-picker branch (pendingBulkSectionTasks) now commits the already-mutated
// project through the workflow-aware coordinator instead of the raw
// persistWithWorkflow tier: the batch result carries the per-view summary and the
// selection to clear, every selected task's section is committed on disk, and the
// workspace ledger restores the prior state.
func TestHandleSectionPickerDoneBulkCommitsAsyncAndUndoes(t *testing.T) {
	taskA := newBulkPickerTestTask(t, 1, "Alpha")
	taskB := newBulkPickerTestTask(t, 2, "Beta")
	env := newAppBulkPickerEnv(t, []*models.Task{taskA, taskB})
	app := env.appModel()
	app.pushView(ViewSectionPicker)
	project := env.projects[0]

	app.pendingBulkSectionTasks = []*models.Task{project.Tasks[0], project.Tasks[1]}
	app.pendingBulkSectionProject = project

	handled, cmd := app.handleSectionMessages(SectionPickerDoneMsg{Section: models.SectionNext})
	if !handled {
		t.Fatal("bulk SectionPickerDoneMsg should be handled")
	}
	if app.pendingBulkSectionTasks != nil || app.pendingBulkSectionProject != nil {
		t.Fatal("pending bulk section state should be cleared once the batch is issued")
	}

	msg := runBatchResult(t, cmd)
	if msg.Summary != "Moved 2 tasks to Next" {
		t.Errorf("summary = %q, want %q", msg.Summary, "Moved 2 tasks to Next")
	}
	if len(msg.ClearSelectionTaskIDs) != 2 {
		t.Errorf("clear selection ids = %v, want 2", msg.ClearSelectionTaskIDs)
	}

	reloaded := env.reload(t, 0)
	for _, id := range []string{taskA.ID, taskB.ID} {
		if got := reloaded.GetTaskByID(id); got == nil || models.SectionForState(got.State()) != models.SectionNext {
			t.Fatalf("task %s section = %+v, want %q", id, got, models.SectionNext)
		}
	}

	env.undo(t)
	reloaded = env.reload(t, 0)
	for _, id := range []string{taskA.ID, taskB.ID} {
		if got := reloaded.GetTaskByID(id); got == nil || models.SectionForState(got.State()) == models.SectionNext {
			t.Fatalf("undo section for %s = %+v, want restored", id, got)
		}
	}
}

func TestHandleSectionPickerDoneCommitsAndUndoes(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "Section task")
	env := newAppBulkPickerEnv(t, []*models.Task{task})
	app := env.appModel()
	app.pushView(ViewSectionPicker)
	project := env.projects[0]

	handled, cmd := app.handleSectionMessages(SectionPickerDoneMsg{
		Task:    project.Tasks[0],
		Project: project,
		Section: models.SectionNext,
	})
	requireDirectMutationCommitted(t, app, handled, cmd)

	if got := env.reload(t, 0).GetTaskByID(task.ID); got == nil || models.SectionForState(got.State()) != models.SectionNext {
		t.Fatalf("committed section = %+v, want %q", got, models.SectionNext)
	}

	env.undo(t)
	if got := env.reload(t, 0).GetTaskByID(task.ID); got == nil || models.SectionForState(got.State()) != "" {
		t.Fatalf("after undo section = %+v, want empty", got)
	}
}
