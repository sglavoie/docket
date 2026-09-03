package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// This file exercises the single-task TaskItemCoordinator
// (feature_coordinator_task_item.go) end to end through the real transactional
// storage stack and undo ledger, mirroring the bulk-coordinator scenarios. The
// coordinator is the additive migration seam for the cross-project single-task
// raw-save family (session 33.7.3.4.4.1); no production caller routes through it
// yet, so these scenarios drive it directly.

// TestTaskItemTransitionScenario covers the canonical state-transition seam.
func TestTaskItemTransitionScenario(t *testing.T) {
	task := seedTask(t, 1, "Park me", models.StateBacklog)
	env, features := newSecondaryEnv(t, "state", task)

	runResult(t, features.TaskItem.Transition(taskref.For(task, env.project), models.StateSomeday))
	got := env.reload(t).GetTaskByID(task.ID)
	if got == nil || got.State() != models.StateSomeday {
		t.Fatalf("reloaded state = %v, want %q", got, models.StateSomeday)
	}
	if got.State() != models.StateSomeday {
		t.Errorf("reloaded state = %q, want %q", got.State(), models.StateSomeday)
	}
}

// reloadedRootOrder reads the root-task titles in slice order from a reloaded
// project so an assertion sees the committed ordering. Titles (not short IDs) are
// the stable identity across a reload: the v1 markdown round-trip renumbers short
// IDs by document position, so the moved task's short ID changes even though its
// relative order is what the move committed.
func reloadedRootOrder(p *models.Project) []string {
	var titles []string
	for _, t := range p.Tasks {
		if t.IsRoot() {
			titles = append(titles, t.Title)
		}
	}
	return titles
}

// TestTaskItemMoveToTopScenario covers the reorder seam: moving a task to the top
// commits a project-scoped batch transaction and the reloaded order reflects it.
func TestTaskItemMoveToTopScenario(t *testing.T) {
	a := seedTask(t, 1, "A", models.StateBacklog)
	b := seedTask(t, 2, "B", models.StateBacklog)
	c := seedTask(t, 3, "C", models.StateBacklog)
	env, features := newSecondaryEnv(t, "reorder", a, b, c)

	msg := runBatchResult(t, features.TaskItem.MoveToTop(taskref.For(c, env.project)))
	if len(msg.TouchedProjectIDs) != 1 {
		t.Fatalf("touched projects = %v, want 1", msg.TouchedProjectIDs)
	}
	if msg.Summary != "Moved #3 to top" {
		t.Errorf("summary = %q, want %q", msg.Summary, "Moved #3 to top")
	}
	if got := reloadedRootOrder(env.reload(t)); !equalStrings(got, []string{"C", "A", "B"}) {
		t.Errorf("reloaded order = %v, want [C A B]", got)
	}
}

// TestTaskItemMoveToBottomScenario is the mirror of the move-to-top scenario.
func TestTaskItemMoveToBottomScenario(t *testing.T) {
	a := seedTask(t, 1, "A", models.StateBacklog)
	b := seedTask(t, 2, "B", models.StateBacklog)
	c := seedTask(t, 3, "C", models.StateBacklog)
	env, features := newSecondaryEnv(t, "reorder", a, b, c)

	msg := runBatchResult(t, features.TaskItem.MoveToBottom(taskref.For(a, env.project)))
	if msg.Summary != "Moved #1 to bottom" {
		t.Errorf("summary = %q, want %q", msg.Summary, "Moved #1 to bottom")
	}
	if got := reloadedRootOrder(env.reload(t)); !equalStrings(got, []string{"B", "C", "A"}) {
		t.Errorf("reloaded order = %v, want [B C A]", got)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
