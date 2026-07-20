package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// TestFocusBoardDoneTabCompletedTodayOnly locks the Planner Done tab to the
// "closed today" horizon: only terminal tasks whose CompletedAt falls on the
// current local day appear, older terminal work is excluded, and legacy
// terminal tasks with a nil CompletedAt are excluded explicitly. Both StateDone
// and StateCanceled are exercised across today and prior-day timestamps.
func TestFocusBoardDoneTabCompletedTodayOnly(t *testing.T) {
	today := util.StartOfDay(time.Now())
	todayAfternoon := today.Add(15 * time.Hour)
	yesterday := today.AddDate(0, 0, -1).Add(15 * time.Hour)

	newTerminal := func(id string, state models.WorkflowState, completedAt *time.Time) *models.Task {
		task := newDenseRowTask(t, id, 1, id)
		task.WorkflowState = state
		task.CompletedAt = completedAt
		if !task.IsTerminalState() {
			t.Fatalf("task %q expected terminal state, got %v", id, state)
		}
		return task
	}

	doneToday := newTerminal("done-today", models.StateDone, &todayAfternoon)
	doneYesterday := newTerminal("done-yesterday", models.StateDone, &yesterday)
	canceledToday := newTerminal("canceled-today", models.StateCanceled, &todayAfternoon)
	canceledYesterday := newTerminal("canceled-yesterday", models.StateCanceled, &yesterday)
	doneNilCompletedAt := newTerminal("done-legacy", models.StateDone, nil)
	canceledNilCompletedAt := newTerminal("canceled-legacy", models.StateCanceled, nil)

	cases := []struct {
		task *models.Task
		want bool
	}{
		{doneToday, true},
		{canceledToday, true},
		{doneYesterday, false},
		{canceledYesterday, false},
		{doneNilCompletedAt, false},
		{canceledNilCompletedAt, false},
	}

	// The helper is the single decision loadItems consults when populating the
	// Done (closedRows) slice, so asserting it here pins the horizon.
	for _, tc := range cases {
		if got := terminalTaskClosedToday(tc.task, today); got != tc.want {
			t.Errorf("terminalTaskClosedToday(%q) = %v, want %v", tc.task.ID, got, tc.want)
		}
	}

	// Non-terminal and nil tasks must never enter the Done tab.
	if terminalTaskClosedToday(nil, today) {
		t.Error("nil task should not be a Done-tab candidate")
	}
	active := newDenseRowTask(t, "active", 2, "active")
	active.CompletedAt = &todayAfternoon
	if terminalTaskClosedToday(active, today) {
		t.Error("non-terminal task should not be a Done-tab candidate even with CompletedAt today")
	}

	// Integration: mirror loadItems' filter, then render the Done tab and assert
	// the displayed slice carries only today's terminal tasks.
	project := newDenseRowProject(t)
	var closed []boardRow
	for _, tc := range cases {
		if terminalTaskClosedToday(tc.task, today) {
			closed = append(closed, boardRow{Task: tc.task, Project: project, Source: project.Name})
		}
	}
	m := cockpitModel(nil, nil, closed, nil, OpenLoops{}, boardTabDone)

	got := make(map[string]bool, len(m.filtered))
	for _, r := range m.filtered {
		if r.Task != nil {
			got[r.Task.ID] = true
		}
	}
	if len(got) != 2 || !got["done-today"] || !got["canceled-today"] {
		t.Fatalf("Done tab should show only today's terminal tasks, got %v", got)
	}
}
