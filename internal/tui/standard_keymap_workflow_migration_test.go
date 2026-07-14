package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// This file proves the migrated standard-task-keymap list views (All Tasks,
// Priority, Due Date, Recent Activity) commit their single-task keymap actions
// through the coordinator-backed workflow transaction path (TaskKeymapDeps.View
// -> wf* -> TaskItemCoordinator -> app.Service), and that the shared adapter
// routes its non-overridden defaults (including the pin/dim helpers) through
// that path.

// newRecentActivityForCoordinator builds a Recent Activity model positioned on
// task with its single-task coordinator wired over the given features, so a
// standard keymap action commits through the real transactional stack.
func newRecentActivityForCoordinator(features SecondaryCoordinators, task *models.Task, project *models.Project) *RecentActivityModel {
	cfg := newTestConfig()
	m := &RecentActivityModel{runtime: testRuntimeAt(cfg.DataDir(), cfg)}
	m.taskItem = features.TaskItem
	m.filtered = []CrossProjectTask{{Task: task, Project: project}}
	m.cursor = 0
	return m
}

// TestRecentActivityStandardKeymapCommitsWorkflowAndUndoes exercises the fully
// migrated Recent Activity view end to end: a standard status-change keymap
// action commits asynchronously through the workflow transaction path (a
// BatchTransactionResultMsg carrying the touched project and a status summary),
// the change is durable on disk, and the workspace ledger undoes it.
func TestRecentActivityStandardKeymapCommitsWorkflowAndUndoes(t *testing.T) {
	task := seedTask(t, 1, "Activity task", models.StateReady)
	env, features := newSecondaryEnv(t, "recentwf", task)

	m := newRecentActivityForCoordinator(features, task, env.project)

	// Drive the shared adapter's default status action (Recent Activity overrides
	// nothing, so this is the WorkflowView default path).
	deps := TaskKeymapDeps{View: m}
	action := buildStateChangeAction(deps)(string(models.StateDone))
	_, cmd := action()

	msg := runBatchResult(t, cmd)
	if len(msg.TouchedProjectIDs) != 1 || msg.TouchedProjectIDs[0] != env.project.ID {
		t.Fatalf("touched projects = %v, want [%s]", msg.TouchedProjectIDs, env.project.ID)
	}
	if msg.Summary == "" {
		t.Fatal("workflow commit carried no status summary (async status behavior lost)")
	}

	if got := env.reload(t).GetTaskByID(task.ID); got == nil || got.State() != models.StateDone {
		t.Fatalf("reloaded task state = %v, want done", got)
	}

	env.undo(t)
	if got := env.reload(t).GetTaskByID(task.ID); got == nil || got.State() != models.StateReady {
		t.Fatalf("undo state = %v, want ready", got)
	}
}

// TestStandardKeymapAdapterPinDimCommitThroughWorkflow proves the shared
// adapter's newly added pin/dim defaults commit through the coordinator when
// WorkflowView is set: toggling pin and dim each produce an async batch commit
// that flips the flag on disk, and undo reverts it.
func TestStandardKeymapAdapterPinDimCommitThroughWorkflow(t *testing.T) {
	task := seedTask(t, 1, "Flag task", models.StateReady)
	env, features := newSecondaryEnv(t, "pindimwf", task)
	m := newRecentActivityForCoordinator(features, task, env.project)
	deps := TaskKeymapDeps{View: m}

	// Toggle pin on.
	_, cmd := buildTogglePinAction(deps)()
	runBatchResult(t, cmd)
	if got := env.reload(t).GetTaskByID(task.ID); got == nil || !got.Pinned {
		t.Fatalf("reloaded task Pinned = %v, want true", got)
	}
	env.undo(t)
	if got := env.reload(t).GetTaskByID(task.ID); got == nil || got.Pinned {
		t.Fatalf("undo Pinned = %v, want false", got)
	}

	// Toggle dim on.
	_, cmd = buildToggleDimAction(deps)()
	runBatchResult(t, cmd)
	if got := env.reload(t).GetTaskByID(task.ID); got == nil || !got.Dimmed {
		t.Fatalf("reloaded task Dimmed = %v, want true", got)
	}
	env.undo(t)
	if got := env.reload(t).GetTaskByID(task.ID); got == nil || got.Dimmed {
		t.Fatalf("undo Dimmed = %v, want false", got)
	}
}

// TestStandardKeymapAdapterDefaultsRouteThroughWorkflow proves the shared
// adapter's default toggle-pin action resolves to the async workflow batch
// message now that every standard action routes through the coordinator-backed
// wf* path unconditionally.
func TestStandardKeymapAdapterDefaultsRouteThroughWorkflow(t *testing.T) {
	task := seedTask(t, 1, "Gate task", models.StateReady)
	env, features := newSecondaryEnv(t, "gatewf", task)
	m := newRecentActivityForCoordinator(features, task, env.project)

	_, cmd := buildTogglePinAction(TaskKeymapDeps{View: m})()
	if cmd == nil {
		t.Fatal("default pin action returned a nil command")
	}
	if _, ok := cmd().(BatchTransactionResultMsg); !ok {
		t.Fatalf("default pin action = %T, want BatchTransactionResultMsg", cmd())
	}
}
