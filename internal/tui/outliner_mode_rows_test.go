package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func makeBoardTask(t *testing.T, id string, shortID int, title string) *models.Task {
	t.Helper()
	task, err := models.NewTask(id, shortID, models.TaskTypeTask, title)
	if err != nil {
		t.Fatal(err)
	}
	task.WorkflowState = models.StateInProgress
	return task
}

func makeBoardTaskWithStatus(t *testing.T, id string, shortID int, title string, status models.TaskStatus) *models.Task {
	t.Helper()
	task := makeBoardTask(t, id, shortID, title)
	task.WorkflowState = models.TargetStateForStatus(status)
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

// TestOutlinerModeBacklogExcludesTodayAndNextRanked ports the Planner Backlog
// regression to the mode engine: BuildBacklogModeRows lists actionable tasks
// excluding the Today (daily plan) and Next (StateNext) horizons, ordered by the
// shared cross-project ranker (higher score first).
func TestOutlinerModeBacklogExcludesTodayAndNextRanked(t *testing.T) {
	project, _ := models.NewProject("proj", "Project", "/tmp/backlog-proj")

	hi := &models.Task{ID: "hi", ShortID: 1, Type: models.TaskTypeTask, Title: "High", WorkflowState: models.StateInProgress, Priority: 1}
	lo := &models.Task{ID: "lo", ShortID: 2, Type: models.TaskTypeTask, Title: "Low", WorkflowState: models.StateReady, Priority: 5}
	next := &models.Task{ID: "next", ShortID: 3, Type: models.TaskTypeTask, Title: "Next", WorkflowState: models.StateNext, Priority: 1}
	planned := &models.Task{ID: "planned", ShortID: 4, Type: models.TaskTypeTask, Title: "Planned", WorkflowState: models.StateReady, Priority: 1}
	project.Tasks = []*models.Task{hi, lo, next, planned}

	rows := BuildBacklogModeRows(OutlinerModeInput{
		Projects:     []*models.Project{project},
		DailyPlanIDs: map[TaskIdentity]bool{taskIdentityFor(planned, project): true},
		Scorer:       config.EmptyRuntimeState(""),
	})

	// Next (StateNext) and Today (daily plan) are excluded; the remaining
	// actionable tasks are ranked, so the in-progress P1 outranks the ready P5.
	want := []string{"hi", "lo"}
	got := modeRowIDs(rows)
	if len(got) != len(want) {
		t.Fatalf("backlog IDs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("backlog order = %v, want %v", got, want)
		}
	}
}

// TestOutlinerModeNextOrdersByQueueRank ports the Planner Next ordering
// regression: BuildNextModeRows lists StateNext tasks ordered by the focus-queue
// index, appending unindexed members in stable input order, and excludes
// terminal/snoozed tasks.
func TestOutlinerModeNextOrdersByQueueRank(t *testing.T) {
	project, _ := models.NewProject("proj", "Project", "/tmp/next-proj")

	a := &models.Task{ID: "id-a", ShortID: 1, Type: models.TaskTypeTask, Title: "Alpha", WorkflowState: models.StateNext}
	b := &models.Task{ID: "id-b", ShortID: 2, Type: models.TaskTypeTask, Title: "Bravo", WorkflowState: models.StateNext}
	c := &models.Task{ID: "id-c", ShortID: 3, Type: models.TaskTypeTask, Title: "Charlie", WorkflowState: models.StateNext}
	d := &models.Task{ID: "id-d", ShortID: 4, Type: models.TaskTypeTask, Title: "Delta", WorkflowState: models.StateNext}
	e := &models.Task{ID: "id-e", ShortID: 5, Type: models.TaskTypeTask, Title: "Echo", WorkflowState: models.StateNext}
	ready := &models.Task{ID: "ready", ShortID: 6, Type: models.TaskTypeTask, Title: "Ready", WorkflowState: models.StateReady}
	done := &models.Task{ID: "done", ShortID: 7, Type: models.TaskTypeTask, Title: "Done", WorkflowState: models.StateDone}
	// Input (project) order is a,b,c,d,e so unindexed d,e keep input order.
	project.Tasks = []*models.Task{a, b, c, d, e, ready, done}

	rows := BuildNextModeRows(OutlinerModeInput{
		Projects: []*models.Project{project},
		// Only A,B,C indexed; D,E unindexed StateNext members.
		FocusQueue: []config.FocusQueueItem{
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
	childDone := makeBoardTaskWithStatus(t, "id-done", 2, "Done", models.StatusDone)
	childOpenA := makeBoardTaskWithStatus(t, "id-openA", 3, "OpenA", models.StatusInProgress)
	childCanceled := makeBoardTaskWithStatus(t, "id-canceled", 4, "Canceled", models.StatusCanceled)
	childOpenB := makeBoardTaskWithStatus(t, "id-openB", 5, "OpenB", models.StatusTodo)
	parent.Children = []*models.Task{childDone, childOpenA, childCanceled, childOpenB}
	project.Tasks = []*models.Task{parent}

	rows := BuildTodayModeRows(OutlinerModeInput{
		Projects:        []*models.Project{project},
		DailyPlanIDs:    map[TaskIdentity]bool{taskIdentityFor(parent, project): true},
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
	mid := makeBoardTaskWithStatus(t, "id-mid", 2, "Mid", models.StatusInProgress)
	grandDone := makeBoardTaskWithStatus(t, "id-gdone", 3, "GDone", models.StatusDone)
	grandOpen := makeBoardTaskWithStatus(t, "id-gopen", 4, "GOpen", models.StatusTodo)
	grandCanceled := makeBoardTaskWithStatus(t, "id-gcanceled", 5, "GCanceled", models.StatusCanceled)
	mid.Children = []*models.Task{grandDone, grandOpen, grandCanceled}
	parent.Children = []*models.Task{mid}
	project.Tasks = []*models.Task{parent}

	rows := BuildTodayModeRows(OutlinerModeInput{
		Projects:        []*models.Project{project},
		DailyPlanIDs:    map[TaskIdentity]bool{taskIdentityFor(parent, project): true},
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
	childDone := makeBoardTaskWithStatus(t, "id-done", 2, "Done", models.StatusDone)
	childOpen := makeBoardTaskWithStatus(t, "id-open", 3, "Open", models.StatusInProgress)
	childCanceled := makeBoardTaskWithStatus(t, "id-canceled", 4, "Canceled", models.StatusCanceled)
	parent.Children = []*models.Task{childDone, childCanceled, childOpen}
	project.Tasks = []*models.Task{parent}

	rows := BuildTodayModeRows(OutlinerModeInput{
		Projects:        []*models.Project{project},
		DailyPlanIDs:    map[TaskIdentity]bool{taskIdentityFor(parent, project): true},
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
		DailyPlanIDs:    map[TaskIdentity]bool{taskIdentityFor(parent, project): true},
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
