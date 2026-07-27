package cmd

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestUpdateTaskStatusComplete(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test Project")
	task := project.Tasks[0]
	originalStatus := task.State()

	// Verify the task starts in a non-complete state
	if originalStatus == models.StateDone {
		t.Skip("test task already done")
	}

	task.Complete()
	testutil.AssertTaskState(t, task, models.StateDone)

	if task.CompletedAt == nil {
		t.Error("CompletedAt should be set after Complete()")
	}

	// Verify we can check original status
	if originalStatus == models.StateDone {
		t.Error("original status should not have been done")
	}
}

func TestUpdateTaskStatusStart(t *testing.T) {
	task := testutil.NewTestTask("Test Task", models.TaskTypeTask)
	testutil.AssertTaskState(t, task, models.StateReady)

	task.Start()
	testutil.AssertTaskState(t, task, models.StateInProgress)
}

func TestUpdateTaskStatusBlock(t *testing.T) {
	task := testutil.NewTestTask("Test Task", models.TaskTypeTask)
	testutil.AssertTaskState(t, task, models.StateReady)

	task.Block()
	testutil.AssertTaskState(t, task, models.StateWaiting)
}

func TestUpdateTaskStatusCancel(t *testing.T) {
	task := testutil.NewTestTask("Test Task", models.TaskTypeTask)
	testutil.AssertTaskState(t, task, models.StateReady)

	task.Cancel()
	testutil.AssertTaskState(t, task, models.StateCanceled)

	if task.CompletedAt == nil {
		t.Error("CompletedAt should be set after Cancel()")
	}
}

func TestUpdateTaskStatusReopen(t *testing.T) {
	task := testutil.NewTestTaskWithState("Completed Task", models.StateDone)
	testutil.AssertTaskState(t, task, models.StateDone)

	if task.CompletedAt == nil {
		t.Error("completed task should have CompletedAt set")
	}

	task.Reopen()
	testutil.AssertTaskState(t, task, models.StateInbox)

	if task.CompletedAt != nil {
		t.Error("CompletedAt should be cleared after Reopen()")
	}
}

func TestCompleteBlockedByDependencies(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	blocker, err := models.NewTask("blocker-1", 1, models.TaskTypeTask, "Blocker Task")
	if err != nil {
		t.Fatal(err)
	}
	dependent, err := models.NewTask("dependent-1", 2, models.TaskTypeTask, "Dependent Task")
	if err != nil {
		t.Fatal(err)
	}
	_ = dependent.AddDependency(blocker.ID)

	project.Tasks = []*models.Task{blocker, dependent}

	// Verify we have incomplete dependencies
	if !project.HasIncompleteDependencies(dependent) {
		t.Error("dependent should have incomplete dependencies")
	}

	incompleteDeps := project.GetIncompleteDependencies(dependent)
	if len(incompleteDeps) != 1 {
		t.Errorf("expected 1 incomplete dependency, got %d", len(incompleteDeps))
	}
	if incompleteDeps[0].ID != blocker.ID {
		t.Errorf("expected blocker to be the incomplete dependency")
	}

	// Complete the blocker
	blocker.Complete()

	// Now the dependent should have no incomplete dependencies
	if project.HasIncompleteDependencies(dependent) {
		t.Error("dependent should not have incomplete dependencies after blocker is complete")
	}
}

func TestAutoUnblockAfterComplete(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	blocker, err := models.NewTask("blocker-1", 1, models.TaskTypeTask, "Blocker Task")
	if err != nil {
		t.Fatal(err)
	}
	blocked, err := models.NewTask("blocked-1", 2, models.TaskTypeTask, "Blocked Task")
	if err != nil {
		t.Fatal(err)
	}
	_ = blocked.AddDependency(blocker.ID)
	blocked.WorkflowState = models.StateWaiting

	project.Tasks = []*models.Task{blocker, blocked}

	testutil.AssertTaskState(t, blocked, models.StateWaiting)

	// Complete the blocker
	blocker.Complete()

	// Run auto-unblock check
	project.CheckAndAutoUnblock()

	// Blocked task should now be unblocked to todo
	testutil.AssertTaskState(t, blocked, models.StateInbox)
}

func TestAutoUnblockAfterCancel(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	blocker, err := models.NewTask("blocker-1", 1, models.TaskTypeTask, "Blocker Task")
	if err != nil {
		t.Fatal(err)
	}
	blocked, err := models.NewTask("blocked-1", 2, models.TaskTypeTask, "Blocked Task")
	if err != nil {
		t.Fatal(err)
	}
	_ = blocked.AddDependency(blocker.ID)
	blocked.WorkflowState = models.StateWaiting

	project.Tasks = []*models.Task{blocker, blocked}

	// Cancel the blocker (also considered complete for dependency purposes)
	blocker.Cancel()

	// Run auto-unblock check
	project.CheckAndAutoUnblock()

	// Blocked task should now be unblocked to todo
	testutil.AssertTaskState(t, blocked, models.StateInbox)
}

func TestStatusTransitions(t *testing.T) {
	tests := []struct {
		name         string
		initialState models.WorkflowState
		action       func(*models.Task)
		wantStatus   models.WorkflowState
	}{
		{
			name:         "todo to in_progress",
			initialState: models.StateReady,
			action:       (*models.Task).Start,
			wantStatus:   models.StateInProgress,
		},
		{
			name:         "todo to done",
			initialState: models.StateReady,
			action:       (*models.Task).Complete,
			wantStatus:   models.StateDone,
		},
		{
			name:         "todo to blocked",
			initialState: models.StateReady,
			action:       (*models.Task).Block,
			wantStatus:   models.StateWaiting,
		},
		{
			name:         "todo to canceled",
			initialState: models.StateReady,
			action:       (*models.Task).Cancel,
			wantStatus:   models.StateCanceled,
		},
		{
			name:         "in_progress to done",
			initialState: models.StateInProgress,
			action:       (*models.Task).Complete,
			wantStatus:   models.StateDone,
		},
		{
			name:         "in_progress to blocked",
			initialState: models.StateInProgress,
			action:       (*models.Task).Block,
			wantStatus:   models.StateWaiting,
		},
		{
			name:         "blocked to in_progress",
			initialState: models.StateWaiting,
			action:       (*models.Task).Start,
			wantStatus:   models.StateInProgress,
		},
		{
			name:         "done to todo via reopen",
			initialState: models.StateDone,
			action:       (*models.Task).Reopen,
			wantStatus:   models.StateInbox,
		},
		{
			name:         "canceled to todo via reopen",
			initialState: models.StateCanceled,
			action:       (*models.Task).Reopen,
			wantStatus:   models.StateInbox,
		},
		{
			name:         "in_progress to review",
			initialState: models.StateInProgress,
			action:       (*models.Task).Review,
			wantStatus:   models.StateReview,
		},
		{
			name:         "todo to review",
			initialState: models.StateReady,
			action:       (*models.Task).Review,
			wantStatus:   models.StateReview,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := testutil.NewTestTaskWithState("Test Task", tt.initialState)
			tt.action(task)
			testutil.AssertTaskState(t, task, tt.wantStatus)
		})
	}
}

func TestWorkflowStateIsTerminal(t *testing.T) {
	tests := []struct {
		status   models.WorkflowState
		complete bool
	}{
		{models.StateReady, false},
		{models.StateInProgress, false},
		{models.StateReview, false},
		{models.StateWaiting, false},
		{models.StateDone, true},
		{models.StateCanceled, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsTerminal(); got != tt.complete {
				t.Errorf("WorkflowState(%q).IsTerminal() = %v, want %v", tt.status, got, tt.complete)
			}
		})
	}
}

func TestCompleteParentAllChildrenDone(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	parent, err := models.NewTask("parent-1", 1, models.TaskTypeEpic, "Parent Epic")
	if err != nil {
		t.Fatal(err)
	}
	child1, err := models.NewTask("child-1", 2, models.TaskTypeTask, "Child Done")
	if err != nil {
		t.Fatal(err)
	}
	child2, err := models.NewTask("child-2", 3, models.TaskTypeTask, "Child Canceled")
	if err != nil {
		t.Fatal(err)
	}
	child1.ParentID = &parent.ID
	child2.ParentID = &parent.ID
	child1.WorkflowState = models.StateDone
	child2.WorkflowState = models.StateCanceled

	project.Tasks = []*models.Task{parent, child1, child2}
	project.BuildTaskTree()

	parentTask := project.GetTaskByID(parent.ID)
	if err := CheckCompletionBlockers(project, parentTask); err != nil {
		t.Errorf("expected no blocker when all children done/canceled, got %v", err)
	}
}

func TestCompleteParentBlockedByIncompleteChildren(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	parent, err := models.NewTask("parent-1", 1, models.TaskTypeEpic, "Parent Epic")
	if err != nil {
		t.Fatal(err)
	}
	child1, err := models.NewTask("child-1", 2, models.TaskTypeTask, "Child Todo")
	if err != nil {
		t.Fatal(err)
	}
	child2, err := models.NewTask("child-2", 3, models.TaskTypeTask, "Child Done")
	if err != nil {
		t.Fatal(err)
	}
	child1.ParentID = &parent.ID
	child2.ParentID = &parent.ID
	child2.WorkflowState = models.StateDone

	project.Tasks = []*models.Task{parent, child1, child2}
	project.BuildTaskTree()

	parentTask := project.GetTaskByID(parent.ID)
	err = CheckCompletionBlockers(project, parentTask)
	if err == nil {
		t.Fatal("expected CheckCompletionBlockers to block parent with an incomplete child")
	}
	if !strings.Contains(err.Error(), "incomplete children") {
		t.Errorf("expected incomplete-children error, got %v", err)
	}
	if !strings.Contains(err.Error(), child1.Title) {
		t.Errorf("expected error to name incomplete child %q, got %v", child1.Title, err)
	}
}

func TestCompleteBlockedByCrossProjectDependencies(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Task with cross-project dep")
	if err != nil {
		t.Fatal(err)
	}

	// Add a cross-project dependency directly (format "projectID:taskID")
	task.DependsOn = []string{"other-project:remote-task-id"}
	project.Tasks = []*models.Task{task}

	// HasIncompleteDependencies should detect the cross-project dep
	if !project.HasIncompleteDependencies(task) {
		t.Error("expected HasIncompleteDependencies to return true for cross-project dep")
	}

	// GetIncompleteDependencies cannot resolve cross-project deps, so returns empty
	incompleteDeps := project.GetIncompleteDependencies(task)
	if len(incompleteDeps) != 0 {
		t.Errorf("expected 0 locally-resolvable incomplete deps, got %d", len(incompleteDeps))
	}

	// countCrossProjectDeps should count 1
	crossCount := countCrossProjectDeps(task)
	if crossCount != 1 {
		t.Errorf("expected 1 cross-project dep, got %d", crossCount)
	}
}

func TestCompleteBlockedByMixedDependencies(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	blocker, err := models.NewTask("blocker-1", 1, models.TaskTypeTask, "Local Blocker")
	if err != nil {
		t.Fatal(err)
	}
	task, err := models.NewTask("task-1", 2, models.TaskTypeTask, "Task with mixed deps")
	if err != nil {
		t.Fatal(err)
	}

	// Add both a local and cross-project dependency
	_ = task.AddDependency(blocker.ID)
	task.DependsOn = append(task.DependsOn, "other-project:remote-task-id")
	project.Tasks = []*models.Task{blocker, task}

	// Both should block
	if !project.HasIncompleteDependencies(task) {
		t.Error("expected HasIncompleteDependencies to return true for mixed deps")
	}

	// GetIncompleteDependencies should only return the local one
	incompleteDeps := project.GetIncompleteDependencies(task)
	if len(incompleteDeps) != 1 {
		t.Errorf("expected 1 locally-resolvable incomplete dep, got %d", len(incompleteDeps))
	}
	if len(incompleteDeps) > 0 && incompleteDeps[0].ID != blocker.ID {
		t.Error("expected local blocker to be the incomplete dep")
	}

	crossCount := countCrossProjectDeps(task)
	if crossCount != 1 {
		t.Errorf("expected 1 cross-project dep, got %d", crossCount)
	}

	// Complete local blocker — should still block due to cross-project dep
	blocker.Complete()
	if !project.HasIncompleteDependencies(task) {
		t.Error("expected HasIncompleteDependencies to still return true after completing local blocker (cross-project dep remains)")
	}
}

func TestWorkflowStateIsNonterminal(t *testing.T) {
	tests := []struct {
		status models.WorkflowState
		active bool
	}{
		{models.StateReady, true},
		{models.StateInProgress, true},
		{models.StateReview, true},
		{models.StateWaiting, true},
		{models.StateDone, false},
		{models.StateCanceled, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsNonterminal(); got != tt.active {
				t.Errorf("WorkflowState(%q).IsNonterminal() = %v, want %v", tt.status, got, tt.active)
			}
		})
	}
}
