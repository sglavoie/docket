package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

// scenarioEditForm builds an edit task form for the given seeded task wired to
// the scenario task-form coordinator, so handleSave commits through the real
// transactional stack instead of a raw project save.
func scenarioEditForm(t *testing.T, env *scenarioEnv, task *models.Task) *TaskFormModel {
	t.Helper()
	cfg := newTestConfig()
	m := NewTaskFormModelEdit(env.project, task, testRuntimeAt(cfg.DataDir(), cfg))
	m.SetTaskFormCoordinator(env.features.TaskForm)
	return m
}

// runEdit executes handleSave and returns the committed TaskUpdatedMsg.
func runEdit(t *testing.T, m *TaskFormModel) TaskUpdatedMsg {
	t.Helper()
	cmd := m.handleSave()
	if cmd == nil {
		t.Fatalf("expected non-nil Cmd on successful edit; err=%v", m.err)
	}
	msg, ok := cmd().(TaskUpdatedMsg)
	if !ok {
		t.Fatalf("expected TaskUpdatedMsg, got %T", cmd())
	}
	if msg.Result == nil {
		t.Fatal("TaskUpdatedMsg.Result should be non-nil on the coordinator path")
	}
	return msg
}

// ── Edit happy path ─────────────────────────────────────────────────────────

func TestTaskForm_Save_EditHappyPath_CommitsThroughCoordinator(t *testing.T) {
	task := seedTask(t, 1, "Original", models.StateReady)
	env := newScenarioEnv(t, "edit", task)
	m := scenarioEditForm(t, env, task)

	m.draft.title = "Updated Title"
	m.titleInput.SetValue("Updated Title")
	m.draft.priority = 1
	m.draft.desc = "Updated description"

	msg := runEdit(t, m)
	if msg.Result.TaskID != task.ID {
		t.Errorf("Result.TaskID = %q, want %q", msg.Result.TaskID, task.ID)
	}

	got := env.reload(t).GetTaskByID(task.ID)
	if got == nil {
		t.Fatal("edited task absent from reloaded project")
	}
	if got.Title != "Updated Title" {
		t.Errorf("Title = %q, want Updated Title", got.Title)
	}
	if got.Priority != 1 {
		t.Errorf("Priority = %d, want 1", got.Priority)
	}
	if got.Description != "Updated description" {
		t.Errorf("Description = %q, want Updated description", got.Description)
	}
}

func TestTaskForm_Save_EditMutatesTaskAndUndoRestores(t *testing.T) {
	task := seedTask(t, 1, "Before", models.StateReady)
	task.Priority = 3
	env := newScenarioEnv(t, "edit-undo", task)
	m := scenarioEditForm(t, env, task)

	m.draft.title = "Mutated Title"
	m.titleInput.SetValue("Mutated Title")
	m.draft.priority = 5
	m.draft.tags = []string{"mutated"}

	runEdit(t, m)
	got := env.reload(t).GetTaskByID(task.ID)
	if got == nil || got.Title != "Mutated Title" || got.Priority != 5 {
		t.Fatalf("edit not persisted: %v", got)
	}
	if !got.HasTag("mutated") {
		t.Errorf("tags = %v, want mutated", got.Tags)
	}

	env.undo(t)
	restored := env.reload(t).GetTaskByID(task.ID)
	if restored == nil || restored.Title != "Before" || restored.Priority != 3 {
		t.Fatalf("undo did not restore the pre-edit task: %v", restored)
	}
}

func TestTaskForm_Save_EditFiltersBlankChecklistItems(t *testing.T) {
	task := seedTask(t, 1, "Task", models.StateReady)
	env := newScenarioEnv(t, "edit-checklist", task)
	m := scenarioEditForm(t, env, task)

	m.draft.title = "Edited task"
	m.titleInput.SetValue("Edited task")
	m.draft.checklist = []models.ChecklistItem{
		{Text: "keep me", Done: false},
		{Text: "   ", Done: false},
		{Text: "", Done: true},
		{Text: "also keep", Done: true},
	}

	runEdit(t, m)
	got := env.reload(t).GetTaskByID(task.ID)
	if got == nil {
		t.Fatal("edited task absent from reloaded project")
	}
	if len(got.Checklist) != 2 {
		t.Fatalf("expected 2 checklist items (blanks filtered), got %d", len(got.Checklist))
	}
	if got.Checklist[0].Text != "keep me" {
		t.Errorf("first item = %q, want keep me", got.Checklist[0].Text)
	}
	if got.Checklist[1].Text != "also keep" || !got.Checklist[1].Done {
		t.Errorf("second item = %+v, want {also keep, done}", got.Checklist[1])
	}
}

// TestTaskForm_Save_EditWithoutCoordinatorBuildsCmd guards that an edit form with
// no coordinator wired still builds a command without panicking; only executing
// it needs the coordinator.
func TestTaskForm_Save_EditWithoutCoordinatorBuildsCmd(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Edit")
	cfg := newTestConfig()
	if len(project.Tasks) == 0 {
		t.Fatal("test project has no tasks")
	}
	m := NewTaskFormModelEdit(project, project.Tasks[0], testRuntimeAt(cfg.DataDir(), cfg))
	m.draft.title = "changed"
	m.titleInput.SetValue("changed")
	if cmd := m.saveEdit(); cmd == nil {
		t.Fatal("saveEdit should build a command even without a coordinator")
	}
}
