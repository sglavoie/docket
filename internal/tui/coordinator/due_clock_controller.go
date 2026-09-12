package coordinator

import (
	"time"

	"github.com/sglavoie/tt/internal/app"
)

// dueClockHistoryLimit matches the workspace undo ledger's own depth. Memory
// deltas are only ever consumed by a ledger restore, so remembering more of them
// than the ledger can restore would keep deltas alive for transactions the user
// can no longer reach.
const dueClockHistoryLimit = 50

// CommittedDueEffect is the typed evidence one committed transaction carries
// about deadlines and task addresses. It exists so the clock memory below never
// has to read a status line or re-derive intent from a reloaded workspace: every
// mutation path that can touch a deadline reports what it did in these four
// fields, and a path that touched none reports an empty value.
//
// An empty effect is still worth delivering, because observing a committed
// transaction is also what abandons the ledger's redo branch.
type CommittedDueEffect struct {
	// Steps are the deadline-stepping outcomes of this transaction, each naming
	// the target's post-commit address and the deadline on either side of it.
	Steps []app.DueStepOutcome
	// Edits name every task this transaction gave an explicit deadline
	// instruction, whatever deadline resulted. A title-only or comment-only edit
	// names nothing here.
	Edits []app.TaskLocator
	// Moves is the committed source-to-destination address map, every relocated
	// descendant included, so memory keyed by the old address travels with the
	// task instead of being stranded or landing on a stranger.
	Moves []app.TaskAddressChange
	// Removed names every task this transaction deleted.
	Removed []app.TaskLocator
}

// ObservedDeadline is one task the shell actually loaded and looked at, together
// with whether it currently carries a deadline. Only observed tasks are
// reconciled: a task that is filtered out, parked, archived, or simply not
// loaded says nothing about its own deadline, and forgetting its clock because
// the view stopped showing it would lose memory the user never overrode.
type ObservedDeadline struct {
	Target      app.TaskLocator
	HasDeadline bool
}

// DueClockController is the session's memory of wall clocks that deadline
// stepping cleared, so stepping back out of unset restores the time the task
// had rather than inventing one.
//
// Memory is keyed by project-qualified task address, never by title and never by
// task UUID alone: the same UUID can name two different tasks in two different
// projects, and only the qualified address distinguishes them. It is entirely
// session-local — nothing here is persisted, so a restart starts empty, which is
// exactly the contract.
//
// The zero value is ready to use and every method tolerates a nil receiver.
type DueClockController struct {
	clocks map[app.TaskLocator]app.DueStepClock
	// undo and redo mirror the workspace ledger's two stacks. Each entry is the
	// before/after memory delta of one durable transaction, so a ledger restore
	// takes memory back with it instead of leaving a clock remembered for a
	// deadline that has been put back on the task.
	undo []dueClockHistoryEntry
	redo []dueClockHistoryEntry
}

type dueClockHistoryEntry struct {
	transactionID string
	changes       []dueClockChange
}

// dueClockChange is one key's memory on either side of a transaction. A nil
// clock is absence, which is a real value here: forgetting a clock has to be
// reversible the same way remembering one is.
type dueClockChange struct {
	target app.TaskLocator
	before *app.DueStepClock
	after  *app.DueStepClock
}

// RememberedClocks returns the clocks remembered for the given targets, as the
// detached map a step request carries. Nil targets ask for every remembered
// clock, which is what a reconcile pass needs: it has to visit the tasks memory
// still speaks about, not the tasks the view happens to show.
func (c *DueClockController) RememberedClocks(targets []app.TaskLocator) map[app.TaskLocator]app.DueStepClock {
	if c == nil || len(c.clocks) == 0 {
		return nil
	}
	if targets == nil {
		remembered := make(map[app.TaskLocator]app.DueStepClock, len(c.clocks))
		for target, clock := range c.clocks {
			remembered[target] = clock
		}
		return remembered
	}
	var remembered map[app.TaskLocator]app.DueStepClock
	for _, target := range targets {
		clock, ok := c.clocks[target]
		if !ok {
			continue
		}
		if remembered == nil {
			remembered = make(map[app.TaskLocator]app.DueStepClock, len(targets))
		}
		remembered[target] = clock
	}
	return remembered
}

// ObserveCommit folds one committed transaction into memory and records the
// delta it produced under that transaction's ID.
//
// It is called for every committed transaction, not only the ones that touched a
// deadline, because a forward commit is also what abandons the ledger's redo
// branch: the deltas that branch owned are pruned with it rather than left to be
// replayed onto a history that no longer leads there.
//
// A transaction with no ID committed nothing durable, so there is no ledger
// entry to undo it with and no redo branch to abandon. Its memory effect still
// applies — an explicit clear the workspace had nothing to write for is still
// the user overriding what was remembered — it simply cannot be reversed.
func (c *DueClockController) ObserveCommit(transactionID string, effect CommittedDueEffect) {
	if c == nil {
		return
	}
	writer := dueClockWriter{controller: c}
	// Addresses are re-keyed first, because every other field of the effect names
	// the post-commit address of its target.
	for _, move := range effect.Moves {
		writer.move(move.From, move.To)
	}
	for _, outcome := range effect.Steps {
		writer.step(outcome)
	}
	// An explicit deadline instruction overrides what was remembered, whatever
	// deadline it produced: the user named the clock they want.
	for _, target := range effect.Edits {
		writer.forget(target)
	}
	// A deleted task's memory is dropped rather than left addressable, so nothing
	// can later attach it to a task that reuses the address.
	for _, target := range effect.Removed {
		writer.forget(target)
	}
	c.record(transactionID, writer.changes)
}

// ObserveRestore takes memory back across one successful ledger undo or redo.
// The transaction ID is the restored ledger entry's, read only after the restore
// actually succeeded: a refused or failed restore changed no durable state and
// must leave memory exactly as it was.
//
// Our stacks are a subsequence of the ledger's in the same order — a transaction
// with no memory delta records no entry — so the restored transaction can only
// ever be the top of the matching stack. Anything else is a transaction memory
// knows nothing about, which is what a restart looks like: durable state is
// restored and no clock is invented for it.
func (c *DueClockController) ObserveRestore(transactionID string, redo bool) {
	if c == nil || transactionID == "" {
		return
	}
	source, destination := &c.undo, &c.redo
	if redo {
		source, destination = &c.redo, &c.undo
	}
	entries := *source
	if len(entries) == 0 || entries[len(entries)-1].transactionID != transactionID {
		return
	}
	restored := entries[len(entries)-1]
	*source = entries[:len(entries)-1]
	for _, change := range restored.changes {
		value := change.before
		if redo {
			value = change.after
		}
		c.assign(change.target, value)
	}
	*destination = append(*destination, restored)
}

// ReconcileObservedDeadlines drops the memory of every observed task that now
// carries a deadline, whoever gave it one. A remembered clock is only ever spent
// stepping out of unset, so a task with a deadline has nothing left to restore:
// this is how a deadline set on another machine, or by any path that reports no
// typed effect, retires the clock it superseded.
//
// It records no history entry. An observation is not a transaction, and no
// ledger restore can take it back.
func (c *DueClockController) ReconcileObservedDeadlines(observed []ObservedDeadline) {
	if c == nil || len(c.clocks) == 0 {
		return
	}
	for _, deadline := range observed {
		if !deadline.HasDeadline {
			continue
		}
		delete(c.clocks, deadline.Target)
	}
}

// record commits one transaction's memory delta to the history stacks.
func (c *DueClockController) record(transactionID string, changes []dueClockChange) {
	if transactionID == "" {
		return
	}
	c.redo = nil
	changes = compactDueClockChanges(changes)
	if len(changes) == 0 {
		return
	}
	c.undo = append(c.undo, dueClockHistoryEntry{transactionID: transactionID, changes: changes})
	if len(c.undo) > dueClockHistoryLimit {
		c.undo = c.undo[len(c.undo)-dueClockHistoryLimit:]
	}
}

// compactDueClockChanges drops the keys a transaction ended up leaving exactly
// as it found them. One transaction can legitimately touch a key twice — a
// cross-project edit that also names a deadline re-keys memory and then
// overrides it — and a key whose before and after agree carries no information a
// restore could use.
func compactDueClockChanges(changes []dueClockChange) []dueClockChange {
	compacted := changes[:0]
	for _, change := range changes {
		if sameDueClock(change.before, change.after) {
			continue
		}
		compacted = append(compacted, change)
	}
	return compacted
}

func sameDueClock(a, b *app.DueStepClock) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// assign writes one key without recording anything, which is what a restore
// does: the delta being replayed is already the record.
func (c *DueClockController) assign(target app.TaskLocator, clock *app.DueStepClock) {
	if clock == nil {
		delete(c.clocks, target)
		return
	}
	if c.clocks == nil {
		c.clocks = make(map[app.TaskLocator]app.DueStepClock)
	}
	c.clocks[target] = *clock
}

func (c *DueClockController) lookup(target app.TaskLocator) *app.DueStepClock {
	clock, ok := c.clocks[target]
	if !ok {
		return nil
	}
	return &clock
}

// dueClockWriter applies one transaction's memory effects while accumulating the
// reversible delta they add up to. Repeated writes to one key coalesce into a
// single change, so the recorded before is the value the transaction started
// from and the recorded after is the value it ended on.
type dueClockWriter struct {
	controller *DueClockController
	changes    []dueClockChange
	indexes    map[app.TaskLocator]int
}

func (w *dueClockWriter) write(target app.TaskLocator, clock *app.DueStepClock) {
	if index, ok := w.indexes[target]; ok {
		// The recorded before already holds what this transaction started from,
		// so a repeat write moves the after alone.
		w.changes[index].after = clock
	} else {
		if w.indexes == nil {
			w.indexes = make(map[app.TaskLocator]int)
		}
		w.indexes[target] = len(w.changes)
		w.changes = append(w.changes, dueClockChange{
			target: target,
			before: w.controller.lookup(target),
			after:  clock,
		})
	}
	w.controller.assign(target, clock)
}

func (w *dueClockWriter) forget(target app.TaskLocator) {
	if w.controller.lookup(target) == nil {
		if _, pending := w.indexes[target]; !pending {
			return
		}
	}
	w.write(target, nil)
}

// move carries one task's memory from its old address to its new one. A source
// with nothing remembered writes nothing at all: the destination address names
// the same task the source did, so it cannot already hold a different task's
// clock, and a delta per relocated descendant would be noise.
func (w *dueClockWriter) move(from, to app.TaskLocator) {
	if from == to {
		return
	}
	clock := w.controller.lookup(from)
	if clock == nil {
		return
	}
	w.write(from, nil)
	w.write(to, clock)
}

// step folds one deadline-stepping outcome into memory: a step that landed on a
// deadline has put the clock back on the task and needs nothing remembered,
// while a step that cleared one remembers the wall clock it cleared.
func (w *dueClockWriter) step(outcome app.DueStepOutcome) {
	if outcome.After != nil {
		w.forget(outcome.Target)
		return
	}
	clock, ok := dueStepClockOf(outcome.Before)
	if !ok {
		w.forget(outcome.Target)
		return
	}
	w.write(outcome.Target, &clock)
}

// dueStepClockOf reads the wall clock a cleared deadline carried. Local midnight
// is the durable date-only signal, so a deadline that fell on it remembers
// nothing: restoring it would invent a clock the task never had, and stepping
// out of unset with nothing remembered already produces exactly that date-only
// deadline.
func dueStepClockOf(due *time.Time) (app.DueStepClock, bool) {
	if due == nil {
		return app.DueStepClock{}, false
	}
	hour, minute := due.Hour(), due.Minute()
	if hour == 0 && minute == 0 {
		return app.DueStepClock{}, false
	}
	return app.DueStepClock{Hour: hour, Minute: minute}, true
}
