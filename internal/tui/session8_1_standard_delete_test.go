package tui

import (
	"slices"
	"testing"
)

// Session 8.1 repair (canonical-standard-delete): every standard uppercase-X
// task action must emit the shell-owned TaskDeleteRequestedMsg (which prompts
// before any mutation) instead of committing a delete directly, and the
// selection-aware surfaces must target the full active selection rather than
// only the cursor task. These tests drive the built keymap's "X" action and
// assert the emitted canonical request. deleteRequestFromCmd / requestTaskIDs
// live in session3_uppercase_delete_test.go.

// --- FR-005: standard surfaces emit the canonical request (no bypass) --------

// --- FR-006: remaining surfaces honor their advertised X target --------------

// TestWeeklyReviewUppercaseXDeleteTargetsSelection proves Weekly Review targets
// the full active multi-selection (not just the cursor) through the canonical
// request.
func TestWeeklyReviewUppercaseXDeleteTargetsSelection(t *testing.T) {
	project, tasks := newSharedBulkProject("Weekly", "one", "two", "three")
	m := &WeeklyReviewModel{
		MultiSelectState: MultiSelectState{
			multiSelectMode: true,
			selectedTaskIdentities: map[TaskIdentity]bool{
				taskIdentityFor(tasks[1], project): true,
				taskIdentityFor(tasks[2], project): true,
			},
		},
		staleTasks: []WeeklyReviewTask{
			{Task: tasks[0], Project: project},
			{Task: tasks[1], Project: project},
			{Task: tasks[2], Project: project},
		},
	}

	wantIDs := []string{tasks[1].ID, tasks[2].ID}
	slices.Sort(wantIDs)
	action := findSharedTaskKeymapAction(t, buildWeeklyReviewKeyMap(m), "X")
	_, cmd := action()
	msg := deleteRequestFromCmd(t, cmd)
	if got := requestTaskIDs(msg); !slices.Equal(got, wantIDs) {
		t.Fatalf("weekly review delete targets = %v, want the selection %v", got, wantIDs)
	}
	if msg.Origin != ViewWeeklyReview {
		t.Errorf("origin = %v, want ViewWeeklyReview", msg.Origin)
	}
}
