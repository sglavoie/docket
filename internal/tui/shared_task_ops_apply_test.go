package tui

import (
	"errors"
	"testing"
	"time"

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

func TestSimplePersist_AutoUnblocksAfterCompletion(t *testing.T) {
	project, err := models.NewProject("proj-1", "TestProject", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// Task A: will be completed
	taskA, err := models.NewTask("task-a", 1, models.TaskTypeTask, "Dep task")
	if err != nil {
		t.Fatal(err)
	}
	taskA.Status = models.StatusInProgress

	// Task B: blocked on task A
	taskB, err := models.NewTask("task-b", 2, models.TaskTypeTask, "Blocked task")
	if err != nil {
		t.Fatal(err)
	}
	taskB.Status = models.StatusBlocked
	taskB.DependsOn = []string{"task-a"}

	project.Tasks = []*models.Task{taskA, taskB}
	project.NextShortID = 3
	project.BuildTaskTree()

	mock := NewMockPersister()
	pf := simplePersist(mock)

	// Complete task A via simplePersist
	result := pf(project, "task-a", func(t *models.Task) {
		t.Complete()
	})
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	// Task B should have been auto-unblocked to todo
	if taskB.Status != models.StatusTodo {
		t.Errorf("expected task B to be auto-unblocked to %s, got %s", models.StatusTodo, taskB.Status)
	}
}

func TestSimplePersist_GeneratesRecurringInstances(t *testing.T) {
	project, err := models.NewProject("proj-1", "TestProject", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	task, err := models.NewTask("task-r", 1, models.TaskTypeTask, "Recurring task")
	if err != nil {
		t.Fatal(err)
	}
	task.Status = models.StatusInProgress
	task.Recurrence = &models.RecurrencePattern{
		Unit:     models.RecurrenceDaily,
		Interval: 1,
	}

	project.Tasks = []*models.Task{task}
	project.NextShortID = 2
	project.BuildTaskTree()

	mock := NewMockPersister()
	pf := simplePersist(mock)

	// Complete the recurring task via simplePersist
	result := pf(project, "task-r", func(t *models.Task) {
		t.Complete()
	})
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	// A new recurring instance should have been generated
	if len(project.Tasks) != 2 {
		t.Fatalf("expected 2 tasks (original + recurring instance), got %d", len(project.Tasks))
	}

	// The new instance should be todo with the same recurrence
	var newTask *models.Task
	for _, tt := range project.Tasks {
		if tt.ID != "task-r" {
			newTask = tt
			break
		}
	}
	if newTask == nil {
		t.Fatal("expected a new recurring task instance to be created")
	}
	if newTask.Status != models.StatusTodo {
		t.Errorf("expected new instance status %s, got %s", models.StatusTodo, newTask.Status)
	}
	if newTask.Recurrence == nil {
		t.Error("expected new instance to have recurrence set")
	}
}

func TestApplyBulkDueDate_ClearsHasTime(t *testing.T) {
	project, err := models.NewProject("proj-1", "TestProject", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	task1, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Task one")
	if err != nil {
		t.Fatal(err)
	}
	task1.HasTime = true // Simulate a task that previously had a time component
	task2, err := models.NewTask("task-2", 2, models.TaskTypeTask, "Task two")
	if err != nil {
		t.Fatal(err)
	}
	task2.HasTime = true
	project.Tasks = []*models.Task{task1, task2}
	project.BuildTaskTree()

	tomorrow := time.Now().AddDate(0, 0, 1)
	count := applyBulkDueDate(project, project.Tasks, &tomorrow)
	if count != 2 {
		t.Fatalf("expected 2 tasks changed, got %d", count)
	}

	for _, task := range project.Tasks {
		if task.HasTime {
			t.Errorf("task %s: HasTime should be false after bulk due date set", task.ID)
		}
		if task.DueDate == nil {
			t.Errorf("task %s: DueDate should be set", task.ID)
		}
	}
}

func TestApplyBulkDueDate_EachTaskOwnsDueDate(t *testing.T) {
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
	project.BuildTaskTree()

	tomorrow := time.Now().AddDate(0, 0, 1)
	applyBulkDueDate(project, project.Tasks, &tomorrow)

	// Each task should have its own pointer (not sharing the same one)
	if task1.DueDate == task2.DueDate {
		t.Error("tasks should not share the same DueDate pointer")
	}
	// But the values should be equal
	if !task1.DueDate.Equal(*task2.DueDate) {
		t.Error("tasks should have equal DueDate values")
	}
}

func TestApplyBulkDueDate_ClearsHasTimeOnNil(t *testing.T) {
	project, err := models.NewProject("proj-1", "TestProject", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Task one")
	if err != nil {
		t.Fatal(err)
	}
	task.HasTime = true
	tomorrow := time.Now().AddDate(0, 0, 1)
	task.DueDate = &tomorrow
	project.Tasks = []*models.Task{task}
	project.BuildTaskTree()

	count := applyBulkDueDate(project, project.Tasks, nil)
	if count != 1 {
		t.Fatalf("expected 1 task changed, got %d", count)
	}
	if task.DueDate != nil {
		t.Error("DueDate should be nil after clearing")
	}
	if task.HasTime {
		t.Error("HasTime should be false after clearing due date")
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

func passthroughPersist(project *models.Project) persistFn {
	return func(proj *models.Project, taskID string, mutate func(*models.Task)) TaskOpResult {
		if proj != project {
			return TaskOpResult{Err: errors.New("unexpected project")}
		}
		task := proj.GetTaskByID(taskID)
		if task == nil {
			return TaskOpResult{StatusMessage: "Task no longer exists in project"}
		}
		mutate(task)
		return TaskOpResult{}
	}
}

func nilPF(t *testing.T) persistFn {
	t.Helper()
	return func(proj *models.Project, taskID string, mutate func(*models.Task)) TaskOpResult {
		t.Fatal("persistFn must not be called when project is nil")
		return TaskOpResult{}
	}
}

func TestApplyStatusChange_NilProject(t *testing.T) {
	result := applyStatusChange(nilPF(t), nil, "task-1", models.StatusDone)
	if !errors.Is(result.Err, errNilProject) {
		t.Errorf("expected errNilProject, got %v", result.Err)
	}
}

func TestApplyPriorityChange_NilProject(t *testing.T) {
	result := applyPriorityChange(nilPF(t), nil, "task-1", 1)
	if !errors.Is(result.Err, errNilProject) {
		t.Errorf("expected errNilProject, got %v", result.Err)
	}
}

func TestApplyStageChange_NilProject(t *testing.T) {
	result := applyStageChange(nilPF(t), nil, "task-1", "planning")
	if !errors.Is(result.Err, errNilProject) {
		t.Errorf("expected errNilProject, got %v", result.Err)
	}
}

func TestApplyQuickDueDate_NilProject(t *testing.T) {
	result := applyQuickDueDate(nilPF(t), nil, "task-1", 1)
	if !errors.Is(result.Err, errNilProject) {
		t.Errorf("expected errNilProject, got %v", result.Err)
	}
}

func TestApplyClearDueDate_NilProject(t *testing.T) {
	result := applyClearDueDate(nilPF(t), nil, "task-1")
	if !errors.Is(result.Err, errNilProject) {
		t.Errorf("expected errNilProject, got %v", result.Err)
	}
}

func TestApplyQuickSnooze_NilProject(t *testing.T) {
	result := applyQuickSnooze(nilPF(t), nil, "task-1", nil)
	if !errors.Is(result.Err, errNilProject) {
		t.Errorf("expected errNilProject, got %v", result.Err)
	}
}

func TestApplyToggleBool_NilProject(t *testing.T) {
	result := applyToggleBool(nilPF(t), nil, "task-1", toggleActive)
	if !errors.Is(result.Err, errNilProject) {
		t.Errorf("expected errNilProject, got %v", result.Err)
	}
}

func TestApplyRemoveFromFocus_NilProject(t *testing.T) {
	result := applyRemoveFromFocus(nilPF(t), nil, "task-1", 1)
	if !errors.Is(result.Err, errNilProject) {
		t.Errorf("expected errNilProject, got %v", result.Err)
	}
}

func TestApplySectionChange_NilProject(t *testing.T) {
	result := applySectionChange(nilPF(t), nil, "task-1", "someSection")
	if !errors.Is(result.Err, errNilProject) {
		t.Errorf("expected errNilProject, got %v", result.Err)
	}
}

func TestApplyDeleteTask_NilProject(t *testing.T) {
	mock := NewMockPersister()
	result := applyDeleteTask(nilPF(t), mock, nil, "task-1")
	if !errors.Is(result.Err, errNilProject) {
		t.Errorf("expected errNilProject, got %v", result.Err)
	}
}
