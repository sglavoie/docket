package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// This file proves the remaining focus/review standard-task-keymap views route their non-overridden standard
// keymap actions through the coordinator-backed workflow transaction path
// (TaskKeymapDeps.View -> wf* -> TaskItemCoordinator -> app.Service). Each test
// drives an action that the view does NOT override, so it exercises the shared
// adapter's default (batch) path, and asserts the change is durable on disk and
// reverts through the workspace undo ledger.

// TestProjectFocusStandardKeymapCommitsWorkflowAndUndoes drives Project Focus's
// non-overridden status-change (mark-done) default through the workflow path.
func TestProjectFocusStandardKeymapCommitsWorkflowAndUndoes(t *testing.T) {
	task := seedTask(t, 1, "Project focus task", models.StateReady)
	env, features := newSecondaryEnv(t, "projectfocuswf", task)

	cfg := newTestConfig()
	m := &ProjectFocusViewModel{runtime: testRuntimeAt(cfg.DataDir(), cfg), project: env.project}
	m.taskItem = features.TaskItem
	m.tasks = []*models.Task{task}
	m.cursor = 0

	hasItem := func() bool { return m.getTaskAtCursor() != nil }
	deps := m.projectFocusTaskKeymapDeps(hasItem)

	_, cmd := buildStateChangeAction(deps)(string(models.StatusDone))()
	msg := runBatchResult(t, cmd)
	if len(msg.TouchedProjectIDs) != 1 || msg.TouchedProjectIDs[0] != env.project.ID {
		t.Fatalf("touched projects = %v, want [%s]", msg.TouchedProjectIDs, env.project.ID)
	}
	if got := env.reload(t).GetTaskByID(task.ID); got == nil || got.State() != models.StateDone {
		t.Fatalf("reloaded state = %v, want done", got)
	}
	env.undo(t)
	if got := env.reload(t).GetTaskByID(task.ID); got == nil || got.State() != models.StateReady {
		t.Fatalf("undo state = %v, want ready", got)
	}
}

// TestWeeklyReviewStandardKeymapCommitsWorkflowAndUndoes drives Weekly Review's
// non-overridden toggle-pin default through the workflow path, positioned on a
// task in the Stale Tasks section.
func TestWeeklyReviewStandardKeymapCommitsWorkflowAndUndoes(t *testing.T) {
	task := seedTask(t, 1, "Weekly review task", models.StateReady)
	env, features := newSecondaryEnv(t, "weeklyreviewwf", task)

	cfg := newTestConfig()
	m := &WeeklyReviewModel{PrimaryViewBundle: NewPrimaryViewBundle(nil, testRuntimeAt(cfg.DataDir(), cfg))}
	m.taskItem = features.TaskItem
	m.staleTasks = []WeeklyReviewTask{{Task: task, Project: env.project}}
	m.sectionOrder = []WeeklyReviewSection{WeeklySectionStaleTasks}
	m.collapsedSections = map[int]bool{}
	m.mutationCounts = map[string]int{}
	m.totalItems = 2 // section header + one task
	m.cursor = 1     // 0 is the section header, 1 is the task

	if m.getTaskAtCursor() == nil {
		t.Fatal("weekly review cursor not positioned on the seeded task")
	}

	deps := weeklyReviewTaskActionDeps(m)

	_, cmd := buildTogglePinAction(deps)()
	msg := runBatchResult(t, cmd)
	if len(msg.TouchedProjectIDs) != 1 || msg.TouchedProjectIDs[0] != env.project.ID {
		t.Fatalf("touched projects = %v, want [%s]", msg.TouchedProjectIDs, env.project.ID)
	}
	if got := env.reload(t).GetTaskByID(task.ID); got == nil || !got.Pinned {
		t.Fatalf("reloaded Pinned = %v, want true", got)
	}
	env.undo(t)
	if got := env.reload(t).GetTaskByID(task.ID); got == nil || got.Pinned {
		t.Fatalf("undo Pinned = %v, want false", got)
	}
}
