package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func makeBoardTask(t *testing.T, id string, shortID int, title string) *models.Task {
	t.Helper()
	task, err := models.NewGenericTask(id, shortID, title)
	if err != nil {
		t.Fatal(err)
	}
	task.WorkflowState = models.StateInProgress
	return task
}

func makeBoardTaskWithStatus(t *testing.T, id string, shortID int, title string, status models.WorkflowState) *models.Task {
	t.Helper()
	task := makeBoardTask(t, id, shortID, title)
	task.WorkflowState = status
	return task
}

func modeRowIDs(rows []OutlinerModeRow) []string {
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.Task != nil {
			ids = append(ids, r.Task.ID)
		}
	}
	return ids
}

// TestOutlinerModeNextOrdersByQueueRank ports the Planner Next ordering
// regression: BuildNextModeRows lists StateNext tasks ordered by the focus-queue
// index, appending unindexed members in stable input order, and excludes
// terminal/snoozed tasks.
func TestOutlinerModeNextOrdersByQueueRank(t *testing.T) {
	project, _ := models.NewProject("proj", "Project", "/tmp/next-proj")

	a := &models.Task{ID: "id-a", ShortID: 1, Title: "Alpha", WorkflowState: models.StateNext}
	b := &models.Task{ID: "id-b", ShortID: 2, Title: "Bravo", WorkflowState: models.StateNext}
	c := &models.Task{ID: "id-c", ShortID: 3, Title: "Charlie", WorkflowState: models.StateNext}
	d := &models.Task{ID: "id-d", ShortID: 4, Title: "Delta", WorkflowState: models.StateNext}
	e := &models.Task{ID: "id-e", ShortID: 5, Title: "Echo", WorkflowState: models.StateNext}
	ready := &models.Task{ID: "ready", ShortID: 6, Title: "Ready", WorkflowState: models.StateReady}
	done := &models.Task{ID: "done", ShortID: 7, Title: "Done", WorkflowState: models.StateDone}
	// Input (project) order is a,b,c,d,e so unindexed d,e keep input order.
	project.Tasks = []*models.Task{a, b, c, d, e, ready, done}

	rows := BuildNextModeRows(OutlinerModeInput{
		Projects: []*models.Project{project},
		// Only A,B,C indexed; D,E unindexed StateNext members.
		NextOrder: []config.ProjectTaskLocator{
			{TaskID: "id-a", ProjectID: project.ID},
			{TaskID: "id-b", ProjectID: project.ID},
			{TaskID: "id-c", ProjectID: project.ID},
		},
	})

	want := []string{"id-a", "id-b", "id-c", "id-d", "id-e"}
	got := modeRowIDs(rows)
	if len(got) != len(want) {
		t.Fatalf("next IDs = %v, want %v (ready/done must be excluded)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("next order = %v, want %v", got, want)
		}
	}
}

// TestTodaySectionRowsOpenBeforeCompleted ports the Planner Today ordering
// regression: a planned parent whose children are stored as
// [done, open, canceled, open] renders as [open, open, done, canceled].
func TestTodaySectionRowsOpenBeforeCompleted(t *testing.T) {
	project, _ := models.NewProject("proj", "proj", "/tmp/test-proj")

	parent := makeBoardTask(t, "id-parent", 1, "Parent")
	childDone := makeBoardTaskWithStatus(t, "id-done", 2, "Done", models.StateDone)
	childOpenA := makeBoardTaskWithStatus(t, "id-openA", 3, "OpenA", models.StateInProgress)
	childCanceled := makeBoardTaskWithStatus(t, "id-canceled", 4, "Canceled", models.StateCanceled)
	childOpenB := makeBoardTaskWithStatus(t, "id-openB", 5, "OpenB", models.StateReady)
	parent.Children = []*models.Task{childDone, childOpenA, childCanceled, childOpenB}
	project.Tasks = []*models.Task{parent}

	rows := BuildTodayModeRows(OutlinerModeInput{
		Projects:        []*models.Project{project},
		TodayIDs:        map[TaskIdentity]bool{taskIdentityFor(parent, project): true},
		ChildDepthLimit: -1,
	})

	wantIDs := []string{"id-parent", "id-openA", "id-openB", "id-done", "id-canceled"}
	assertModeRowOrder(t, rows, wantIDs)
}

// TestTodaySectionRowsRecursiveOrdering ports the recursive partition: open
// children come before completed ones at every hierarchy depth.
func TestTodaySectionRowsRecursiveOrdering(t *testing.T) {
	project, _ := models.NewProject("proj", "proj", "/tmp/test-proj")

	parent := makeBoardTask(t, "id-parent", 1, "Parent")
	mid := makeBoardTaskWithStatus(t, "id-mid", 2, "Mid", models.StateInProgress)
	grandDone := makeBoardTaskWithStatus(t, "id-gdone", 3, "GDone", models.StateDone)
	grandOpen := makeBoardTaskWithStatus(t, "id-gopen", 4, "GOpen", models.StateReady)
	grandCanceled := makeBoardTaskWithStatus(t, "id-gcanceled", 5, "GCanceled", models.StateCanceled)
	mid.Children = []*models.Task{grandDone, grandOpen, grandCanceled}
	parent.Children = []*models.Task{mid}
	project.Tasks = []*models.Task{parent}

	rows := BuildTodayModeRows(OutlinerModeInput{
		Projects:        []*models.Project{project},
		TodayIDs:        map[TaskIdentity]bool{taskIdentityFor(parent, project): true},
		ChildDepthLimit: -1,
	})

	wantIDs := []string{"id-parent", "id-mid", "id-gopen", "id-gdone", "id-gcanceled"}
	assertModeRowOrder(t, rows, wantIDs)
}

// TestTodaySectionRowsTreeConnectors ports the connector invariant: after the
// open-before-completed partition, the last visible sibling carries the └───
// connector even if it was not stored last.
func TestTodaySectionRowsTreeConnectors(t *testing.T) {
	project, _ := models.NewProject("proj", "proj", "/tmp/test-proj")

	parent := makeBoardTask(t, "id-parent", 1, "Parent")
	childDone := makeBoardTaskWithStatus(t, "id-done", 2, "Done", models.StateDone)
	childOpen := makeBoardTaskWithStatus(t, "id-open", 3, "Open", models.StateInProgress)
	childCanceled := makeBoardTaskWithStatus(t, "id-canceled", 4, "Canceled", models.StateCanceled)
	parent.Children = []*models.Task{childDone, childCanceled, childOpen}
	project.Tasks = []*models.Task{parent}

	rows := BuildTodayModeRows(OutlinerModeInput{
		Projects:        []*models.Project{project},
		TodayIDs:        map[TaskIdentity]bool{taskIdentityFor(parent, project): true},
		ChildDepthLimit: -1,
	})

	wantIDs := []string{"id-parent", "id-open", "id-done", "id-canceled"}
	assertModeRowOrder(t, rows, wantIDs)

	if prefix := rows[3].TreePrefix; !strings.Contains(prefix, "└───") {
		t.Errorf("last child prefix %q should contain └───", prefix)
	}
	for i := 1; i < 3; i++ {
		if prefix := rows[i].TreePrefix; !strings.Contains(prefix, "├───") {
			t.Errorf("row[%d] prefix %q should contain ├───", i, rows[i].TreePrefix)
		}
	}
}

func TestTodaySectionRowsHardExcludePrunesDescendantsWithoutTaskFilter(t *testing.T) {
	project, _ := models.NewProject("proj", "proj", "/tmp/test-proj")
	parent := makeBoardTask(t, "id-parent", 1, "Parent")
	visible := makeBoardTask(t, "id-visible", 2, "Visible")
	hidden := makeBoardTask(t, "id-hidden", 3, "Hidden")
	parent.Children = []*models.Task{hidden, visible}
	project.Tasks = []*models.Task{parent}

	rows := BuildTodayModeRows(OutlinerModeInput{
		Projects:        []*models.Project{project},
		TodayIDs:        map[TaskIdentity]bool{taskIdentityFor(parent, project): true},
		ChildDepthLimit: -1,
		TaskExclude: func(task *models.Task, _ *models.Project) bool {
			return task.ID == hidden.ID
		},
	})

	assertModeRowOrder(t, rows, []string{parent.ID, visible.ID})
}

func assertModeRowOrder(t *testing.T, rows []OutlinerModeRow, wantIDs []string) {
	t.Helper()
	if len(rows) != len(wantIDs) {
		t.Fatalf("expected %d rows, got %d (%v)", len(wantIDs), len(rows), modeRowIDs(rows))
	}
	for i, want := range wantIDs {
		if rows[i].Task.ID != want {
			t.Errorf("row[%d]: expected %s, got %s", i, want, rows[i].Task.ID)
		}
	}
}
