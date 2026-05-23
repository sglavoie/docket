package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// TestCrossProjectBulkStatusSkipsInvalidTransitions verifies that a Done task
// included in a bulk "start" (InProgress) operation is not forcibly un-completed.
// Prior to the fix, ApplyTransition was called inside MutateTask without a
// CanTransitionTo guard, so a Done task would have its CompletedAt cleared and
// its status reverted to InProgress.
func TestCrossProjectBulkStatusSkipsInvalidTransitions(t *testing.T) {
	projDir := t.TempDir()

	doneTask, _ := models.NewTask("done-task-001", 1, models.TaskTypeTask, "Already done")
	completedAt := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	doneTask.Status = models.StatusDone
	doneTask.CompletedAt = &completedAt
	doneTask.UpdatedAt = completedAt

	todoTask, _ := models.NewTask("todo-task-001", 2, models.TaskTypeTask, "Not started yet")
	todoTask.Status = models.StatusTodo

	proj := &models.Project{ID: "test-proj", Name: "Test", Path: projDir}
	proj.Tasks = []*models.Task{doneTask, todoTask}
	proj.BuildTaskTree()

	if err := storage.SaveProject(proj); err != nil {
		t.Fatalf("save project: %v", err)
	}
	if err := storage.LoadProject(proj); err != nil {
		t.Fatalf("load project: %v", err)
	}

	doneRef := proj.GetTaskByID("done-task-001")
	todoRef := proj.GetTaskByID("todo-task-001")
	if doneRef == nil || todoRef == nil {
		t.Fatal("could not find tasks after reload")
	}

	m := &OutlinerModel{
		PrimaryViewBundle: PrimaryViewBundle{
			projectService: NewProjectService(),
			bulkOp:         NewBulkOperationComponent(),
			quickEdit:      NewQuickEditComponent(),
		},
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		activePerspective: 1,
		loadedProjects:    []*models.Project{proj},
	}

	// Simulate user selecting both tasks and issuing "start" (InProgress)
	m.bulkOp.SetOperation(models.StatusInProgress, []*models.Task{doneRef, todoRef}, 0, nil)
	resultModel, _ := m.crossExecutePendingBulkStatus()
	m = resultModel.(*OutlinerModel)

	// Load the saved state from disk to see what actually got persisted
	if err := storage.LoadProject(proj); err != nil {
		t.Fatalf("reload after bulk: %v", err)
	}

	doneFinal := proj.GetTaskByID("done-task-001")
	todoFinal := proj.GetTaskByID("todo-task-001")
	if doneFinal == nil || todoFinal == nil {
		t.Fatal("could not find tasks after bulk operation")
	}

	if doneFinal.Status != models.StatusDone {
		t.Errorf("done task should remain Done after bulk start, got %s", doneFinal.Status)
	}
	if doneFinal.CompletedAt == nil {
		t.Errorf("done task CompletedAt should be preserved, got nil")
	}
	if todoFinal.Status != models.StatusInProgress {
		t.Errorf("todo task should be InProgress after bulk start, got %s", todoFinal.Status)
	}

	_ = m
}
