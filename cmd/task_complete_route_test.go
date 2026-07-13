package cmd

import (
	"fmt"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

// findDoneTask returns a completed task from the seeded project.
func findDoneTask(t *testing.T, project *models.Project) *models.Task {
	t.Helper()
	for _, task := range project.Tasks {
		if task.State() == models.StateDone {
			return task
		}
	}
	t.Fatalf("no done task found in seeded project")
	return nil
}

// findFirstOfType returns the first task of the given type in the seeded project.
func findFirstOfType(t *testing.T, project *models.Project, taskType models.TaskType) *models.Task {
	t.Helper()
	for _, task := range project.Tasks {
		if task.Type == taskType {
			return task
		}
	}
	t.Fatalf("no %s task found in seeded project", taskType)
	return nil
}

// TestLegacyCompleteRoutesThroughApp proves the legacy `tt task complete` command
// now routes through the application-layer Transition use case and persists the
// unified done state via the repository (fallback save path under the in-memory
// test repository).
func TestLegacyCompleteRoutesThroughApp(t *testing.T) {
	deps, project := setupStateTest(t)
	task := findTodoLeaf(t, project)

	cmd := newStateTestCmd()
	testutil.AssertNoError(t, runTaskComplete(cmd, []string{fmt.Sprintf("%d", task.ShortID)}))

	got := deps.Repository.Projects[project.ID].GetTaskByID(task.ID)
	if got == nil {
		t.Fatalf("task %s missing from saved project", task.ID)
	}
	if got.State() != models.StateDone {
		t.Errorf("state = %q, want %q", got.State(), models.StateDone)
	}
}

// TestLegacyStartRoutesThroughApp proves `tt task start` routes through the app
// layer and persists the in_progress state.
func TestLegacyStartRoutesThroughApp(t *testing.T) {
	deps, project := setupStateTest(t)
	task := findTodoLeaf(t, project)

	cmd := newStateTestCmd()
	testutil.AssertNoError(t, runTaskStart(cmd, []string{fmt.Sprintf("%d", task.ShortID)}))

	got := deps.Repository.Projects[project.ID].GetTaskByID(task.ID)
	if got.State() != models.StateInProgress {
		t.Errorf("state = %q, want %q", got.State(), models.StateInProgress)
	}
}

// TestLegacyReopenRoutesThroughApp proves `tt task reopen` returns a terminal
// task to inbox through the app layer.
func TestLegacyReopenRoutesThroughApp(t *testing.T) {
	deps, project := setupStateTest(t)
	done := findDoneTask(t, project)

	cmd := newStateTestCmd()
	testutil.AssertNoError(t, runTaskReopen(cmd, []string{fmt.Sprintf("%d", done.ShortID)}))

	got := deps.Repository.Projects[project.ID].GetTaskByID(done.ID)
	if got.State() != models.StateInbox {
		t.Errorf("state = %q, want %q", got.State(), models.StateInbox)
	}
}

// TestLegacyCompleteBlockedByIncompleteChildren proves the CLI reroute inherits
// the application-layer parent-child completion guard: completing a parent that
// still has an incomplete child is refused before any mutation, so the parent is
// never persisted as done.
func TestLegacyCompleteBlockedByIncompleteChildren(t *testing.T) {
	deps, project := setupStateTest(t)
	epic := findFirstOfType(t, project, models.TaskTypeEpic)

	cmd := newStateTestCmd()
	err := runTaskComplete(cmd, []string{fmt.Sprintf("%d", epic.ShortID)})
	if err == nil {
		t.Fatal("expected completing a parent with incomplete children to be refused")
	}

	got := deps.Repository.Projects[project.ID].GetTaskByID(epic.ID)
	if got.State() == models.StateDone {
		t.Error("epic must not be completed while it has incomplete children")
	}
}
