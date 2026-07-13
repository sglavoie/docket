package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// These tests pin the sidebar-unlink migration (session 33.7.3.4.5): every
// relation kind routes through the application-layer LinkCoordinator as one
// ledger-undoable ChangeSet (replacing the retired raw save), surfacing a
// LinkRemovedMsg that carries the touched project IDs the shell reloads.

func linkCoordFromEnv(env *kanbanBulkEnv) LinkCoordinator {
	return newSecondaryCoordinators(env.svc, nil).Link
}

func runLinkRemoved(t *testing.T, cmd tea.Cmd) LinkRemovedMsg {
	t.Helper()
	if cmd == nil {
		t.Fatal("sidebarUnlinkCommand returned nil")
	}
	msg := cmd()
	lr, ok := msg.(LinkRemovedMsg)
	if !ok {
		t.Fatalf("msg = %T, want LinkRemovedMsg", msg)
	}
	return lr
}

func TestSidebarUnlink_DependencyThroughCoordinatorAndLedger(t *testing.T) {
	a1 := seedTask(t, 1, "Blocked task", models.StateReady)
	a2 := seedTask(t, 2, "Blocker task", models.StateReady)
	a1.DependsOn = []string{a2.ID}
	env := newKanbanBulkEnv(t, []*models.Task{a1, a2})
	link := linkCoordFromEnv(env)

	// Section 0: remove a1's own dependency edge onto a2.
	lr := runLinkRemoved(t, sidebarUnlinkCommand(link, env.projects[0], a1, 0, 0))
	if lr.Section != 0 {
		t.Fatalf("section = %d, want 0", lr.Section)
	}
	if len(lr.TouchedProjectIDs) != 1 || lr.TouchedProjectIDs[0] != env.projects[0].ID {
		t.Fatalf("touched = %v, want [%s]", lr.TouchedProjectIDs, env.projects[0].ID)
	}
	if got := env.reload(t, 0).GetTaskByID(a1.ID); len(got.DependsOn) != 0 {
		t.Fatalf("DependsOn after unlink = %v, want empty", got.DependsOn)
	}

	// The removal committed through the workspace ledger, so a global undo
	// restores the dependency edge.
	env.undo(t)
	if got := env.reload(t, 0).GetTaskByID(a1.ID); len(got.DependsOn) != 1 || got.DependsOn[0] != a2.ID {
		t.Fatalf("DependsOn after ledger undo = %v, want [%s]", got.DependsOn, a2.ID)
	}
}

func TestSidebarUnlink_InverseBlockingRemovesDependentEdge(t *testing.T) {
	a1 := seedTask(t, 1, "Blocked task", models.StateReady)
	a2 := seedTask(t, 2, "Blocker task", models.StateReady)
	a1.DependsOn = []string{a2.ID}
	env := newKanbanBulkEnv(t, []*models.Task{a1, a2})
	link := linkCoordFromEnv(env)

	// Section 1 from a2's sidebar lists a1 (blocked by a2); removing it drops
	// a1's edge, not a2's.
	lr := runLinkRemoved(t, sidebarUnlinkCommand(link, env.projects[0], a2, 1, 0))
	if lr.Section != 1 {
		t.Fatalf("section = %d, want 1", lr.Section)
	}
	if got := env.reload(t, 0).GetTaskByID(a1.ID); len(got.DependsOn) != 0 {
		t.Fatalf("dependent edge not removed: %v", got.DependsOn)
	}
}

func TestSidebarUnlink_RelatedIsBidirectional(t *testing.T) {
	a1 := seedTask(t, 1, "Source", models.StateReady)
	a2 := seedTask(t, 2, "Peer", models.StateReady)
	a1.RelatedTo = []string{a2.ID}
	a2.RelatedTo = []string{a1.ID}
	env := newKanbanBulkEnv(t, []*models.Task{a1, a2})
	link := linkCoordFromEnv(env)

	lr := runLinkRemoved(t, sidebarUnlinkCommand(link, env.projects[0], a1, 2, 0))
	if lr.Section != 2 {
		t.Fatalf("section = %d, want 2", lr.Section)
	}
	if len(lr.TouchedProjectIDs) != 1 {
		t.Fatalf("touched = %v, want one project (same-project pair)", lr.TouchedProjectIDs)
	}
	reloaded := env.reload(t, 0)
	if got := reloaded.GetTaskByID(a1.ID); len(got.RelatedTo) != 0 {
		t.Fatalf("source related not cleared: %v", got.RelatedTo)
	}
	if got := reloaded.GetTaskByID(a2.ID); len(got.RelatedTo) != 0 {
		t.Fatalf("peer related not cleared (removal not bidirectional): %v", got.RelatedTo)
	}
}

func TestSidebarUnlink_OutOfRangeCursorIsNoOp(t *testing.T) {
	a1 := seedTask(t, 1, "Lonely", models.StateReady)
	env := newKanbanBulkEnv(t, []*models.Task{a1})
	link := linkCoordFromEnv(env)

	if cmd := sidebarUnlinkCommand(link, env.projects[0], a1, 0, 0); cmd != nil {
		t.Fatal("expected nil command for empty dependency section")
	}
}
