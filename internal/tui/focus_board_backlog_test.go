package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestFocusBoardBacklogExcludesTodayAndNextRanked verifies the Backlog tab lists
// actionable tasks excluding the Today (daily plan) and Next (StateNext)
// horizons, ordered by the shared cross-project ranker (higher score first).
func TestFocusBoardBacklogExcludesTodayAndNextRanked(t *testing.T) {
	project, _ := models.NewProject("proj", "Project", "/tmp/backlog-proj")

	hi := &models.Task{ID: "hi", ShortID: 1, Type: models.TaskTypeTask, Title: "High", WorkflowState: models.StateInProgress, Priority: 1}
	lo := &models.Task{ID: "lo", ShortID: 2, Type: models.TaskTypeTask, Title: "Low", WorkflowState: models.StateReady, Priority: 5}
	next := &models.Task{ID: "next", ShortID: 3, Type: models.TaskTypeTask, Title: "Next", WorkflowState: models.StateNext, Priority: 1}
	planned := &models.Task{ID: "planned", ShortID: 4, Type: models.TaskTypeTask, Title: "Planned", WorkflowState: models.StateReady, Priority: 1}
	project.Tasks = []*models.Task{hi, lo, next, planned}

	m := newTestFocusBoard()
	m.dailyPlanIDs[taskIdentityFor(planned, project)] = true

	m.buildBacklogRows([]*models.Project{project})

	gotIDs := make([]string, 0, len(m.backlogRows))
	for _, r := range m.backlogRows {
		gotIDs = append(gotIDs, r.Task.ID)
	}

	// Next (StateNext) and Today (daily plan) are excluded; the remaining
	// actionable tasks are ranked, so the in-progress P1 outranks the ready P5.
	want := []string{"hi", "lo"}
	if len(gotIDs) != len(want) {
		t.Fatalf("backlog IDs = %v, want %v", gotIDs, want)
	}
	for i := range want {
		if gotIDs[i] != want[i] {
			t.Fatalf("backlog order = %v, want %v", gotIDs, want)
		}
	}
}

// TestFocusBoardBacklogByProjectQuickFilter verifies the "By Project" quick
// filter narrows the ranked Backlog to the selected projects while preserving
// rank order, and that clearing it restores the full list.
func TestFocusBoardBacklogByProjectQuickFilter(t *testing.T) {
	alpha, _ := models.NewProject("alpha", "Alpha", "/tmp/alpha")
	beta, _ := models.NewProject("beta", "Beta", "/tmp/beta")

	a1 := &models.Task{ID: "a1", ShortID: 1, Type: models.TaskTypeTask, Title: "A1", WorkflowState: models.StateReady, Priority: 1}
	b1 := &models.Task{ID: "b1", ShortID: 2, Type: models.TaskTypeTask, Title: "B1", WorkflowState: models.StateReady, Priority: 2}
	alpha.Tasks = []*models.Task{a1}
	beta.Tasks = []*models.Task{b1}

	m := newTestFocusBoard()
	m.tabs.Active = boardTabBacklog
	m.buildBacklogRows([]*models.Project{alpha, beta})

	m.applyFilter()
	if len(m.filtered) != 2 {
		t.Fatalf("unfiltered backlog = %d rows, want 2", len(m.filtered))
	}

	m.backlogProjectFilter = map[string]bool{alpha.ID: true}
	m.applyFilter()
	if len(m.filtered) != 1 || m.filtered[0].Task.ID != "a1" {
		t.Fatalf("project-filtered backlog = %v, want [a1]", boardRowIDs(m.filtered))
	}
	if !m.HasActiveFilters() {
		t.Fatal("HasActiveFilters should report the active project filter")
	}

	m.ClearAllFilters()
	m.applyFilter()
	if len(m.filtered) != 2 {
		t.Fatalf("cleared backlog = %d rows, want 2", len(m.filtered))
	}
}

func boardRowIDs(rows []boardRow) []string {
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.Task != nil {
			ids = append(ids, r.Task.ID)
		}
	}
	return ids
}
