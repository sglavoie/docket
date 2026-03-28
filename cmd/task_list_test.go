package cmd

import (
	"testing"
	"time"

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
	listSnoozed = false
	listSort = ""
	listJSON = false
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
		{"review", "review", models.StatusReview},
		{"standby", "standby", models.StatusStandby},
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
				testutil.NewTestTaskWithStatus("Review task", models.StatusReview),
				testutil.NewTestTaskWithStatus("Standby task", models.StatusStandby),
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
		testutil.NewTestTaskWithStatus("Review", models.StatusReview),
		testutil.NewTestTaskWithStatus("Standby", models.StatusStandby),
		testutil.NewTestTaskWithStatus("Blocked", models.StatusBlocked),
		testutil.NewTestTaskWithStatus("Done", models.StatusDone),
		testutil.NewTestTaskWithStatus("Canceled", models.StatusCanceled),
	}

	result := filterTasks(tasks)

	// Only active tasks (todo, in_progress, review, standby, blocked) should be included
	if len(result) != 5 {
		t.Errorf("expected 5 active tasks, got %d", len(result))
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

func TestSortTasks_ByPriority(t *testing.T) {
	tasks := []*models.Task{
		testutil.NewTestTaskWithPriority("Low", 3),
		testutil.NewTestTaskWithPriority("High", 1),
		testutil.NewTestTaskWithPriority("Medium", 2),
	}

	err := sortTasks(tasks, "priority")
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, tasks[0].Title, "High")
	testutil.AssertEqual(t, tasks[1].Title, "Medium")
	testutil.AssertEqual(t, tasks[2].Title, "Low")
}

func TestSortTasks_ByCreated(t *testing.T) {
	now := time.Now()

	t1 := testutil.NewTestTask("Oldest", models.TaskTypeTask)
	t1.CreatedAt = now.Add(-2 * time.Hour)

	t2 := testutil.NewTestTask("Newest", models.TaskTypeTask)
	t2.CreatedAt = now

	t3 := testutil.NewTestTask("Middle", models.TaskTypeTask)
	t3.CreatedAt = now.Add(-1 * time.Hour)

	tasks := []*models.Task{t1, t2, t3}

	err := sortTasks(tasks, "created")
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, tasks[0].Title, "Newest")
	testutil.AssertEqual(t, tasks[1].Title, "Middle")
	testutil.AssertEqual(t, tasks[2].Title, "Oldest")
}

func TestSortTasks_ByUpdated(t *testing.T) {
	now := time.Now()

	t1 := testutil.NewTestTask("Oldest", models.TaskTypeTask)
	t1.UpdatedAt = now.Add(-2 * time.Hour)

	t2 := testutil.NewTestTask("Newest", models.TaskTypeTask)
	t2.UpdatedAt = now

	t3 := testutil.NewTestTask("Middle", models.TaskTypeTask)
	t3.UpdatedAt = now.Add(-1 * time.Hour)

	tasks := []*models.Task{t1, t2, t3}

	err := sortTasks(tasks, "updated")
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, tasks[0].Title, "Newest")
	testutil.AssertEqual(t, tasks[1].Title, "Middle")
	testutil.AssertEqual(t, tasks[2].Title, "Oldest")
}

func TestSortTasks_ByStatus(t *testing.T) {
	tasks := []*models.Task{
		testutil.NewTestTaskWithStatus("Done task", models.StatusDone),
		testutil.NewTestTaskWithStatus("InProgress task", models.StatusInProgress),
		testutil.NewTestTaskWithStatus("Todo task", models.StatusTodo),
	}

	err := sortTasks(tasks, "status")
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, tasks[0].Title, "InProgress task")
	testutil.AssertEqual(t, tasks[1].Title, "Todo task")
	testutil.AssertEqual(t, tasks[2].Title, "Done task")
}

func TestSortTasks_ByTitle(t *testing.T) {
	tasks := []*models.Task{
		testutil.NewTestTask("Charlie", models.TaskTypeTask),
		testutil.NewTestTask("alpha", models.TaskTypeTask),
		testutil.NewTestTask("Bravo", models.TaskTypeTask),
	}

	err := sortTasks(tasks, "title")
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, tasks[0].Title, "alpha")
	testutil.AssertEqual(t, tasks[1].Title, "Bravo")
	testutil.AssertEqual(t, tasks[2].Title, "Charlie")
}

func TestSortTasks_InvalidField(t *testing.T) {
	tasks := []*models.Task{
		testutil.NewTestTask("Task 1", models.TaskTypeTask),
	}

	err := sortTasks(tasks, "nonexistent")
	if err == nil {
		t.Error("expected error for invalid sort field, got nil")
	}
}

func TestSortTasks_EmptySlice(t *testing.T) {
	tasks := []*models.Task{}

	err := sortTasks(tasks, "priority")
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, len(tasks), 0)
}

func TestSortTasks_StableSort(t *testing.T) {
	tasks := []*models.Task{
		testutil.NewTestTaskWithPriority("First", 1),
		testutil.NewTestTaskWithPriority("Second", 1),
		testutil.NewTestTaskWithPriority("Third", 1),
	}

	err := sortTasks(tasks, "priority")
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, tasks[0].Title, "First")
	testutil.AssertEqual(t, tasks[1].Title, "Second")
	testutil.AssertEqual(t, tasks[2].Title, "Third")
}

func TestTaskMatchesFilters_ByType(t *testing.T) {
	defer resetFilterVars()
	listShowAll = true
	listTaskType = "bug"

	bug := testutil.NewTestTask("A bug", models.TaskTypeBug)
	task := testutil.NewTestTask("A task", models.TaskTypeTask)

	if !taskMatchesFilters(bug) {
		t.Error("expected bug to match type=bug filter")
	}
	if taskMatchesFilters(task) {
		t.Error("expected task to NOT match type=bug filter")
	}
}

func TestTaskMatchesFilters_ByTag(t *testing.T) {
	defer resetFilterVars()
	listShowAll = true
	listTag = "urgent"

	tagged := testutil.NewTestTask("Tagged", models.TaskTypeTask)
	tagged.Tags = []string{"urgent"}
	untagged := testutil.NewTestTask("Untagged", models.TaskTypeTask)
	untagged.Tags = []string{"low"}

	if !taskMatchesFilters(tagged) {
		t.Error("expected tagged task to match")
	}
	if taskMatchesFilters(untagged) {
		t.Error("expected untagged task to NOT match")
	}
}

func TestTaskMatchesFilters_SnoozedHiddenByDefault(t *testing.T) {
	defer resetFilterVars()

	snoozed := testutil.NewTestTask("Snoozed", models.TaskTypeTask)
	future := time.Now().Add(24 * time.Hour)
	snoozed.SnoozedUntil = &future

	normal := testutil.NewTestTask("Normal", models.TaskTypeTask)

	if taskMatchesFilters(snoozed) {
		t.Error("expected snoozed task to be hidden by default")
	}
	if !taskMatchesFilters(normal) {
		t.Error("expected normal task to be visible")
	}

	// With --snoozed flag, snoozed tasks are included
	listSnoozed = true
	if !taskMatchesFilters(snoozed) {
		t.Error("expected snoozed task to be visible with --snoozed flag")
	}
}

func TestTaskMatchesFilters_CompletedHiddenByDefault(t *testing.T) {
	defer resetFilterVars()

	done := testutil.NewTestTaskWithStatus("Done", models.StatusDone)
	todo := testutil.NewTestTaskWithStatus("Todo", models.StatusTodo)

	if taskMatchesFilters(done) {
		t.Error("expected completed task to be hidden by default")
	}
	if !taskMatchesFilters(todo) {
		t.Error("expected todo task to be visible")
	}

	// With --all flag, completed tasks are included
	listShowAll = true
	if !taskMatchesFilters(done) {
		t.Error("expected completed task to be visible with --all flag")
	}
}

func TestStatusOrderCoversAllStatuses(t *testing.T) {
	for _, status := range models.AllTaskStatuses() {
		if _, ok := statusOrder[status]; !ok {
			t.Errorf("statusOrder map is missing status %q", status)
		}
	}
}

func TestFilterTasks_ByStatusReviewAndStandby(t *testing.T) {
	defer resetFilterVars()
	listShowAll = true

	tasks := []*models.Task{
		testutil.NewTestTaskWithStatus("Todo task", models.StatusTodo),
		testutil.NewTestTaskWithStatus("Review task", models.StatusReview),
		testutil.NewTestTaskWithStatus("Standby task", models.StatusStandby),
	}

	listTaskStatus = "review"
	result := filterTasks(tasks)
	if len(result) != 1 {
		t.Errorf("expected 1 review task, got %d", len(result))
	}
	if len(result) > 0 && result[0].Status != models.StatusReview {
		t.Errorf("expected status review, got %s", result[0].Status)
	}

	listTaskStatus = "standby"
	result = filterTasks(tasks)
	if len(result) != 1 {
		t.Errorf("expected 1 standby task, got %d", len(result))
	}
	if len(result) > 0 && result[0].Status != models.StatusStandby {
		t.Errorf("expected status standby, got %s", result[0].Status)
	}
}

func TestSubtreeHasMatch_MatchingSelf(t *testing.T) {
	task := testutil.NewTestTaskWithStatus("Active", models.StatusTodo)
	task.Children = []*models.Task{}

	matches := func(t *models.Task) bool { return !t.Status.IsComplete() }

	if !subtreeHasMatch(task, matches) {
		t.Error("expected active task to match")
	}
}

func TestSubtreeHasMatch_NonMatchingSelfNoChildren(t *testing.T) {
	task := testutil.NewTestTaskWithStatus("Done", models.StatusDone)
	task.Children = []*models.Task{}

	matches := func(t *models.Task) bool { return !t.Status.IsComplete() }

	if subtreeHasMatch(task, matches) {
		t.Error("expected completed leaf task to not match")
	}
}

func TestSubtreeHasMatch_NonMatchingParentWithMatchingChild(t *testing.T) {
	parent := testutil.NewTestTaskWithStatus("Done Epic", models.StatusDone)
	parent.Type = models.TaskTypeEpic

	child := testutil.NewTestTaskWithStatus("Active Task", models.StatusInProgress)
	child.Children = []*models.Task{}

	parent.Children = []*models.Task{child}

	matches := func(t *models.Task) bool { return !t.Status.IsComplete() }

	if !subtreeHasMatch(parent, matches) {
		t.Error("expected completed parent with active child to match via subtree")
	}
}

func TestSubtreeHasMatch_NonMatchingParentWithMatchingGrandchild(t *testing.T) {
	grandparent := testutil.NewTestTaskWithStatus("Done Epic", models.StatusDone)
	grandparent.Type = models.TaskTypeEpic

	parent := testutil.NewTestTaskWithStatus("Done Story", models.StatusDone)
	parent.Type = models.TaskTypeStory

	child := testutil.NewTestTaskWithStatus("Active Task", models.StatusTodo)
	child.Children = []*models.Task{}

	parent.Children = []*models.Task{child}
	grandparent.Children = []*models.Task{parent}

	matches := func(t *models.Task) bool { return !t.Status.IsComplete() }

	if !subtreeHasMatch(grandparent, matches) {
		t.Error("expected completed grandparent with active grandchild to match via subtree")
	}
}

func TestSubtreeHasMatch_AllNonMatching(t *testing.T) {
	parent := testutil.NewTestTaskWithStatus("Done Epic", models.StatusDone)
	parent.Type = models.TaskTypeEpic

	child := testutil.NewTestTaskWithStatus("Done Task", models.StatusDone)
	child.Children = []*models.Task{}

	parent.Children = []*models.Task{child}

	matches := func(t *models.Task) bool { return !t.Status.IsComplete() }

	if subtreeHasMatch(parent, matches) {
		t.Error("expected all-completed subtree to not match")
	}
}
