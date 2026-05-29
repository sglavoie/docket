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

// TestRankCrossProjectTasksDeterministicTiesWithDuplicateTaskIDs guards that
// tied tasks remain stable even when separate projects contain the same task
// ID. That can happen with imported or hand-authored data, and task ID alone
// is not a strict ordering across projects.
func TestRankCrossProjectTasksDeterministicTiesWithDuplicateTaskIDs(t *testing.T) {
	projectA, err := models.NewProject("project-a", "A", "")
	if err != nil {
		t.Fatal(err)
	}
	projectB, err := models.NewProject("project-b", "B", "")
	if err != nil {
		t.Fatal(err)
	}

	mk := func(project *models.Project) FocusTask {
		task, taskErr := models.NewTask("same-task-id", 1, models.TaskTypeTask, "T")
		if taskErr != nil {
			t.Fatal(taskErr)
		}
		task.Priority = 3
		return FocusTask{Task: task, Project: project}
	}

	inputs := [][]FocusTask{
		{mk(projectA), mk(projectB)},
		{mk(projectB), mk(projectA)},
	}

	var firstProjectIDs []string
	for i, in := range inputs {
		ranked := RankCrossProjectTasks(in, nil, nil)
		if len(ranked) != 2 {
			t.Fatalf("input %d: expected 2 ranked tasks, got %d", i, len(ranked))
		}
		projectIDs := []string{ranked[0].Project.ID, ranked[1].Project.ID}
		if i == 0 {
			firstProjectIDs = projectIDs
			continue
		}
		for j := range projectIDs {
			if projectIDs[j] != firstProjectIDs[j] {
				t.Fatalf("input %d: order %v differs from baseline %v", i, projectIDs, firstProjectIDs)
			}
		}
	}

	want := []string{"project-a", "project-b"}
	for i, id := range firstProjectIDs {
		if id != want[i] {
			t.Errorf("expected project ID tiebreak %v, got %v", want, firstProjectIDs)
			break
		}
	}
}
