package focus

import (
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
)

// TestRankCrossProjectTasksDeterministicTies guards that tied scores produce a
// stable ordering across repeated invocations. Without an explicit tiebreaker,
// the relative order of equal-score tasks depends on the input slice order,
// which itself depends on map iteration (random) when callers build the slice
// from a map. The UI list must not shuffle rows on each refresh.
func TestRankCrossProjectTasksDeterministicTies(t *testing.T) {
	project, err := models.NewProject(uuid.New().String(), "P", "")
	if err != nil {
		t.Fatal(err)
	}

	// Two tasks with identical scoring inputs ⇒ identical scores.
	mk := func(id string) FocusTask {
		task, err := models.NewTask(id, 1, models.TaskTypeTask, "T-"+id)
		if err != nil {
			t.Fatal(err)
		}
		task.Priority = 3
		return FocusTask{Task: task, Project: project}
	}

	a := mk("aaa")
	b := mk("bbb")
	c := mk("ccc")

	// Run with several input orderings; output order must be invariant.
	inputs := [][]FocusTask{
		{a, b, c},
		{c, b, a},
		{b, c, a},
		{a, c, b},
	}
	var firstIDs []string
	for i, in := range inputs {
		ranked := RankCrossProjectTasks(in, nil, nil)
		if len(ranked) != 3 {
			t.Fatalf("input %d: expected 3 ranked tasks, got %d", i, len(ranked))
		}
		ids := []string{ranked[0].Task.ID, ranked[1].Task.ID, ranked[2].Task.ID}
		if i == 0 {
			firstIDs = ids
			continue
		}
		for j := range ids {
			if ids[j] != firstIDs[j] {
				t.Fatalf("input %d: order %v differs from baseline %v", i, ids, firstIDs)
			}
		}
	}

	// And the deterministic order should be ID-ascending.
	want := []string{"aaa", "bbb", "ccc"}
	for i, id := range firstIDs {
		if id != want[i] {
			t.Errorf("expected ID-ascending tiebreak %v, got %v", want, firstIDs)
			break
		}
	}
}
