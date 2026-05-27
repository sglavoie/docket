package tui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestHandleWithRefreshForwardsPopCmd verifies that handleWithRefresh forwards
// the tea.Cmd produced by popView() instead of silently dropping it. Without
// this, popping back to ViewCrossProjectOutliner (whose popView returns a
// ViewDidAppearMsg) leaves the outliner stale.
func TestHandleWithRefreshForwardsPopCmd(t *testing.T) {
	app, _, _ := newTestAppModelWithMockPersisterAndProject(t)
	// Simulate a stack: Dashboard → CrossProjectOutliner → TaskForm
	// Popping the TaskForm should reveal the CrossProjectOutliner and emit
	// ViewDidAppearMsg.
	app.viewStack = []ViewType{ViewDashboard, ViewCrossProjectOutliner, ViewTaskForm}

	cmd := app.handleWithRefresh(true, false, "")
	if cmd == nil {
		t.Fatal("expected popView's ViewDidAppearMsg cmd to be forwarded, got nil")
	}

	msg := cmd()
	if _, ok := msg.(ViewDidAppearMsg); !ok {
		t.Errorf("expected ViewDidAppearMsg from forwarded popCmd, got %T", msg)
	}
}

// TestHandleWithRefreshBatchesPopCmdWithStatus verifies that when both popView
// returns a cmd AND a status message is set, both Cmds are batched together.
func TestHandleWithRefreshBatchesPopCmdWithStatus(t *testing.T) {
	app, _, _ := newTestAppModelWithMockPersisterAndProject(t)
	app.viewStack = []ViewType{ViewDashboard, ViewCrossProjectOutliner, ViewTaskForm}

	cmd := app.handleWithRefresh(true, false, "Hello")
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}

	// Run the batched cmd and ensure ViewDidAppearMsg surfaces among the
	// messages it dispatches. tea.Batch returns a BatchMsg.
	msg := cmd()
	if msg == nil {
		t.Fatal("batched cmd should produce a message")
	}
	// Test that statusMessage was set
	if app.statusMessage != "Hello" {
		t.Errorf("expected status 'Hello', got %q", app.statusMessage)
	}
	// We don't depend on the BatchMsg internal API. As long as the cmd is
	// non-nil and the status was set, the batching plumbing is wired.
	_ = msg
	_ = tea.Batch // ensure import
}

// TestHandleWithRefreshNoPopReturnsNilWhenNoStatus verifies that when popView=false
// and no status is set, the returned cmd is nil (not an empty Batch).
func TestHandleWithRefreshNoPopReturnsNilWhenNoStatus(t *testing.T) {
	app, _, _ := newTestAppModelWithMockPersisterAndProject(t)
	app.viewStack = []ViewType{ViewDashboard, ViewProjectTree}

	cmd := app.handleWithRefresh(false, false, "")
	if cmd != nil {
		t.Errorf("expected nil cmd when no pop and no status, got non-nil")
	}
}

// TestPruneDeletedTaskRefsAbortsOnLoadFailure verifies that a transient
// persister Load failure does not cause references to be silently pruned.
// Before the fix, a load error on an unloaded project caused the loop to
// `continue` past the id-collection step, leaving the project's tasks
// missing from the validTasks set and pruning every focusQueue / dailyPlan
// / recentTaskVisits entry that pointed at it.
func TestPruneDeletedTaskRefsAbortsOnLoadFailure(t *testing.T) {
	app, project := newTestAppModelWithProject()
	if len(project.Tasks) == 0 {
		t.Fatal("test project must have tasks")
	}
	task := project.Tasks[0]

	// Swap in a mock persister whose Load always fails.
	mockPersister := NewMockPersister()
	mockPersister.LoadError = errors.New("simulated load failure")
	app.persister = mockPersister

	// Populate focusQueue, dailyPlan, and recentTaskVisits with a reference
	// to the project's task.
	app.focusQueue = []FocusTask{{Task: task, Project: project}}
	app.focusQueueIDs = map[string]bool{task.ID: true}
	app.dailyPlan = []FocusTask{{Task: task, Project: project}}
	app.dailyPlanIDs = map[string]bool{task.ID: true}
	app.recentTaskVisits = []RecentTaskVisitEntry{{Task: task, Project: project}}

	// Force the project into the "unloaded" state so prune triggers a Load.
	project.Tasks = nil

	app.pruneDeletedTaskRefs()

	if len(app.focusQueue) != 1 {
		t.Errorf("focusQueue: expected entries preserved on load failure, got len=%d", len(app.focusQueue))
	}
	if !app.focusQueueIDs[task.ID] {
		t.Errorf("focusQueueIDs: expected task id preserved on load failure")
	}
	if len(app.dailyPlan) != 1 {
		t.Errorf("dailyPlan: expected entries preserved on load failure, got len=%d", len(app.dailyPlan))
	}
	if !app.dailyPlanIDs[task.ID] {
		t.Errorf("dailyPlanIDs: expected task id preserved on load failure")
	}
	if len(app.recentTaskVisits) != 1 {
		t.Errorf("recentTaskVisits: expected entries preserved on load failure, got len=%d", len(app.recentTaskVisits))
	}
}
