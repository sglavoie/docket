package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func makeBoardTaskWithStatus(t *testing.T, id string, shortID int, title string, status models.TaskStatus) *models.Task {
	t.Helper()
	task := makeBoardTask(t, id, shortID, title)
	task.WorkflowState = models.TargetStateForStatus(status)
	return task
}

// TestExpandTodayChildren_OpenBeforeCompleted verifies that a planned parent
// whose children are stored as [done, open, canceled, open] renders as
// [open, open, done, canceled].
func TestExpandTodayChildren_OpenBeforeCompleted(t *testing.T) {
	project, _ := models.NewProject("proj", "proj", "/tmp/test-proj")

	parent := makeBoardTask(t, "id-parent", 1, "Parent")
	childDone := makeBoardTaskWithStatus(t, "id-done", 2, "Done", models.StatusDone)
	childOpenA := makeBoardTaskWithStatus(t, "id-openA", 3, "OpenA", models.StatusInProgress)
	childCanceled := makeBoardTaskWithStatus(t, "id-canceled", 4, "Canceled", models.StatusCanceled)
	childOpenB := makeBoardTaskWithStatus(t, "id-openB", 5, "OpenB", models.StatusTodo)

	parent.Children = []*models.Task{childDone, childOpenA, childCanceled, childOpenB}

	m := newTestFocusBoard()
	m.childDepthLimit = -1
	m.dailyPlanIDs = map[string]bool{parent.ID: true}
	m.todayRows = []boardRow{
		{Task: parent, Project: project, Source: project.Name},
	}

	m.expandTodayChildren()

	wantIDs := []string{"id-parent", "id-openA", "id-openB", "id-done", "id-canceled"}
	if len(m.todayRows) != len(wantIDs) {
		t.Fatalf("expected %d rows, got %d", len(wantIDs), len(m.todayRows))
	}
	for i, want := range wantIDs {
		if m.todayRows[i].Task.ID != want {
			t.Errorf("row[%d]: expected %s, got %s", i, want, m.todayRows[i].Task.ID)
		}
	}
}

// TestExpandTodayChildren_StableWithinGroups verifies relative order is
// preserved within the open and completed groups.
func TestExpandTodayChildren_StableWithinGroups(t *testing.T) {
	project, _ := models.NewProject("proj", "proj", "/tmp/test-proj")

	parent := makeBoardTask(t, "id-parent", 1, "Parent")
	open1 := makeBoardTaskWithStatus(t, "id-open1", 2, "Open1", models.StatusTodo)
	done1 := makeBoardTaskWithStatus(t, "id-done1", 3, "Done1", models.StatusDone)
	open2 := makeBoardTaskWithStatus(t, "id-open2", 4, "Open2", models.StatusInProgress)
	done2 := makeBoardTaskWithStatus(t, "id-done2", 5, "Done2", models.StatusDone)
	open3 := makeBoardTaskWithStatus(t, "id-open3", 6, "Open3", models.StatusReview)
	canceled1 := makeBoardTaskWithStatus(t, "id-canceled1", 7, "Canceled1", models.StatusCanceled)

	parent.Children = []*models.Task{open1, done1, open2, done2, open3, canceled1}

	m := newTestFocusBoard()
	m.childDepthLimit = -1
	m.dailyPlanIDs = map[string]bool{parent.ID: true}
	m.todayRows = []boardRow{
		{Task: parent, Project: project, Source: project.Name},
	}

	m.expandTodayChildren()

	// Opens in original order: open1, open2, open3
	// Completes in original order: done1, done2, canceled1
	wantIDs := []string{"id-parent", "id-open1", "id-open2", "id-open3", "id-done1", "id-done2", "id-canceled1"}
	if len(m.todayRows) != len(wantIDs) {
		t.Fatalf("expected %d rows, got %d", len(wantIDs), len(m.todayRows))
	}
	for i, want := range wantIDs {
		if m.todayRows[i].Task.ID != want {
			t.Errorf("row[%d]: expected %s, got %s", i, want, m.todayRows[i].Task.ID)
		}
	}
}

// TestExpandTodayChildren_RecursiveOrdering verifies the open-first partition
// applies at every hierarchy depth, not just the top child level.
func TestExpandTodayChildren_RecursiveOrdering(t *testing.T) {
	project, _ := models.NewProject("proj", "proj", "/tmp/test-proj")

	parent := makeBoardTask(t, "id-parent", 1, "Parent")
	mid := makeBoardTaskWithStatus(t, "id-mid", 2, "Mid", models.StatusInProgress)
	grandDone := makeBoardTaskWithStatus(t, "id-gdone", 3, "GDone", models.StatusDone)
	grandOpen := makeBoardTaskWithStatus(t, "id-gopen", 4, "GOpen", models.StatusTodo)
	grandCanceled := makeBoardTaskWithStatus(t, "id-gcanceled", 5, "GCanceled", models.StatusCanceled)

	mid.Children = []*models.Task{grandDone, grandOpen, grandCanceled}
	parent.Children = []*models.Task{mid}

	m := newTestFocusBoard()
	m.childDepthLimit = -1
	m.dailyPlanIDs = map[string]bool{parent.ID: true}
	m.todayRows = []boardRow{
		{Task: parent, Project: project, Source: project.Name},
	}

	m.expandTodayChildren()

	// Expected: parent, mid, then mid's children with open first.
	wantIDs := []string{"id-parent", "id-mid", "id-gopen", "id-gdone", "id-gcanceled"}
	if len(m.todayRows) != len(wantIDs) {
		t.Fatalf("expected %d rows, got %d", len(wantIDs), len(m.todayRows))
	}
	for i, want := range wantIDs {
		if m.todayRows[i].Task.ID != want {
			t.Errorf("row[%d]: expected %s, got %s", i, want, m.todayRows[i].Task.ID)
		}
	}
}

// TestExpandTodayChildren_TreeConnectorsReflectReordering verifies that the
// tree connector prefix on each child row reflects the post-partition position
// (e.g., the last completed child gets the └─── connector, not whichever
// child was stored last).
func TestExpandTodayChildren_TreeConnectorsReflectReordering(t *testing.T) {
	project, _ := models.NewProject("proj", "proj", "/tmp/test-proj")

	parent := makeBoardTask(t, "id-parent", 1, "Parent")
	childDone := makeBoardTaskWithStatus(t, "id-done", 2, "Done", models.StatusDone)
	childOpen := makeBoardTaskWithStatus(t, "id-open", 3, "Open", models.StatusInProgress)
	childCanceled := makeBoardTaskWithStatus(t, "id-canceled", 4, "Canceled", models.StatusCanceled)

	// Stored order ends with an open child, but after partition the open
	// child is no longer last — the canceled child should be the └─── tail.
	parent.Children = []*models.Task{childDone, childCanceled, childOpen}

	m := newTestFocusBoard()
	m.childDepthLimit = -1
	m.dailyPlanIDs = map[string]bool{parent.ID: true}
	m.todayRows = []boardRow{
		{Task: parent, Project: project, Source: project.Name},
	}

	m.expandTodayChildren()

	wantIDs := []string{"id-parent", "id-open", "id-done", "id-canceled"}
	if len(m.todayRows) != len(wantIDs) {
		t.Fatalf("expected %d rows, got %d", len(wantIDs), len(m.todayRows))
	}
	for i, want := range wantIDs {
		if m.todayRows[i].Task.ID != want {
			t.Errorf("row[%d]: expected %s, got %s", i, want, m.todayRows[i].Task.ID)
		}
	}

	// Child rows are at indices 1, 2, 3. The last visible sibling (canceled)
	// must use the └─── connector; the others use ├───.
	lastChildPrefix := m.todayRows[3].TreePrefix
	if lastChildPrefix == "" {
		t.Fatalf("expected last child to carry a tree prefix")
	}
	if !strings.Contains(lastChildPrefix, "└───") {
		t.Errorf("last child prefix %q should contain └───", lastChildPrefix)
	}
	for i := 1; i < 3; i++ {
		prefix := m.todayRows[i].TreePrefix
		if !strings.Contains(prefix, "├───") {
			t.Errorf("row[%d] prefix %q should contain ├───", i, prefix)
		}
	}
}
