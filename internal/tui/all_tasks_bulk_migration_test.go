package tui

import (
	"errors"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// TestFinalizeBatchResultPreservesSelectionOnError verifies that a rejected batch
// passes its TransactionErrorMsg through untouched, so the shell never clears the
// multiselect for a batch that did not commit.
func TestFinalizeBatchResultPreservesSelectionOnError(t *testing.T) {
	errCmd := func() tea.Msg { return TransactionErrorMsg{Feature: "bulk", Action: "x", Err: errors.New("boom")} }
	out := finalizeBatchResult(errCmd, []string{"t1", "t2"}, "override")()
	errMsg, ok := out.(TransactionErrorMsg)
	if !ok {
		t.Fatalf("expected TransactionErrorMsg passthrough, got %T", out)
	}
	if errMsg.Err == nil {
		t.Fatal("error passthrough lost its error")
	}
}

// TestFinalizeBatchResultSetsSelectionAndSummary verifies that a committed batch
// gets the caller's selection IDs and per-view summary stamped onto it.
func TestFinalizeBatchResultSetsSelectionAndSummary(t *testing.T) {
	okCmd := func() tea.Msg { return BatchTransactionResultMsg{Summary: "coordinator wording"} }
	out := finalizeBatchResult(okCmd, []string{"t1", "t2"}, "view wording")()
	batch, ok := out.(BatchTransactionResultMsg)
	if !ok {
		t.Fatalf("expected BatchTransactionResultMsg, got %T", out)
	}
	if batch.Summary != "view wording" {
		t.Errorf("summary = %q, want the view override", batch.Summary)
	}
	if len(batch.ClearSelectionTaskIDs) != 2 {
		t.Errorf("clear selection ids = %v, want 2", batch.ClearSelectionTaskIDs)
	}

	// An empty override keeps the coordinator's own summary.
	kept := finalizeBatchResult(okCmd, nil, "")().(BatchTransactionResultMsg)
	if kept.Summary != "coordinator wording" {
		t.Errorf("summary = %q, want the coordinator wording kept", kept.Summary)
	}
}

// TestHandleBatchTransactionResultClearsSelectionAndRepairsRecentVisits verifies
// the shell's async result path: a committed batch clears the multiselect for the
// carried task IDs and repairs machine-local recent visits for the removed tasks.
func TestHandleBatchTransactionResultClearsSelectionAndRepairsRecentVisits(t *testing.T) {
	dir := t.TempDir()
	mgr := config.NewManager(filepath.Join(dir, ".tt.json"))
	cfg := newTestConfig()
	cfg.Machine().RecentTaskVisits = []config.RecentTaskVisit{
		{ProjectID: "p", TaskID: "gone"},
		{ProjectID: "p", TaskID: "keep"},
	}
	if err := saveTestRuntime(mgr, cfg); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	runtime := testRuntimeAt(cfg.DataDir(), cfg)
	m := &AppModel{configManager: mgr, runtime: runtime, views: NewViewRegistry()}
	atv := NewAllTasksViewModel(runtime, mgr)
	atv.multiSelectMode = true
	atv.selectedTasks["gone"] = true
	m.allTasksView = atv
	m.views.Register(ViewAllTasks, atv)

	m.handleBatchTransactionResult(BatchTransactionResultMsg{
		Summary:               "Deleted 1 task(s)",
		ClearSelectionTaskIDs: []string{"gone"},
		RemovedTaskIDs:        []string{"gone"},
	})

	if atv.multiSelectMode || atv.SelectedCount() != 0 {
		t.Error("committed batch should clear the multiselect")
	}
	visits := m.runtime.Machine().RecentTaskVisits
	if len(visits) != 1 || visits[0].TaskID != "keep" {
		t.Errorf("recent visits = %v, want only keep", visits)
	}
}

// TestAllTasksCanonicalDeleteFoldsDurableRefsAndRepairsRecentVisits exercises the
// full canonical All Tasks bulk delete: uppercase X emits the shell-owned
// TaskDeleteRequestedMsg for the active selection, the shell confirmation folds
// the durable planning refs into the delete transaction as a config artifact
// (cleaned before commit), the tasks leave disk, the shell repairs machine-local
// recent visits and clears the selection, and the durable cleanup persists on disk.
func TestAllTasksCanonicalDeleteFoldsDurableRefsAndRepairsRecentVisits(t *testing.T) {
	a := seedTask(t, 1, "Doomed A", models.StateReady)
	b := seedTask(t, 2, "Doomed B", models.StateReady)
	env, features := newSecondaryEnv(t, "atbulkdel", a, b)

	configPath := filepath.Join(env.dataDir, ".tt.json")
	mgr := config.NewManager(configPath)
	cfg := newTestConfigAt(env.dataDir)
	setTestProjects(cfg, []*models.Project{env.project})
	cfg.Planning().FocusQueue = []config.FocusQueueItem{{ProjectID: env.project.ID, TaskID: a.ID}}
	cfg.Machine().RecentTaskVisits = []config.RecentTaskVisit{{ProjectID: env.project.ID, TaskID: b.ID}}
	if err := saveTestRuntime(mgr, cfg); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	runtime := testRuntimeAt(cfg.DataDir(), cfg)
	m := &AppModel{configManager: mgr, runtime: runtime, secondaryFeatures: features, views: NewViewRegistry()}
	atv := NewAllTasksViewModel(runtime, mgr)
	atv.bulk = features.Bulk
	atv.multiSelectMode = true
	atv.selectedTasks[a.ID] = true
	atv.selectedTasks[b.ID] = true
	atv.groups = []AllTasksGroup{{Name: "All", Tasks: []CrossProjectTask{
		{Task: a, Project: env.project},
		{Task: b, Project: env.project},
	}}}
	m.allTasksView = atv
	m.views.Register(ViewAllTasks, atv)

	// Uppercase X emits the canonical request for the full selection and mutates
	// nothing; the shell arms its own confirmation, then commits on y.
	action := findSharedTaskKeymapAction(t, atv.buildKeyMap(), "X")
	_, reqCmd := action()
	reqMsg := deleteRequestFromCmd(t, reqCmd)
	if reqMsg.Origin != ViewAllTasks {
		t.Errorf("origin = %v, want ViewAllTasks", reqMsg.Origin)
	}
	m.handleTaskDeleteRequested(reqMsg)
	if got := m.taskDeleteConfirm.Prompt(); got != "Delete 2 tasks?" {
		t.Errorf("prompt = %q, want %q", got, "Delete 2 tasks?")
	}
	handled, cmd := m.handleTaskDeleteConfirmKey("y")
	if !handled || cmd == nil {
		t.Fatal("y should confirm and dispatch the delete transaction")
	}

	// Building the checked artifact must not mutate the cached runtime snapshot
	// before the task transaction actually commits.
	if len(runtime.Planning().GetFocusQueue()) != 1 {
		t.Errorf("cached focus queue changed before commit: %v", runtime.Planning().GetFocusQueue())
	}

	msg := runBatchResult(t, cmd)
	if len(msg.RemovedTaskIDs) != 2 {
		t.Fatalf("removed task ids = %v, want 2", msg.RemovedTaskIDs)
	}

	m.handleBatchTransactionResult(msg)

	if len(runtime.Planning().GetFocusQueue()) != 0 {
		t.Errorf("runtime focus queue should refresh after delete, got %v", runtime.Planning().GetFocusQueue())
	}
	if len(runtime.Machine().RecentTaskVisits) != 0 {
		t.Errorf("recent visits should be repaired after delete, got %v", runtime.Machine().RecentTaskVisits)
	}
	if atv.multiSelectMode || atv.SelectedCount() != 0 {
		t.Error("committed delete should clear the multiselect")
	}

	reloaded := env.reload(t)
	if reloaded.GetTaskByID(a.ID) != nil || reloaded.GetTaskByID(b.ID) != nil {
		t.Error("deleted tasks should be gone from disk")
	}

	// The durable focus-queue cleanup committed to disk with the delete.
	reloadedCfg, err := config.NewManager(configPath).LoadRuntimeState()
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if len(reloadedCfg.Planning().FocusQueue) != 0 {
		t.Errorf("durable focus queue should be cleaned on disk, got %v", reloadedCfg.Planning().FocusQueue)
	}
}
