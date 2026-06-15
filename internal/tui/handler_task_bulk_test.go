package tui

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
)

// TestCommandPaletteBulkActionSkipsInvalidTransitions verifies that the bulk
// "complete" action does not mutate tasks that cannot transition to Done.
// A Done task selected alongside a Todo task must remain Done with its
// CompletedAt preserved and UpdatedAt untouched; only the Todo task should
// transition.
func TestCommandPaletteBulkActionSkipsInvalidTransitions(t *testing.T) {
	app := newTestAppModel()

	project, err := models.NewProject(uuid.New().String(), "Test", "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	todoTask, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "todo task")
	if err != nil {
		t.Fatalf("NewTask todo: %v", err)
	}
	todoTask.Status = models.StatusTodo

	doneTask, err := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "done task")
	if err != nil {
		t.Fatalf("NewTask done: %v", err)
	}
	completedAt := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	doneTask.Status = models.StatusDone
	doneTask.CompletedAt = &completedAt
	doneTask.UpdatedAt = completedAt

	project.Tasks = []*models.Task{todoTask, doneTask}

	msg := CommandPaletteBulkActionMsg{
		Action: "complete",
		Tasks: []TaskProjectRef{
			{Task: todoTask, Project: project},
			{Task: doneTask, Project: project},
		},
	}

	handled, _ := app.handleCommandPaletteBulkAction(msg)
	if !handled {
		t.Fatal("expected handled=true")
	}

	if todoTask.Status != models.StatusDone {
		t.Errorf("todo task should be Done, got %s", todoTask.Status)
	}
	if doneTask.Status != models.StatusDone {
		t.Errorf("done task should remain Done, got %s", doneTask.Status)
	}
	if doneTask.CompletedAt == nil || !doneTask.CompletedAt.Equal(completedAt) {
		t.Errorf("done task CompletedAt should be preserved, got %v", doneTask.CompletedAt)
	}
	if !doneTask.UpdatedAt.Equal(completedAt) {
		t.Errorf("done task UpdatedAt should not be bumped for a no-op, got %v (want %v)", doneTask.UpdatedAt, completedAt)
	}
}
