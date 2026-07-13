package cmd

import (
	"fmt"
	"testing"

	"github.com/sglavoie/tt/internal/testutil"
)

// TestPriorityRoutesThroughApp proves `tt task priority` raises a task's priority
// through the application-layer SetPriority use case and persists it via the
// repository (fallback save path under the in-memory test repository).
func TestPriorityRoutesThroughApp(t *testing.T) {
	deps, project := setupStateTest(t)
	task := findTodoLeaf(t, project)

	cmd := newStateTestCmd()
	err := runTaskPriority(cmd, []string{fmt.Sprintf("%d", task.ShortID), "1"})
	testutil.AssertNoError(t, err)

	got := deps.Repository.Projects[project.ID].GetTaskByID(task.ID)
	if got == nil {
		t.Fatalf("task %s missing from saved project", task.ID)
	}
	if got.Priority != 1 {
		t.Errorf("priority = %d, want 1", got.Priority)
	}
}

// TestPriorityNoOpDoesNotMutate proves setting a task to its current priority is a
// no-op that skips the commit entirely, leaving UpdatedAt untouched.
func TestPriorityNoOpDoesNotMutate(t *testing.T) {
	deps, project := setupStateTest(t)
	task := findTodoLeaf(t, project)
	original := task.UpdatedAt

	cmd := newStateTestCmd()
	err := runTaskPriority(cmd, []string{fmt.Sprintf("%d", task.ShortID), fmt.Sprintf("%d", task.Priority)})
	testutil.AssertNoError(t, err)

	got := deps.Repository.Projects[project.ID].GetTaskByID(task.ID)
	if !got.UpdatedAt.Equal(original) {
		t.Errorf("UpdatedAt changed on a no-op priority set: %v -> %v", original, got.UpdatedAt)
	}
}

// TestSnoozeRoutesThroughApp proves `tt task snooze` sets the snooze date through
// the application-layer Snooze use case.
func TestSnoozeRoutesThroughApp(t *testing.T) {
	deps, project := setupStateTest(t)
	task := findTodoLeaf(t, project)

	cmd := newStateTestCmd()
	err := runTaskSnooze(cmd, []string{fmt.Sprintf("%d", task.ShortID), "+1w"})
	testutil.AssertNoError(t, err)

	got := deps.Repository.Projects[project.ID].GetTaskByID(task.ID)
	if got.SnoozedUntil == nil {
		t.Error("expected SnoozedUntil to be set after snooze")
	}
}

// TestSnoozeInProgressRejected proves the app-layer guard rejects snoozing an
// in-progress task through the migrated CLI path.
func TestSnoozeInProgressRejected(t *testing.T) {
	deps, project := setupStateTest(t)
	task := findTodoLeaf(t, project)
	task.Start()
	testutil.AssertNoError(t, deps.Repository.Save(project))

	cmd := newStateTestCmd()
	if err := runTaskSnooze(cmd, []string{fmt.Sprintf("%d", task.ShortID), "+1w"}); err == nil {
		t.Fatal("expected error snoozing an in-progress task")
	}
}

// TestDimRoutesThroughApp proves `tt task dim` toggles the dimmed flag through the
// application-layer ToggleDimmed use case.
func TestDimRoutesThroughApp(t *testing.T) {
	deps, project := setupStateTest(t)
	task := findTodoLeaf(t, project)

	err := runTaskDim(nil, []string{fmt.Sprintf("%d", task.ShortID)})
	testutil.AssertNoError(t, err)

	got := deps.Repository.Projects[project.ID].GetTaskByID(task.ID)
	if !got.Dimmed {
		t.Error("expected task to be dimmed after dim command")
	}
}
