package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// observeCommittedDueEffect folds one committed transaction into the session's
// remembered deadline clocks and then reconciles them against what the reload
// that just ran actually put in front of the shell. Both halves run on every
// committed transaction, because the first is what the transaction says about
// itself and the second is what the workspace says about everyone else.
func (m *AppModel) observeCommittedDueEffect(transactionID string, effect coordinator.CommittedDueEffect) {
	m.supersedeDueStepsForEffect(effect)
	m.dueClocks.ObserveCommit(transactionID, effect)
	m.reconcileRememberedDueClocks()
}

// supersedeDueStepsForEffect abandons queued deadline work whose premise this
// transaction just replaced. It reads the same typed evidence the clock memory
// does, and only the two kinds that replace a premise: an explicit due edit is
// the user naming a deadline outright, which outranks a step queued against the
// one it displaced, and a removal is an address no queued gesture can still
// mean.
//
// Steps are deliberately absent from that test. Every gesture in a burst commits
// through here, so treating a step as a supersede would have the queue cancel
// its own successor on the first commit; a step is the queue's own evidence, not
// evidence against it.
//
// The effect's own targets are handed over rather than dropped, because an edit
// and the queue invalidate two different things. The pending tail goes whole —
// it was to be computed from a committed state this transaction has moved — but
// a gesture already committed is only overridden for the tasks this edit or
// removal actually named. A deadline typed on one task says nothing about the
// clock another task's step just cleared.
func (m *AppModel) supersedeDueStepsForEffect(effect coordinator.CommittedDueEffect) {
	if len(effect.Edits) == 0 && len(effect.Removed) == 0 {
		return
	}
	targets := make([]app.TaskLocator, 0, len(effect.Edits)+len(effect.Removed))
	targets = append(targets, effect.Edits...)
	targets = append(targets, effect.Removed...)
	m.dueSteps.SupersedeTargets(targets)
}

// dispatchDueStepGesture turns one frozen gesture into its transaction.
//
// The remembered clocks are read here rather than at the keypress, and read
// through the controller's one accessor, so a queued step sees the memory the
// preceding step's commit wrote instead of the memory that existed when the key
// was pressed. Nothing is written back: every clock write already happens on the
// committed result.
func (m *AppModel) dispatchDueStepGesture(gesture coordinator.DueStepGesture, sequence int) tea.Cmd {
	capture := m.captureOutlinerRelocationForLocators(gesture.Targets)
	request := app.TaskOperationRequest{
		Operation: app.TaskOperationStepDueDate,
		Targets:   gesture.Targets,
		DueStep: app.DueStepRequest{
			Direction:        gesture.Direction,
			RememberedClocks: m.dueClocks.RememberedClocks(gesture.Targets),
		},
	}
	return withDueStepSequence(withOutlinerRelocationCapture(
		m.secondaryFeatures.TaskItem.StepDueDates(request, gesture.Hidden),
		capture,
	), sequence)
}

// withDueStepSequence stamps a dispatched gesture's result with the sequence the
// queue armed it under. It wraps outside the relocation capture so the capture
// still sees the bare coordinator result it knows how to stamp.
func withDueStepSequence(cmd tea.Cmd, sequence int) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		return coordinator.DueStepResultMsg{Sequence: sequence, Inner: cmd()}
	}
}

// handleDueStepResult delivers one dispatched gesture and, on success, arms the
// next one behind it.
//
// A superseded delivery still commits and still reloads — the transaction
// happened, and pretending otherwise would leave the shell showing stale rows —
// and its typed effect is narrowed to what the supersession left standing, so a
// late result cannot write back clock memory that an explicit edit or a ledger
// restore has already replaced while a target neither of them named still keeps
// the clock its own commit earned. It also arms nothing: the queue behind it
// went with the premise.
//
// The supersession is read before the settle, because the settle is what stops
// the delivered sequence from being the in-flight one.
//
// The successor is armed after the reload rather than before it, so each step is
// computed from a workspace the shell has already taken delivery of. Arming
// after the reload is also what keeps the chain alive across the supersede check
// above: a step's own commit runs that check, finds no explicit edit, and leaves
// the queue standing.
func (m *AppModel) handleDueStepResult(msg coordinator.DueStepResultMsg) tea.Cmd {
	superseded := m.dueSteps.Superseded(msg.Sequence)
	switch inner := msg.Inner.(type) {
	case coordinator.BatchTransactionResultMsg:
		next, hasNext, current := m.dueSteps.Settle(msg.Sequence)
		if !current {
			inner.DueEffect = superseded.Retain(inner.DueEffect)
		}
		cmd := m.handleBatchTransactionResult(inner)
		if !hasNext {
			return cmd
		}
		return tea.Batch(cmd, m.dispatchDueStepGesture(next, m.dueSteps.Arm()))
	case coordinator.TransactionErrorMsg:
		dropped, _ := m.dueSteps.Abandon(msg.Sequence)
		m.feedback.err = coordinator.DueStepRefusal(inner.Err, dropped)
		return status.StatusTimeout(5 * time.Second)
	default:
		m.dueSteps.Abandon(msg.Sequence)
		return func() tea.Msg { return msg.Inner }
	}
}

// observeRestoredDueClocks takes clock memory back across one successful ledger
// undo or redo. Reconciliation runs first and the delta second, so a deadline
// the restore put back retires its remembered clock either way and the delta —
// which knows what this exact transaction did — has the last word.
// A restore also abandons queued deadline work outright: undo and redo move the
// very deadlines a queued step was to be computed from, so the frozen tail has
// lost its premise even when it names tasks the restore never touched.
func (m *AppModel) observeRestoredDueClocks(transactionID string, redo bool) {
	m.dueSteps.Supersede()
	m.reconcileRememberedDueClocks()
	m.dueClocks.ObserveRestore(transactionID, redo)
}

// reconcileRememberedDueClocks drops the memory of every remembered task the
// shell can currently read that now carries a deadline, whoever gave it one.
// That is how a deadline written on another machine, or by any path that reports
// no typed effect, retires the clock it superseded.
//
// Three callers reach it, and they are the three moments the shell's picture of
// the workspace changes: a committed transaction, a ledger restore, and
// adoptCrossProjectSnapshot taking delivery of an accepted external population.
// The first two run against the workspace the commit left behind; the third is
// the only one that can see a deadline no local effect ever named, and it is
// what keeps a later gesture from spending a clock an external write superseded.
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
