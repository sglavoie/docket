package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

// This file proves the focus/review standard-task-keymap views (Focus View,
// Focus Board, Project Focus, Weekly Review) route their non-overridden standard
// keymap actions through the coordinator-backed workflow transaction path
// (TaskKeymapDeps.View -> wf* -> TaskItemCoordinator -> app.Service). Each test
// drives an action that the view does NOT override, so it exercises the shared
// adapter's default (batch) path, and asserts the change is durable on disk and
// reverts through the workspace undo ledger.

// TestFocusViewStandardKeymapCommitsWorkflowAndUndoes drives Focus View's
// non-overridden toggle-pin default through the workflow path.
func TestFocusViewStandardKeymapCommitsWorkflowAndUndoes(t *testing.T) {
	task := seedTask(t, 1, "Focus view task", models.StateReady)
	env, features := newSecondaryEnv(t, "focusviewwf", task)

	m := &FocusViewModel{ViewBundle: ViewBundle{config: testutil.NewTestConfig()}}
	m.taskItem = features.TaskItem
	m.filtered = []FocusTask{{Task: task, Project: env.project}}
	m.cursor = 0

	deps := m.focusTaskActionDeps()

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

// TestFocusBoardStandardKeymapCommitsWorkflowAndUndoes drives Focus Board's
// non-overridden priority-change default through the workflow path. Focus
// Board's post-commit advance/reload rides the same BatchTransactionResultMsg
// the shell handles via refreshTaskViewsAfterSave, so a committed batch message
// is what keeps that behavior intact after the migration.
func TestFocusBoardStandardKeymapCommitsWorkflowAndUndoes(t *testing.T) {
	task := seedTask(t, 1, "Focus board task", models.StateReady)
	env, features := newSecondaryEnv(t, "focusboardwf", task)

	m := &FocusBoardModel{config: testutil.NewTestConfig()}
	m.taskItem = features.TaskItem
	m.filtered = []boardRow{{Task: task, Project: env.project}}
	m.cursor = 0

	deps := m.focusBoardTaskKeymapDeps()

	// Priority 1 differs from the default (3), so the change commits (an
	// unchanged priority is a deliberate no-op in wfChangePriority).
	_, cmd := buildPriorityChangeAction(deps)("1")()
	msg := runBatchResult(t, cmd)
	if len(msg.TouchedProjectIDs) != 1 || msg.TouchedProjectIDs[0] != env.project.ID {
		t.Fatalf("touched projects = %v, want [%s]", msg.TouchedProjectIDs, env.project.ID)
	}
	if got := env.reload(t).GetTaskByID(task.ID); got == nil || got.Priority != 1 {
		t.Fatalf("reloaded Priority = %v, want 1", got)
	}
	env.undo(t)
	if got := env.reload(t).GetTaskByID(task.ID); got == nil || got.Priority != 3 {
		t.Fatalf("undo Priority = %v, want 3", got)
	}
}

// TestProjectFocusStandardKeymapCommitsWorkflowAndUndoes drives Project Focus's
// non-overridden status-change (mark-done) default through the workflow path.
func TestProjectFocusStandardKeymapCommitsWorkflowAndUndoes(t *testing.T) {
	task := seedTask(t, 1, "Project focus task", models.StateReady)
	env, features := newSecondaryEnv(t, "projectfocuswf", task)

	m := &ProjectFocusViewModel{cfg: testutil.NewTestConfig(), project: env.project}
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

	m := &WeeklyReviewModel{ViewBundle: ViewBundle{config: testutil.NewTestConfig()}}
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
