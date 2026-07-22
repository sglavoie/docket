package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestPlannerBacklogRankingIgnoresTodayAndNextCandidates proves that hidden
// Today (daily plan) and Next (StateNext) rows cannot change the rank or score
// of the visible Backlog.
//
// RankCrossProjectTasks applies a cumulative per-project balance penalty
// (-projectCount*300) walking the score-sorted slice. If Today/Next rows were
// ranked alongside the Backlog and only excluded afterward, each hidden
// same-project row would consume a penalty slot and push the project's visible
// Backlog task further down — reordering it relative to another project's row.
// buildBacklogRows must therefore filter the candidate set to actionable tasks
// outside Today and Next BEFORE ranking. This regression seeds enough hidden
// same-project rows to trip the penalty and asserts the visible Backlog order is
// invariant to their presence.
func TestPlannerBacklogRankingIgnoresTodayAndNextCandidates(t *testing.T) {
	projA, _ := models.NewProject("proj-a", "Project A", "/tmp/planner-a")
	projB, _ := models.NewProject("proj-b", "Project B", "/tmp/planner-b")

	// Two visible Backlog candidates in different projects, scored close
	// together: aVis (with a bug bonus) edges out bVis by 100 points, so the
	// unpenalized ranked order is [aVis, bVis].
	aVis := &models.Task{ID: "a-vis", ShortID: 1, Type: models.TaskTypeBug, Title: "A visible", WorkflowState: models.StateReady, Priority: 3}
	bVis := &models.Task{ID: "b-vis", ShortID: 2, Type: models.TaskTypeTask, Title: "B visible", WorkflowState: models.StateReady, Priority: 3}

	// Hidden same-project (Project A) rows that rank ABOVE aVis: three StateNext
	// rows (Next horizon) and two daily-planned rows (Today horizon), all
	// priority 1 so they sort first among Project A tasks. Under the old
	// rank-then-exclude flow these five rows would drive aVis's balance penalty
	// to 5*300 = 1500, sinking it well below bVis and flipping the visible order.
	next1 := &models.Task{ID: "a-next-1", ShortID: 3, Type: models.TaskTypeTask, Title: "A next 1", WorkflowState: models.StateNext, Priority: 1}
	next2 := &models.Task{ID: "a-next-2", ShortID: 4, Type: models.TaskTypeTask, Title: "A next 2", WorkflowState: models.StateNext, Priority: 1}
	next3 := &models.Task{ID: "a-next-3", ShortID: 5, Type: models.TaskTypeTask, Title: "A next 3", WorkflowState: models.StateNext, Priority: 1}
	today1 := &models.Task{ID: "a-today-1", ShortID: 6, Type: models.TaskTypeTask, Title: "A today 1", WorkflowState: models.StateReady, Priority: 1}
	today2 := &models.Task{ID: "a-today-2", ShortID: 7, Type: models.TaskTypeTask, Title: "A today 2", WorkflowState: models.StateReady, Priority: 1}

	// Baseline: only the two visible candidates exist. This is the canonical
	// ranked order the visible Backlog must always show.
	baseline := newTestFocusBoard()
	projA.Tasks = []*models.Task{aVis}
	projB.Tasks = []*models.Task{bVis}
	baseline.buildBacklogRows([]*models.Project{projA, projB})
	baselineIDs := boardRowIDs(baseline.backlogRows)

	want := []string{"a-vis", "b-vis"}
	if len(baselineIDs) != len(want) || baselineIDs[0] != want[0] || baselineIDs[1] != want[1] {
		t.Fatalf("baseline backlog order = %v, want %v", baselineIDs, want)
	}

	// With hidden Today/Next rows present: same visible candidate set, but now
	// Project A also carries five higher-scoring Today/Next rows. The visible
	// Backlog order must be identical to the baseline.
	withHidden := newTestFocusBoard()
	projA.Tasks = []*models.Task{aVis, next1, next2, next3, today1, today2}
	projB.Tasks = []*models.Task{bVis}
	withHidden.dailyPlanIDs[taskIdentityFor(today1, projA)] = true
	withHidden.dailyPlanIDs[taskIdentityFor(today2, projA)] = true
	withHidden.buildBacklogRows([]*models.Project{projA, projB})
	hiddenIDs := boardRowIDs(withHidden.backlogRows)

	if len(hiddenIDs) != len(baselineIDs) {
		t.Fatalf("backlog membership changed with hidden horizons: got %v, want %v", hiddenIDs, baselineIDs)
	}
	for i := range baselineIDs {
		if hiddenIDs[i] != baselineIDs[i] {
			t.Fatalf("hidden Today/Next rows reordered the visible Backlog: got %v, want %v", hiddenIDs, baselineIDs)
		}
	}
}
