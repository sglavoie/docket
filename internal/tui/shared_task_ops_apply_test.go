package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestApplyDeleteTask_PersistsWhenAllTasksDeleted(t *testing.T) {
	// Create a project with a single task
	project, err := models.NewProject("proj-1", "TestProject", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Only task")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = []*models.Task{task}
	project.NextShortID = 2
	project.BuildTaskTree()

	// Set up mock persister
	mock := NewMockPersister()

	// Create a no-op persistFn (simulates workflow persist but does nothing for
	// the mutation path — the important thing is the direct save path is tested)
	pf := func(proj *models.Project, taskID string, mutate func(*models.Task)) TaskOpResult {
		t.Fatal("persistFn should not be called when all tasks are deleted")
		return TaskOpResult{}
	}

	// Delete the only task
	result := applyDeleteTask(pf, mock, project, "task-1")

	// Verify no error
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	// Verify project has no tasks
	if len(project.Tasks) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(project.Tasks))
	}

	// Verify SaveWithBackupSync was called (the bug was that it wasn't)
	if len(mock.SaveWithBackupSyncCall) != 1 {
		t.Fatalf("expected SaveWithBackupSync to be called once, got %d calls", len(mock.SaveWithBackupSyncCall))
	}
	if mock.SaveWithBackupSyncCall[0].ID != "proj-1" {
		t.Errorf("expected SaveWithBackupSync to be called with project 'proj-1', got %q", mock.SaveWithBackupSyncCall[0].ID)
	}
}

func TestApplyDeleteTask_UsesWorkflowPersistWhenTasksRemain(t *testing.T) {
	// Create a project with two tasks
	project, err := models.NewProject("proj-1", "TestProject", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	task1, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Task one")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := models.NewTask("task-2", 2, models.TaskTypeTask, "Task two")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = []*models.Task{task1, task2}
	project.NextShortID = 3
	project.BuildTaskTree()

	// Set up mock persister
	mock := NewMockPersister()

	// Track whether persistFn was called
	pfCalled := false
	pf := func(proj *models.Project, taskID string, mutate func(*models.Task)) TaskOpResult {
		pfCalled = true
		return TaskOpResult{}
	}

	// Delete one task, leaving one remaining
	result := applyDeleteTask(pf, mock, project, "task-1")

	// Verify no error
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	// Verify persistFn was called (not the direct save path)
	if !pfCalled {
		t.Fatal("expected persistFn to be called when tasks remain")
	}

	// Verify SaveWithBackupSync was NOT called
	if len(mock.SaveWithBackupSyncCall) != 0 {
		t.Fatalf("expected SaveWithBackupSync not to be called, got %d calls", len(mock.SaveWithBackupSyncCall))
	}

	// Verify project has one task remaining
	if len(project.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(project.Tasks))
	}
	if project.Tasks[0].ID != "task-2" {
		t.Errorf("expected remaining task to be 'task-2', got %q", project.Tasks[0].ID)
	}
}

func TestApplyDeleteTask_ClearsDanglingRefsWhenAllDeleted(t *testing.T) {
	// Create a project with parent and child task where child depends on parent
	project, err := models.NewProject("proj-1", "TestProject", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	epic, err := models.NewTask("epic-1", 1, models.TaskTypeEpic, "Parent epic")
	if err != nil {
		t.Fatal(err)
	}
	task, err := models.NewTask("task-1", 2, models.TaskTypeTask, "Child task")
	if err != nil {
		t.Fatal(err)
	}
	task.ParentID = &epic.ID
	task.DependsOn = []string{"epic-1"}

	project.Tasks = []*models.Task{epic, task}
	project.NextShortID = 3
	project.BuildTaskTree()

	mock := NewMockPersister()
	pf := func(proj *models.Project, taskID string, mutate func(*models.Task)) TaskOpResult {
		return TaskOpResult{}
	}

	// Delete the epic (which should cascade to the child via collectDescendantIDs)
	result := applyDeleteTask(pf, mock, project, "epic-1")
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	// All tasks should be deleted (epic + child descendant)
	if len(project.Tasks) != 0 {
		t.Fatalf("expected 0 tasks after deleting epic with child, got %d", len(project.Tasks))
	}

	// SaveWithBackupSync should have been called since no tasks remain
	if len(mock.SaveWithBackupSyncCall) != 1 {
		t.Fatalf("expected SaveWithBackupSync to be called once, got %d", len(mock.SaveWithBackupSyncCall))
	}
}
