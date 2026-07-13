package tui

import (
	"testing"

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

func TestHandleDailyPlanReorder_FilteredSubset(t *testing.T) {
	// This tests that reordering works correctly when filtered is a subset
	// of todayRows (e.g., due to search/filter). The swap in todayRows must
	// use task-ID-based lookup, not cursor position.
	project, _ := models.NewProject("proj", "proj", "/tmp/test-proj")

	taskA := makeBoardTask(t, "id-a", 1, "Alpha")
	taskB := makeBoardTask(t, "id-b", 2, "Beta")
	taskC := makeBoardTask(t, "id-c", 3, "Charlie")

	rowA := boardRow{Task: taskA, Project: project, Source: project.Name}
	rowB := boardRow{Task: taskB, Project: project, Source: project.Name}
	rowC := boardRow{Task: taskC, Project: project, Source: project.Name}

	dailyPlan := []FocusTask{
		{Task: taskA, Project: project},
		{Task: taskB, Project: project},
		{Task: taskC, Project: project},
	}

	m := &FocusBoardModel{
		todayRows: []boardRow{rowA, rowB, rowC},
		// Simulate a filtered view where only A and C are visible (B filtered out)
		filtered:  []boardRow{rowA, rowC},
		dailyPlan: &dailyPlan,
		tabs:      TabState{Active: boardTabToday},
	}

	// Cursor on first filtered item (Alpha), reorder down to swap with Charlie
	m.cursor = 0
	m.handleDailyPlanReorder(1)

	// After reorder: filtered should be [Charlie, Alpha]
	if m.filtered[0].Task.ID != "id-c" {
		t.Errorf("filtered[0]: expected id-c, got %s", m.filtered[0].Task.ID)
	}
	if m.filtered[1].Task.ID != "id-a" {
		t.Errorf("filtered[1]: expected id-a, got %s", m.filtered[1].Task.ID)
	}

	// todayRows should swap Alpha and Charlie by ID, not by position.
	// Original todayRows: [A, B, C] → should become [C, B, A]
	if m.todayRows[0].Task.ID != "id-c" {
		t.Errorf("todayRows[0]: expected id-c, got %s", m.todayRows[0].Task.ID)
	}
	if m.todayRows[1].Task.ID != "id-b" {
		t.Errorf("todayRows[1]: expected id-b (unchanged), got %s", m.todayRows[1].Task.ID)
	}
	if m.todayRows[2].Task.ID != "id-a" {
		t.Errorf("todayRows[2]: expected id-a, got %s", m.todayRows[2].Task.ID)
	}

	// Cursor should have moved to position 1
	if m.cursor != 1 {
		t.Errorf("cursor: expected 1, got %d", m.cursor)
	}

	// dailyPlan should also have swapped A and C
	dp := *m.dailyPlan
	if dp[0].Task.ID != "id-c" {
		t.Errorf("dailyPlan[0]: expected id-c, got %s", dp[0].Task.ID)
	}
	if dp[2].Task.ID != "id-a" {
		t.Errorf("dailyPlan[2]: expected id-a, got %s", dp[2].Task.ID)
	}
}

func TestHandleDailyPlanReorder_NoFilterActive(t *testing.T) {
	// When no filters are active, filtered == todayRows copy, so the swap
	// should still work correctly with the ID-based lookup.
	project, _ := models.NewProject("proj", "proj", "/tmp/test-proj")

	taskA := makeBoardTask(t, "id-a", 1, "Alpha")
	taskB := makeBoardTask(t, "id-b", 2, "Beta")

	rowA := boardRow{Task: taskA, Project: project, Source: project.Name}
	rowB := boardRow{Task: taskB, Project: project, Source: project.Name}

	dailyPlan := []FocusTask{
		{Task: taskA, Project: project},
		{Task: taskB, Project: project},
	}

	m := &FocusBoardModel{
		todayRows: []boardRow{rowA, rowB},
		filtered:  []boardRow{rowA, rowB},
		dailyPlan: &dailyPlan,
		tabs:      TabState{Active: boardTabToday},
	}

	m.cursor = 0
	m.handleDailyPlanReorder(1)

	if m.filtered[0].Task.ID != "id-b" {
		t.Errorf("filtered[0]: expected id-b, got %s", m.filtered[0].Task.ID)
	}
	if m.filtered[1].Task.ID != "id-a" {
		t.Errorf("filtered[1]: expected id-a, got %s", m.filtered[1].Task.ID)
	}
	if m.todayRows[0].Task.ID != "id-b" {
		t.Errorf("todayRows[0]: expected id-b, got %s", m.todayRows[0].Task.ID)
	}
	if m.todayRows[1].Task.ID != "id-a" {
		t.Errorf("todayRows[1]: expected id-a, got %s", m.todayRows[1].Task.ID)
	}
}

func TestHandleDailyPlanReorder_WrongTab(t *testing.T) {
	// Reorder should be a no-op on non-Today tabs.
	m := &FocusBoardModel{
		tabs: TabState{Active: boardTabNext},
	}
	m.cursor = 0
	model, cmd := m.handleDailyPlanReorder(1)
	if model != m {
		t.Error("expected same model returned")
	}
	if cmd != nil {
		t.Error("expected nil cmd")
	}
}

func TestHandleDailyPlanReorder_CompletedCrossover(t *testing.T) {
	// Should not swap between completed and non-completed tasks.
	project, _ := models.NewProject("proj", "proj", "/tmp/test-proj")

	taskA := makeBoardTask(t, "id-a", 1, "Active")
	taskB := makeBoardTask(t, "id-b", 2, "Done")
	taskB.WorkflowState = models.StateDone

	rowA := boardRow{Task: taskA, Project: project, Source: project.Name}
	rowB := boardRow{Task: taskB, Project: project, Source: project.Name}

	dailyPlan := []FocusTask{
		{Task: taskA, Project: project},
		{Task: taskB, Project: project},
	}

	m := &FocusBoardModel{
		todayRows: []boardRow{rowA, rowB},
		filtered:  []boardRow{rowA, rowB},
		dailyPlan: &dailyPlan,
		tabs:      TabState{Active: boardTabToday},
	}

	m.cursor = 0
	m.handleDailyPlanReorder(1)

	// Should be unchanged — can't swap active with completed
	if m.filtered[0].Task.ID != "id-a" {
		t.Errorf("filtered[0]: expected id-a (unchanged), got %s", m.filtered[0].Task.ID)
	}
	if m.cursor != 0 {
		t.Errorf("cursor: expected 0 (unchanged), got %d", m.cursor)
	}
}

func TestSkipGroupHeaderForward_TrailingHeaderFallsBack(t *testing.T) {
	project, _ := models.NewProject("proj", "proj", "/tmp/test-proj")
	taskA := makeBoardTask(t, "id-a", 1, "Alpha")

	rowA := boardRow{Task: taskA, Project: project, Source: project.Name}
	headerRow := boardRow{GroupHeader: "Section"}

	m := &FocusBoardModel{
		filtered: []boardRow{rowA, headerRow, headerRow},
	}

	m.cursor = 1
	m.skipGroupHeaderForward()
	if m.cursor != 0 {
		t.Errorf("cursor on trailing headers should fall back to task row, got %d", m.cursor)
	}
	if m.filtered[m.cursor].GroupHeader != "" {
		t.Errorf("cursor landed on group header at %d", m.cursor)
	}
}

func TestSkipGroupHeaderForward_AllHeadersClampsToZero(t *testing.T) {
	headerRow := boardRow{GroupHeader: "Section"}
	m := &FocusBoardModel{
		filtered: []boardRow{headerRow, headerRow},
	}
	m.cursor = 0
	m.skipGroupHeaderForward()
	if m.cursor < 0 || m.cursor >= len(m.filtered) {
		t.Errorf("cursor out of range: %d", m.cursor)
	}
}
