package tui

import (
	"sort"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestApplySortDirection_StrictWeakOrdering verifies that ApplySortDirection
// satisfies strict weak ordering: Less(a, b) and Less(b, a) cannot both be
// true. Prior to the fix, descending mode treated equal items (c == 0) as
// "less than" in both directions, corrupting sort.Slice.
func TestApplySortDirection_StrictWeakOrdering(t *testing.T) {
	cases := []struct {
		c   int
		asc bool
	}{
		{c: -1, asc: true},
		{c: 0, asc: true},
		{c: 1, asc: true},
		{c: -1, asc: false},
		{c: 0, asc: false},
		{c: 1, asc: false},
	}
	for _, tc := range cases {
		forward := ApplySortDirection(tc.c, tc.asc)
		reverse := ApplySortDirection(-tc.c, tc.asc)
		if forward && reverse {
			t.Errorf("strict weak ordering violated for c=%d asc=%v: Less(a,b)=Less(b,a)=true", tc.c, tc.asc)
		}
		if tc.c == 0 && (forward || reverse) {
			t.Errorf("equal items must be Less==false, got forward=%v reverse=%v asc=%v", forward, reverse, tc.asc)
		}
	}
}

// TestCompareTasks_DeterministicTieBreak verifies that two tasks identical
// on the primary sort key are ordered deterministically by ID.
func TestCompareTasks_DeterministicTieBreak(t *testing.T) {
	a := &models.Task{ID: "a", Priority: 3, WorkflowState: models.StateReady}
	b := &models.Task{ID: "b", Priority: 3, WorkflowState: models.StateReady}

	if c := CompareTasks(a, b, SortByPriority); c >= 0 {
		t.Errorf("expected CompareTasks(a,b) < 0 (tie broken by ID), got %d", c)
	}
	if c := CompareTasks(b, a, SortByPriority); c <= 0 {
		t.Errorf("expected CompareTasks(b,a) > 0 (tie broken by ID), got %d", c)
	}
}

// TestSortDescending_DoesNotPanicOnTies verifies that a descending sort
// over many tasks with identical primary keys completes and produces a
// stable, deterministic order. Without the fix, sort.Slice would treat
// equal items as both "less than" each other and could yield garbage or
// loop indefinitely depending on the implementation.
func TestSortDescending_DoesNotPanicOnTies(t *testing.T) {
	tasks := []*models.Task{
		{ID: "a", Priority: 2, WorkflowState: models.StateReady},
		{ID: "b", Priority: 2, WorkflowState: models.StateReady},
		{ID: "c", Priority: 2, WorkflowState: models.StateReady},
		{ID: "d", Priority: 2, WorkflowState: models.StateReady},
		{ID: "e", Priority: 2, WorkflowState: models.StateReady},
	}

	sort.Slice(tasks, func(i, j int) bool {
		c := CompareTasks(tasks[i], tasks[j], SortByPriority)
		return ApplySortDirection(c, false)
	})

	// Descending sort with ID tiebreak should produce IDs in reverse-alpha order.
	want := []string{"e", "d", "c", "b", "a"}
	for i, w := range want {
		if tasks[i].ID != w {
			t.Errorf("position %d: got %q, want %q (full=%v)", i, tasks[i].ID, w, idsOf(tasks))
			return
		}
	}
}

func idsOf(tasks []*models.Task) []string {
	ids := make([]string, len(tasks))
	for i, t := range tasks {
		ids[i] = t.ID
	}
	return ids
}
