package tui

import (
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// observeCommittedDueEffect folds one committed transaction into the session's
// remembered deadline clocks and then reconciles them against what the reload
// that just ran actually put in front of the shell. Both halves run on every
// committed transaction, because the first is what the transaction says about
// itself and the second is what the workspace says about everyone else.
func (m *AppModel) observeCommittedDueEffect(transactionID string, effect coordinator.CommittedDueEffect) {
	m.dueClocks.ObserveCommit(transactionID, effect)
	m.reconcileRememberedDueClocks()
}

// observeRestoredDueClocks takes clock memory back across one successful ledger
// undo or redo. Reconciliation runs first and the delta second, so a deadline
// the restore put back retires its remembered clock either way and the delta —
// which knows what this exact transaction did — has the last word.
func (m *AppModel) observeRestoredDueClocks(transactionID string, redo bool) {
	m.reconcileRememberedDueClocks()
	m.dueClocks.ObserveRestore(transactionID, redo)
}

// reconcileRememberedDueClocks drops the memory of every remembered task the
// shell can currently read that now carries a deadline, whoever gave it one.
// That is how a deadline written on another machine, or by any path that reports
// no typed effect, retires the clock it superseded.
//
// It asks about the tasks memory speaks about rather than the tasks the view
// shows, and a target the shell cannot resolve — filtered out is irrelevant
// here, but parked, archived, or simply not loaded are not — says nothing about
// its own deadline and keeps what was remembered for it.
func (m *AppModel) reconcileRememberedDueClocks() {
	remembered := m.dueClocks.RememberedClocks(nil)
	if len(remembered) == 0 {
		return
	}
	observed := make([]coordinator.ObservedDeadline, 0, len(remembered))
	for target := range remembered {
		task, _ := m.taskAndProjectForIdentity(taskref.FromLocator(target))
		if task == nil {
			continue
		}
		observed = append(observed, coordinator.ObservedDeadline{
			Target:      target,
			HasDeadline: task.DueDate != nil,
		})
	}
	m.dueClocks.ReconcileObservedDeadlines(observed)
}

// taskLineEditDueEffect projects one committed inline or external-editor task
// edit into the typed evidence the clock memory reads. DueEdited is the edit's
// own intent rather than the deadline it produced, which is what keeps a
// title-only or comment-only edit silent here and an explicit clear audible even
// when the deadline was already absent.
func taskLineEditDueEffect(result app.TaskLineEditResult) coordinator.CommittedDueEffect {
	effect := coordinator.CommittedDueEffect{Moves: result.Moves}
	if result.DueEdited {
		effect.Edits = []app.TaskLocator{result.Task}
	}
	return effect
}

// outlinePlanDueEffect projects one applied outline buffer into the same typed
// evidence. The buffer never crosses a project boundary, so it reports no moves;
// its deletions are addresses nothing may attach a remembered clock to again.
func outlinePlanDueEffect(result app.OutlinePlanResult) coordinator.CommittedDueEffect {
	effect := coordinator.CommittedDueEffect{Edits: result.DueEdits}
	if len(result.Deleted) == 0 {
		return effect
	}
	effect.Removed = make([]app.TaskLocator, 0, len(result.Deleted))
	for _, deleted := range result.Deleted {
		effect.Removed = append(effect.Removed, app.TaskLocator{
			ProjectID: deleted.ProjectID,
			TaskID:    deleted.TaskID,
		})
	}
	return effect
}
