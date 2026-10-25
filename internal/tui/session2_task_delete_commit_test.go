package tui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/coordinator"
	"github.com/sglavoie/docket/internal/tui/listsurface"
)

// --- single-root subtree delete: full contract -----------------------------

func TestTaskDeleteSingleRootCommitsSubtreePlanningAndUndo(t *testing.T) {
	e := newTaskDeleteEnv(t)
	m := e.appModel()
	runtime := m.domain.runtime

	m.handleTaskDeleteRequested(TaskDeleteRequestedMsg{
		Refs: []TaskProjectRef{{Task: e.parent, Project: e.alpha}},
	})

	handled, cmd := m.handleGlobalKeyPress(keyPress("y"))
	if !handled || cmd == nil {
		t.Fatal("y should confirm and dispatch the delete transaction")
	}
	if m.taskMutation.deleteConfirm.IsActive() {
		t.Error("confirm should be dismissed once the transaction is dispatched")
	}

	// Building the checked artifact must not mutate the cached snapshot before commit.
	if len(runtime.Planning().GetNext()) != 1 {
		t.Errorf("cached focus queue changed before commit: %v", runtime.Planning().GetNext())
	}

	msg := runBatchResult(t, cmd)
	// The requested root is the selection to clear.
	if len(msg.ClearSelectionTaskIDs) != 1 || msg.ClearSelectionTaskIDs[0] != e.parent.ID {
		t.Errorf("clear-selection ids = %v, want [parent]", msg.ClearSelectionTaskIDs)
	}

	m.handleBatchTransactionResult(msg)

	// Subtree gone, sibling survives on disk.
	reloaded := e.reload(t, e.alpha)
	if reloaded.GetTaskByID(e.parent.ID) != nil || reloaded.GetTaskByID(e.child.ID) != nil {
		t.Error("parent and child should be gone from disk")
	}
	if reloaded.GetTaskByID(e.sibling.ID) == nil {
		t.Error("sibling should survive the subtree delete")
	}

	// Durable planning cleanup committed with the delete.
	if len(runtime.Planning().GetNext()) != 0 {
		t.Errorf("runtime focus queue should be cleaned, got %v", runtime.Planning().GetNext())
	}
	reloadedCfg, err := config.NewManager(filepath.Join(e.dataDir, ".tt.json")).LoadRuntimeState()
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if len(reloadedCfg.Planning().Next) != 0 {
		t.Errorf("durable focus queue should be cleaned on disk, got %v", reloadedCfg.Planning().Next)
	}

	// Undo restores the deleted subtree from the workspace ledger.
	if _, err := e.ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("ledger undo: %v", err)
	}
	restored := e.reload(t, e.alpha)
	if restored.GetTaskByID(e.parent.ID) == nil || restored.GetTaskByID(e.child.ID) == nil {
		t.Error("undo should restore the deleted subtree")
	}
}

// --- nested roots: duplicate subtree collapses, sibling survives -----------

func TestTaskDeleteNestedRootsPromptAndRemoval(t *testing.T) {
	e := newTaskDeleteEnv(t)
	m := e.appModel()

	// Select both the parent and its own child as roots.
	m.handleTaskDeleteRequested(TaskDeleteRequestedMsg{
		Refs: []TaskProjectRef{
			{Task: e.parent, Project: e.alpha},
			{Task: e.child, Project: e.alpha},
		},
	})
	if got := m.taskMutation.deleteConfirm.Prompt(); got != "Delete task #1?" {
		t.Errorf("nested-root prompt = %q, want canonical root prompt %q", got, "Delete task #1?")
	}

	_, cmd := m.handleGlobalKeyPress(keyPress("y"))
	msg := runBatchResult(t, cmd)
	m.handleBatchTransactionResult(msg)

	reloaded := e.reload(t, e.alpha)
	if reloaded.GetTaskByID(e.parent.ID) != nil || reloaded.GetTaskByID(e.child.ID) != nil {
		t.Error("nested roots should delete the whole subtree")
	}
	if reloaded.GetTaskByID(e.sibling.ID) == nil {
		t.Error("sibling should survive nested-root delete")
	}
}

func TestTaskDeleteConfirmationReportsFrozenCountsAndAttachmentConsequences(t *testing.T) {
	e := newTaskDeleteEnv(t)
	m := e.appModel()
	e.parent.Attachments = []models.Attachment{
		{Filename: "report.pdf"},
		{Filename: "https://example.test", IsURL: true},
	}
	e.child.Attachments = []models.Attachment{
		{Filename: "internal/app/destructive.go", IsCodeRef: true},
	}

	m.handleTaskDeleteRequested(TaskDeleteRequestedMsg{
		Refs: []TaskProjectRef{
			{Task: e.parent, Project: e.alpha},
			{Task: e.child, Project: e.alpha},
		},
		Selection: listsurface.SelectionCounts{Total: 2, Visible: 1, Hidden: 1},
	})
	detail := m.taskMutation.deleteConfirm.Detail
	if detail == nil {
		t.Fatal("delete confirmation omitted rich impact detail")
	}
	for _, want := range []string{
		"1 subtask also deleted",
		"2 tasks total",
		"2 selected (1 visible · 1 hidden)",
		"1 stored file deleted",
		"2 links removed",
	} {
		if !strings.Contains(detail.CascadeNote, want) {
			t.Errorf("delete impact %q omits %q", detail.CascadeNote, want)
		}
	}
	if !detail.UndoHint {
		t.Error("delete confirmation omitted undo availability")
	}
	if rendered := ansi.Strip(m.taskMutation.deleteConfirm.RenderOverlay(180)); !strings.Contains(rendered, "Undo available (u)") {
		t.Errorf("rendered delete confirmation omitted undo hint:\n%s", rendered)
	}
}

// TestTaskDeleteConfirmationOfOnePlainTaskWarnsAboutNothing pins the other half
// of the impact note: a lone task with no subtasks, no hidden selection, and no
// attachments has no consequence beyond the title the prompt already lists, so
// the dialog draws no warning line of zero counts.
func TestTaskDeleteConfirmationOfOnePlainTaskWarnsAboutNothing(t *testing.T) {
	e := newTaskDeleteEnv(t)
	m := e.appModel()

	m.handleTaskDeleteRequested(TaskDeleteRequestedMsg{
		Refs:      []TaskProjectRef{{Task: e.sibling, Project: e.alpha}},
		Selection: listsurface.SelectionCounts{Total: 1, Visible: 1},
	})
	detail := m.taskMutation.deleteConfirm.Detail
	if detail == nil {
		t.Fatal("delete confirmation omitted its detail")
	}
	if detail.CascadeNote != "" {
		t.Errorf("plain delete warns %q, want no cascade note", detail.CascadeNote)
	}
	if len(detail.Items) != 1 || !detail.UndoHint {
		t.Errorf("plain delete detail = %+v, want the one title and the undo hint", detail)
	}
}

// --- cross-project selection commits atomically ----------------------------

func TestTaskDeleteCrossProjectSelection(t *testing.T) {
	e := newTaskDeleteEnv(t)
	m := e.appModel()

	m.handleTaskDeleteRequested(TaskDeleteRequestedMsg{
		Refs: []TaskProjectRef{
			{Task: e.sibling, Project: e.alpha},
			{Task: e.lone, Project: e.beta},
		},
	})
	if got := m.taskMutation.deleteConfirm.Prompt(); got != "Delete 2 tasks?" {
		t.Errorf("prompt = %q, want %q", got, "Delete 2 tasks?")
	}

	_, cmd := m.handleGlobalKeyPress(keyPress("y"))
	msg := runBatchResult(t, cmd)
	if len(msg.TouchedProjectIDs) != 2 {
		t.Errorf("touched projects = %v, want alpha+beta", msg.TouchedProjectIDs)
	}
	m.handleBatchTransactionResult(msg)

	if e.reload(t, e.alpha).GetTaskByID(e.sibling.ID) != nil {
		t.Error("alpha sibling should be gone")
	}
	if e.reload(t, e.beta).GetTaskByID(e.lone.ID) != nil {
		t.Error("beta lone should be gone")
	}
	// The parent subtree in alpha is untouched by this selection.
	if e.reload(t, e.alpha).GetTaskByID(e.parent.ID) == nil {
		t.Error("unselected parent should survive")
	}
}

func TestTUITaskDeletePreservesDuplicateIDReferencesInOtherProject(t *testing.T) {
	e := newTaskDeleteEnv(t)
	e.lone.ID = e.parent.ID
	if err := storage.SaveProject(e.beta); err != nil {
		t.Fatalf("save beta duplicate: %v", err)
	}
	e.cfg.Planning().SetNext([]config.ProjectTaskLocator{
		{ProjectID: e.alpha.ID, TaskID: e.parent.ID},
		{ProjectID: e.beta.ID, TaskID: e.lone.ID},
	})
	e.cfg.Planning().Today = []config.ProjectTaskLocator{
		{ProjectID: e.alpha.ID, TaskID: e.parent.ID},
		{ProjectID: e.beta.ID, TaskID: e.lone.ID},
	}
	if err := saveTestRuntime(e.mgr, e.cfg); err != nil {
		t.Fatalf("save duplicate refs: %v", err)
	}

	m := e.appModel()
	m.handleTaskDeleteRequested(TaskDeleteRequestedMsg{
		Refs: []TaskProjectRef{{Task: e.lone, Project: e.beta}},
	})
	_, cmd := m.handleGlobalKeyPress(keyPress("y"))
	msg := runBatchResult(t, cmd)
	m.handleBatchTransactionResult(msg)

	if e.reload(t, e.alpha).GetTaskByID(e.parent.ID) == nil {
		t.Fatal("alpha/shared was deleted")
	}
	if e.reload(t, e.beta).GetTaskByID(e.lone.ID) != nil {
		t.Fatal("beta/shared survived delete")
	}
	if got := m.domain.runtime.Planning().Next; len(got) != 1 || got[0].ProjectID != e.alpha.ID {
		t.Fatalf("delete cleaned wrong focus refs: %+v", got)
	}
	if got := m.domain.runtime.Planning().Today; len(got) != 1 || got[0].ProjectID != e.alpha.ID {
		t.Fatalf("delete cleaned wrong daily refs: %+v", got)
	}
}

// --- failed delete preserves state and emits no success --------------------

func TestTaskDeleteFailurePreservesStateNoSuccess(t *testing.T) {
	e := newTaskDeleteEnv(t)
	m := e.appModel()

	// A root whose task is not on disk: the application service rejects the batch
	// with ErrTaskNotFound, so the transaction never commits.
	ghost := seedTask(t, 99, "Ghost", models.StateBacklog)
	m.handleTaskDeleteRequested(TaskDeleteRequestedMsg{
		Refs: []TaskProjectRef{{Task: ghost, Project: e.alpha}},
	})

	_, cmd := m.handleGlobalKeyPress(keyPress("y"))
	if cmd == nil {
		t.Fatal("confirm should dispatch a command even for a doomed delete")
	}
	out := cmd()
	if _, ok := out.(coordinator.TransactionErrorMsg); !ok {
		t.Fatalf("failed delete should yield TransactionErrorMsg, got %T", out)
	}
	// A rejected batch never enriches ClearSelectionTaskIDs, so the shell keeps the
	// selection intact for a retry — and the real tasks are untouched on disk.
	if reloaded := e.reload(t, e.alpha); reloaded.GetTaskByID(e.parent.ID) == nil || reloaded.GetTaskByID(e.sibling.ID) == nil {
		t.Error("a failed delete must leave existing tasks on disk")
	}

	// Routed through Update, the error sets the error banner, not a success status.
	m.feedback.err = nil
	if handled, _ := m.handleGlobalSaveMessage(out); !handled {
		t.Fatal("TransactionErrorMsg should be handled by the save-message path")
	}
	if m.feedback.err == nil {
		t.Error("failure should surface an error, not a success state")
	}
}
