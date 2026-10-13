package coordinator

import "github.com/sglavoie/tt/internal/app"

// undoFocusHistoryLimit matches the workspace undo ledger's own depth, for the
// same reason dueClockHistoryLimit does: a correlation is only ever spent by a
// ledger restore, so remembering more of them than the ledger can restore would
// keep targets alive for transactions the user can no longer reach.
const undoFocusHistoryLimit = 50

// CommittedFocusTarget is the one task a gesture was aimed at, named at the
// address it held before the commit and at the address the commit left it at.
// The two differ only for a move; every other mutation reports the same address
// twice.
//
// It is deliberately one target and not a set. A transaction's typed evidence
// already enumerates what the commit touched — every relocated descendant in
// CommittedDueEffect.Moves, every removed task in CommittedDueEffect.Removed —
// and none of that says what the user pointed at. Only the seam that built the
// request knows that, so only that seam names it, and a gesture that named
// several tasks names none here rather than electing one of them.
type CommittedFocusTarget struct {
	Before app.TaskLocator
	After  app.TaskLocator
}

// Valid reports whether either side of the pair addresses a task. A creation
// has no before and a deletion's after is the address the row vacated, so a
// pair is useful as soon as one side resolves.
func (t CommittedFocusTarget) Valid() bool { return t.Before.Valid() || t.After.Valid() }

// FocusTargetFromSource builds the pair from the pre-commit address a gesture
// froze, following the committed move map to the address the task ended at. A
// source no move names stayed where it was.
func FocusTargetFromSource(source app.TaskLocator, moves []app.TaskAddressChange) CommittedFocusTarget {
	target := CommittedFocusTarget{Before: source, After: source}
	if !source.Valid() {
		return CommittedFocusTarget{}
	}
	for _, move := range moves {
		if move.From == source {
			target.After = move.To
		}
	}
	return target
}

// FocusTargetFromResult is the same pair read from the other end: a single-task
// result reports the address the commit left the task at, so the move map is
// walked backwards to recover where it started.
func FocusTargetFromResult(result app.TaskLocator, moves []app.TaskAddressChange) CommittedFocusTarget {
	target := CommittedFocusTarget{Before: result, After: result}
	if !result.Valid() {
		return CommittedFocusTarget{}
	}
	for _, move := range moves {
		if move.To == result {
			target.Before = move.From
		}
	}
	return target
}

// UndoFocusController is the session's memory of which single task each
// committed transaction was aimed at, so a later ledger restore can put the
// cursor back on the work it actually restored instead of guessing from a
// reloaded workspace.
//
// It is entirely session-local — nothing here is persisted, so a restart starts
// empty and every restored entry is simply uncorrelated, which is the contract:
// durable state is restored and no cursor is moved for it.
//
// The zero value is ready to use and every method tolerates a nil receiver.
type UndoFocusController struct {
	// undo and redo mirror the workspace ledger's two stacks, the same way
	// DueClockController's do. They are a subsequence of the ledger's in the same
	// order — a transaction the shell could not name one target for records no
	// entry — which is what makes the top-of-stack match below sound.
	undo []undoFocusEntry
	redo []undoFocusEntry
}

type undoFocusEntry struct {
	transactionID string
	target        CommittedFocusTarget
}

// ObserveCommit records the one task a committed transaction was aimed at.
//
// It is called for every committed transaction, not only the ones that can name
// a target, because a forward commit is also what abandons the ledger's redo
// branch: the correlations that branch owned are pruned with it rather than left
// to be replayed onto a history that no longer leads there. A bulk gesture, a
// project-scoped commit, and a multiline capture therefore all deliver an
// invalid target, which prunes without recording.
//
// A transaction with no ID committed nothing durable, so there is no ledger
// entry to undo it with and no redo branch to abandon.
func (c *UndoFocusController) ObserveCommit(transactionID string, target CommittedFocusTarget) {
	if c == nil || transactionID == "" {
		return
	}
	c.redo = nil
	if !target.Valid() {
		return
	}
	c.undo = append(c.undo, undoFocusEntry{transactionID: transactionID, target: target})
	if len(c.undo) > undoFocusHistoryLimit {
		c.undo = c.undo[len(c.undo)-undoFocusHistoryLimit:]
	}
}

// ObserveRestore reports the address one successful ledger undo or redo put the
// gesture's own task back at, and whether this transaction was correlated at
// all. The transaction ID is the restored ledger entry's, read only after the
// restore actually succeeded: a refused or failed restore changed no durable
// state and must leave this memory exactly as it found it.
//
// Because the stacks are a subsequence of the ledger's, the restored
// transaction can only ever be the top of the matching stack. Anything else is a
// transaction this memory knows nothing about — a bulk change, a project
// mutation, work from before a restart — and is reported as uncorrelated so the
// caller leaves the view alone.
func (c *UndoFocusController) ObserveRestore(transactionID string, redo bool) (app.TaskLocator, bool) {
	if c == nil || transactionID == "" {
		return app.TaskLocator{}, false
	}
	source, destination := &c.undo, &c.redo
	if redo {
		source, destination = &c.redo, &c.undo
	}
	entries := *source
	if len(entries) == 0 || entries[len(entries)-1].transactionID != transactionID {
		return app.TaskLocator{}, false
	}
	restored := entries[len(entries)-1]
	*source = entries[:len(entries)-1]
	*destination = append(*destination, restored)
	// Undo puts the task back where it was before the commit; redo puts it back
	// where the commit had left it.
	address := restored.target.Before
	if redo {
		address = restored.target.After
	}
	if !address.Valid() {
		return app.TaskLocator{}, false
	}
	return address, true
}

// ForgetRetired drops the memory of one entry the ledger retired without
// restoring it. The ledger no longer holds that entry, so leaving it on top
// would break the subsequence the top-of-stack match relies on and leave every
// older entry uncorrelated. Nothing is applied: a retired entry changed no
// durable state.
func (c *UndoFocusController) ForgetRetired(transactionID string, redo bool) {
	if c == nil || transactionID == "" {
		return
	}
	source := &c.undo
	if redo {
		source = &c.redo
	}
	entries := *source
	if len(entries) == 0 || entries[len(entries)-1].transactionID != transactionID {
		return
	}
	*source = entries[:len(entries)-1]
}
