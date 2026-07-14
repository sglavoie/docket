package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// TestCommandPaletteBulkActionSkipsInvalidTransitions verifies that the bulk
// "complete" action, now routed through the application-layer bulk-transition
// use case, is best-effort: a Done task selected alongside a Ready task is
// skipped (it cannot transition to Done again) while the Ready task moves to
// Done, and the toast reports only the one task actually changed. The whole
// batch commits as one workspace-ledger transaction, so ledger undo restores
// the changed task's prior state.
func TestCommandPaletteBulkActionSkipsInvalidTransitions(t *testing.T) {
	todo := seedTask(t, 1, "todo task", models.StateReady)
	done := seedTask(t, 2, "done task", models.StateDone)
	env, features := newSecondaryEnv(t, "cmdbulk", todo, done)

	m := &AppModel{
		runtime:           config.EmptyRuntimeState(""),
		secondaryFeatures: features,
		views:             NewViewRegistry(),
	}

	msg := CommandPaletteBulkActionMsg{
		Action: "complete",
		Tasks: []TaskProjectRef{
			{Task: todo, Project: env.project},
			{Task: done, Project: env.project},
		},
	}

	handled, cmd := m.handleCommandPaletteBulkAction(msg)
	if !handled {
		t.Fatal("expected handled=true")
	}

	result := runBatchResult(t, cmd)
	if result.Summary != "1 tasks completed" {
		t.Errorf("summary = %q, want %q", result.Summary, "1 tasks completed")
	}

	reloaded := env.reload(t)
	if got := reloaded.GetTaskByID(todo.ID); got == nil || got.State() != models.StateDone {
		t.Errorf("ready task should be Done, got %v", got)
	}
	if got := reloaded.GetTaskByID(done.ID); got == nil || got.State() != models.StateDone {
		t.Errorf("done task should remain Done, got %v", got)
	}

	// The batch committed one ledger transaction; undo restores the changed task.
	env.undo(t)
	reloaded = env.reload(t)
	if got := reloaded.GetTaskByID(todo.ID); got == nil || got.State() != models.StateReady {
		t.Errorf("undo should restore the ready task, got %v", got)
	}
}
