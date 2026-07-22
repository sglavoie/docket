package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// This file proves Kanban's remaining single-card standard actions migrated in
// session 33.7.3.5.5.4.3 route through the coordinator-backed workflow
// transaction path (wf* -> TaskItemCoordinator -> app.Service) rather than the
// legacy synchronous taskOps* + workflowPersist path. Each test drives a card
// action whose pre-migration implementation called a taskOps* helper — the
// standard-keymap delete (applySelectedBulkDelete with no selection) and the H/L
// single-card column move (moveTaskToAdjacentCol) for section, status, and stage
// modes. A committed BatchTransactionResultMsg is only produced by the coordinator
// path; the retired taskOps* path resolved to a non-batch save message, so these
// assertions fail if the wf* wiring is reverted. Each change is also asserted
// durable on disk and reverted through the workspace undo ledger.

// kanbanCardModel builds a wired single-card Kanban model over the seeded
// projects: coordinator-backed, not in multi-select, so card actions target the
// cursor task through the workflow path.
func (e *kanbanBulkEnv) kanbanCardModel(t *testing.T, cols []KanbanColumn) *KanbanModel {
	t.Helper()
	m := newTestKanban(cols)
	m.taskItem = e.taskItem
	m.svc = NewProjectService()
	m.loadedProjects = e.projects
	return m
}

// kanbanWorkflowResult executes a card-action command and returns the committed
// BatchTransactionResultMsg, unwrapping the tea.Batch that moveTaskToAdjacentCol
// pairs the workflow command with (the card-flash tick). The workflow command is
// first in the batch, so it resolves before the flash tick is ever executed.
func kanbanWorkflowResult(t *testing.T, cmd tea.Cmd) BatchTransactionResultMsg {
	t.Helper()
	if cmd == nil {
		t.Fatal("card action returned a nil command")
	}
	switch msg := cmd().(type) {
	case BatchTransactionResultMsg:
		return msg
	case TransactionErrorMsg:
		t.Fatalf("expected a committed batch, got error: %v", msg.Err)
	case tea.BatchMsg:
		for _, c := range msg {
			if c == nil {
				continue
			}
			switch inner := c().(type) {
			case BatchTransactionResultMsg:
				return inner
			case TransactionErrorMsg:
				t.Fatalf("expected a committed batch, got error: %v", inner.Err)
			}
		}
		t.Fatalf("no BatchTransactionResultMsg in batch %v", msg)
	default:
		t.Fatalf("expected BatchTransactionResultMsg, got %T", msg)
	}
	return BatchTransactionResultMsg{}
}

// TestKanbanUppercaseXDeleteEmitsCanonicalRequest drives the standard uppercase-X
// delete on a single card. With no active selection it now emits the canonical,
// shell-owned TaskDeleteRequestedMsg for the cursor card and performs NO mutation
// before shell confirmation — the legacy path committed a subtree deletion here
// with no user-facing prompt (FR-005).
func TestKanbanUppercaseXDeleteEmitsCanonicalRequest(t *testing.T) {
	a := seedTask(t, 1, "Delete me", models.StateReady)
	b := seedTask(t, 2, "Keep me", models.StateReady)
	env := newKanbanBulkEnv(t, []*models.Task{a, b})
	m := env.kanbanCardModel(t, env.singleProjectCols(a, b))

	_, cmd := m.applySelectedBulkDelete()
	msg := deleteRequestFromCmd(t, cmd)
	if got := requestTaskIDs(msg); len(got) != 1 || got[0] != a.ID {
		t.Fatalf("delete targets = %v, want [%s]", got, a.ID)
	}
	if msg.Refs[0].Project.ID != env.projects[0].ID {
		t.Errorf("ref project = %q, want %q", msg.Refs[0].Project.ID, env.projects[0].ID)
	}
	if msg.Origin != ViewKanban {
		t.Errorf("origin = %v, want ViewKanban", msg.Origin)
	}

	// No mutation before shell confirmation: both cards remain on disk.
	reloaded := env.reload(t, 0)
	if got := reloaded.GetTaskByID(a.ID); got == nil {
		t.Fatalf("task %s must not be deleted before confirmation", a.ID)
	}
	if got := reloaded.GetTaskByID(b.ID); got == nil {
		t.Fatalf("sibling task %s must survive", b.ID)
	}
}

// TestKanbanColumnMoveSectionCommitsWorkflowAndUndoes drives an H/L single-card
// move in section mode. The target column's GTD section is now applied through
// wfSectionMove (state transition), committed asynchronously and undoable.
func TestKanbanColumnMoveSectionCommitsWorkflowAndUndoes(t *testing.T) {
	a := seedTask(t, 1, "Section move", models.StateReady)
	env := newKanbanBulkEnv(t, []*models.Task{a})
	m := env.kanbanCardModel(t, []KanbanColumn{
		{ID: "ready", Label: "Ready", Tasks: []KanbanCard{{Task: a, Project: env.projects[0]}}},
		{ID: "next", Label: "Next", Tasks: nil},
	})
	m.mode = KanbanModeSection

	_, cmd := m.moveTaskToAdjacentCol(1)
	msg := kanbanWorkflowResult(t, cmd)
	if len(msg.TouchedProjectIDs) != 1 || msg.TouchedProjectIDs[0] != env.projects[0].ID {
		t.Fatalf("touched projects = %v, want [%s]", msg.TouchedProjectIDs, env.projects[0].ID)
	}
	if got := env.reload(t, 0).GetTaskByID(a.ID); got == nil || got.State() != models.StateNext {
		t.Fatalf("reloaded state = %v, want next", got)
	}

	env.undo(t)
	if got := env.reload(t, 0).GetTaskByID(a.ID); got == nil || got.State() != models.StateReady {
		t.Fatalf("undo state = %v, want ready", got)
	}
}

// TestKanbanColumnMoveStatusCommitsWorkflowAndUndoes drives an H/L single-card
// move in status mode. The target column's status is applied through
// wfChangeStatus, committed asynchronously and undoable.
func TestKanbanColumnMoveStatusCommitsWorkflowAndUndoes(t *testing.T) {
	a := seedTask(t, 1, "Status move", models.StateReady)
	env := newKanbanBulkEnv(t, []*models.Task{a})
	m := env.kanbanCardModel(t, []KanbanColumn{
		{ID: string(models.StatusTodo), Label: "Todo", Tasks: []KanbanCard{{Task: a, Project: env.projects[0]}}},
		{ID: string(models.StatusInProgress), Label: "In Progress", Tasks: nil},
	})
	m.mode = KanbanModeStatus

	_, cmd := m.moveTaskToAdjacentCol(1)
	msg := kanbanWorkflowResult(t, cmd)
	if len(msg.TouchedProjectIDs) != 1 || msg.TouchedProjectIDs[0] != env.projects[0].ID {
		t.Fatalf("touched projects = %v, want [%s]", msg.TouchedProjectIDs, env.projects[0].ID)
	}
	if got := env.reload(t, 0).GetTaskByID(a.ID); got == nil || got.State() != models.StateInProgress {
		t.Fatalf("reloaded state = %v, want in_progress", got)
	}

	env.undo(t)
	if got := env.reload(t, 0).GetTaskByID(a.ID); got == nil || got.State() != models.StateReady {
		t.Fatalf("undo state = %v, want ready", got)
	}
}
