package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

// resetFilterVars resets all filter package variables to their zero values.
func resetFilterVars() {
	listTaskType = ""
	listTaskStatus = ""
	listPriority = 0
	listTag = ""
	listShowAll = false
	listTreeView = false
}

func TestFilterTasks_EmptyTaskList(t *testing.T) {
	defer resetFilterVars()

	tasks := []*models.Task{}
	result := filterTasks(tasks)

	// Empty input returns nil slice (idiomatic Go)
	if len(result) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(result))
	}
}

func TestFilterTasks_NoFilters(t *testing.T) {
	defer resetFilterVars()

	tasks := []*models.Task{
		testutil.NewTestTaskWithStatus("Task 1", models.StatusTodo),
		testutil.NewTestTaskWithStatus("Task 2", models.StatusInProgress),
		testutil.NewTestTaskWithStatus("Task 3", models.StatusDone),
	}

	result := filterTasks(tasks)

	// Without --all, completed tasks are filtered out
	if len(result) != 2 {
		t.Errorf("expected 2 active tasks, got %d", len(result))
	}
}

func TestFilterTasks_ShowAll(t *testing.T) {
	defer resetFilterVars()
	listShowAll = true

	tasks := []*models.Task{
		testutil.NewTestTaskWithStatus("Task 1", models.StatusTodo),
		testutil.NewTestTaskWithStatus("Task 2", models.StatusDone),
		testutil.NewTestTaskWithStatus("Task 3", models.StatusCanceled),
	}

	result := filterTasks(tasks)

	if len(result) != 3 {
		t.Errorf("expected all 3 tasks with --all flag, got %d", len(result))
	}
}

func TestFilterTasks_ByType(t *testing.T) {
	tests := []struct {
		name         string
		taskType     string
		expectedType models.TaskType
	}{
		{"epic", "epic", models.TaskTypeEpic},
		{"story", "story", models.TaskTypeStory},
		{"task", "task", models.TaskTypeTask},
		{"bug", "bug", models.TaskTypeBug},
		{"idea", "idea", models.TaskTypeIdea},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer resetFilterVars()
			listShowAll = true // Include completed so we get consistent counts
			listTaskType = tc.taskType

			tasks := []*models.Task{
				testutil.NewTestTask("Epic task", models.TaskTypeEpic),
				testutil.NewTestTask("Story task", models.TaskTypeStory),
				testutil.NewTestTask("Regular task", models.TaskTypeTask),
				testutil.NewTestTask("Bug task", models.TaskTypeBug),
				testutil.NewTestTask("Idea task", models.TaskTypeIdea),
			}

			result := filterTasks(tasks)

			if len(result) != 1 {
				t.Errorf("expected 1 task of type %s, got %d", tc.taskType, len(result))
			}
			if len(result) > 0 && result[0].Type != tc.expectedType {
				t.Errorf("expected task type %s, got %s", tc.expectedType, result[0].Type)
			}
		})
	}
}

func TestFilterTasks_ByStatus(t *testing.T) {
	tests := []struct {
		name           string
		status         string
		expectedStatus models.TaskStatus
	}{
		{"todo", "todo", models.StatusTodo},
		{"in_progress", "in_progress", models.StatusInProgress},
		{"done", "done", models.StatusDone},
		{"blocked", "blocked", models.StatusBlocked},
		{"canceled", "canceled", models.StatusCanceled},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer resetFilterVars()
			listShowAll = true // Include completed to test done/canceled filters
			listTaskStatus = tc.status

			tasks := []*models.Task{
				testutil.NewTestTaskWithStatus("Todo task", models.StatusTodo),
				testutil.NewTestTaskWithStatus("In Progress task", models.StatusInProgress),
				testutil.NewTestTaskWithStatus("Done task", models.StatusDone),
				testutil.NewTestTaskWithStatus("Blocked task", models.StatusBlocked),
				testutil.NewTestTaskWithStatus("Canceled task", models.StatusCanceled),
			}

			result := filterTasks(tasks)

			if len(result) != 1 {
				t.Errorf("expected 1 task with status %s, got %d", tc.status, len(result))
			}
			if len(result) > 0 && result[0].Status != tc.expectedStatus {
				t.Errorf("expected status %s, got %s", tc.expectedStatus, result[0].Status)
			}
		})
	}
}

func TestFilterTasks_ByPriority(t *testing.T) {
	defer resetFilterVars()
	listShowAll = true
	listPriority = 2

	tasks := []*models.Task{
		testutil.NewTestTaskWithPriority("Priority 1 task", 1),
		testutil.NewTestTaskWithPriority("Priority 2 task", 2),
		testutil.NewTestTaskWithPriority("Priority 3 task", 3),
		testutil.NewTestTaskWithPriority("Another priority 2 task", 2),
	}

	result := filterTasks(tasks)

	if len(result) != 2 {
		t.Errorf("expected 2 tasks with priority 2, got %d", len(result))
	}
	for _, task := range result {
		if task.Priority != 2 {
			t.Errorf("expected priority 2, got %d for task %s", task.Priority, task.Title)
		}
	}
}

func TestFilterTasks_ByTag(t *testing.T) {
	defer resetFilterVars()
	listShowAll = true
	listTag = "backend"

	task1 := testutil.NewTestTask("Task with backend tag", models.TaskTypeTask)
	task1.Tags = []string{"backend", "api"}

	task2 := testutil.NewTestTask("Task with frontend tag", models.TaskTypeTask)
	task2.Tags = []string{"frontend", "ui"}

	task3 := testutil.NewTestTask("Task with both tags", models.TaskTypeTask)
	task3.Tags = []string{"backend", "frontend"}

	tasks := []*models.Task{task1, task2, task3}

	result := filterTasks(tasks)

	if len(result) != 2 {
		t.Errorf("expected 2 tasks with 'backend' tag, got %d", len(result))
	}
}

func TestFilterTasks_CombinedFilters_TypeAndStatus(t *testing.T) {
	defer resetFilterVars()
	listShowAll = true
	listTaskType = "task"
	listTaskStatus = "todo"

	tasks := []*models.Task{
		func() *models.Task {
			task := testutil.NewTestTask("Todo task", models.TaskTypeTask)
			task.Status = models.StatusTodo
			return task
		}(),
		func() *models.Task {
			task := testutil.NewTestTask("InProgress task", models.TaskTypeTask)
			task.Status = models.StatusInProgress
			return task
		}(),
		func() *models.Task {
			task := testutil.NewTestTask("Todo bug", models.TaskTypeBug)
			task.Status = models.StatusTodo
			return task
		}(),
	}

	result := filterTasks(tasks)

	if len(result) != 1 {
		t.Errorf("expected 1 task matching type=task AND status=todo, got %d", len(result))
	}
	if len(result) > 0 {
		if result[0].Type != models.TaskTypeTask {
			t.Errorf("expected type task, got %s", result[0].Type)
		}
		if result[0].Status != models.StatusTodo {
			t.Errorf("expected status todo, got %s", result[0].Status)
		}
	}
}

func TestFilterTasks_CombinedFilters_PriorityAndTag(t *testing.T) {
	defer resetFilterVars()
	listShowAll = true
	listPriority = 1
	listTag = "urgent"

	task1 := testutil.NewTestTaskWithPriority("Urgent p1", 1)
	task1.Tags = []string{"urgent"}

	task2 := testutil.NewTestTaskWithPriority("Normal p1", 1)
	task2.Tags = []string{"normal"}

	task3 := testutil.NewTestTaskWithPriority("Urgent p2", 2)
	task3.Tags = []string{"urgent"}

	tasks := []*models.Task{task1, task2, task3}

	result := filterTasks(tasks)

	if len(result) != 1 {
		t.Errorf("expected 1 task matching priority=1 AND tag=urgent, got %d", len(result))
	}
	if len(result) > 0 && result[0].Title != "Urgent p1" {
		t.Errorf("expected 'Urgent p1', got '%s'", result[0].Title)
	}
}

func TestFilterTasks_InvalidType(t *testing.T) {
	defer resetFilterVars()
	listShowAll = true
	listTaskType = "invalid_type"

	tasks := []*models.Task{
		testutil.NewTestTask("Task 1", models.TaskTypeTask),
		testutil.NewTestTask("Task 2", models.TaskTypeTask),
	}

	result := filterTasks(tasks)

	// Invalid type should filter out all tasks because ParseTaskType returns ok=false
	if len(result) != 0 {
		t.Errorf("expected 0 tasks with invalid type, got %d", len(result))
	}
}

func TestFilterTasks_InvalidStatus(t *testing.T) {
	defer resetFilterVars()
	listShowAll = true
	listTaskStatus = "invalid_status"

	tasks := []*models.Task{
		testutil.NewTestTaskWithStatus("Task 1", models.StatusTodo),
		testutil.NewTestTaskWithStatus("Task 2", models.StatusInProgress),
	}

	result := filterTasks(tasks)

	// Invalid status should filter out all tasks because ParseTaskStatus returns ok=false
	if len(result) != 0 {
		t.Errorf("expected 0 tasks with invalid status, got %d", len(result))
	}
}

func TestFilterTasks_CompletedTasksExcludedByDefault(t *testing.T) {
	defer resetFilterVars()
	// listShowAll defaults to false

	tasks := []*models.Task{
		testutil.NewTestTaskWithStatus("Todo", models.StatusTodo),
		testutil.NewTestTaskWithStatus("InProgress", models.StatusInProgress),
		testutil.NewTestTaskWithStatus("Blocked", models.StatusBlocked),
		testutil.NewTestTaskWithStatus("Done", models.StatusDone),
		testutil.NewTestTaskWithStatus("Canceled", models.StatusCanceled),
	}

	result := filterTasks(tasks)

	// Only active tasks (todo, in_progress, blocked) should be included
	if len(result) != 3 {
		t.Errorf("expected 3 active tasks, got %d", len(result))
	}

	for _, task := range result {
		if task.Status.IsComplete() {
			t.Errorf("completed task %s should not be in results", task.Title)
		}
	}
}

func TestFilterTasks_AllFiltersApplied(t *testing.T) {
	defer resetFilterVars()
	listShowAll = true
	listTaskType = "bug"
	listTaskStatus = "in_progress"
	listPriority = 1
	listTag = "critical"

	// Only this task matches all criteria
	matchingTask := func() *models.Task {
		task := testutil.NewTestTask("Critical bug", models.TaskTypeBug)
		task.Status = models.StatusInProgress
		task.Priority = 1
		task.Tags = []string{"critical", "backend"}
		return task
	}()

	// These don't match all criteria
	wrongType := func() *models.Task {
		task := testutil.NewTestTask("Critical task", models.TaskTypeTask)
		task.Status = models.StatusInProgress
		task.Priority = 1
		task.Tags = []string{"critical"}
		return task
	}()

	wrongStatus := func() *models.Task {
		task := testutil.NewTestTask("Todo bug", models.TaskTypeBug)
		task.Status = models.StatusTodo
		task.Priority = 1
		task.Tags = []string{"critical"}
		return task
	}()

	wrongPriority := func() *models.Task {
		task := testutil.NewTestTask("P2 bug", models.TaskTypeBug)
		task.Status = models.StatusInProgress
		task.Priority = 2
		task.Tags = []string{"critical"}
		return task
	}()

	missingTag := func() *models.Task {
		task := testutil.NewTestTask("Non-critical bug", models.TaskTypeBug)
		task.Status = models.StatusInProgress
		task.Priority = 1
		task.Tags = []string{"minor"}
		return task
	}()

	tasks := []*models.Task{matchingTask, wrongType, wrongStatus, wrongPriority, missingTag}

	result := filterTasks(tasks)

	if len(result) != 1 {
		t.Errorf("expected 1 task matching all filters, got %d", len(result))
	}
	if len(result) > 0 && result[0].Title != "Critical bug" {
		t.Errorf("expected 'Critical bug', got '%s'", result[0].Title)
	}
}

func TestFilterTasks_ZeroPriorityIgnored(t *testing.T) {
	defer resetFilterVars()
	listShowAll = true
	listPriority = 0 // Zero means "no priority filter"

	tasks := []*models.Task{
		testutil.NewTestTaskWithPriority("P1 task", 1),
		testutil.NewTestTaskWithPriority("P2 task", 2),
		testutil.NewTestTaskWithPriority("P0 task", 0),
	}

	result := filterTasks(tasks)

	// All tasks should be included when priority filter is 0
	if len(result) != 3 {
		t.Errorf("expected all 3 tasks when priority=0, got %d", len(result))
	}
}
