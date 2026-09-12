package coordinator

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
)

// DueStepGesture is one keypress worth of deadline stepping, frozen at the
// moment the key was pressed. Targets are the project-qualified addresses the
// cursor or the visual selection named then, so navigation, a rebuild, or a
// filtered relocation afterwards cannot retarget work already queued. Hidden is
// the part of that selection the projection was not showing, which only the
// surface that froze the selection can know.
//
// The gesture deliberately carries no remembered clocks and no deadline: both
// are read from the state the preceding gesture committed, at dispatch, so a
// queued step is computed from what actually landed rather than from the row the
// user was looking at when they pressed the key.
type DueStepGesture struct {
	Direction app.DueStepDirection
	Targets   []app.TaskLocator
	Hidden    int
}

// DueStepResultMsg is one dispatched gesture's result, stamped with the sequence
// number the queue armed it under. The envelope exists because a deadline result
// is the one transaction result whose meaning depends on whether it is still
// current: a superseded gesture still committed, so its reload must happen, but
// the clock memory it would write describes a state some later act has already
// replaced.
type DueStepResultMsg struct {
	Sequence int
	Inner    tea.Msg
}

// DueStepQueue serializes deadline gestures into one transaction at a time.
//
// It is narrow on purpose: exactly one gesture is ever in flight, the rest wait
// in arrival order, and the next one is armed only once the previous one's
// commit has been reloaded and relocated. That is what makes each step compute
// from the preceding committed state instead of from a stale row, and it is also
// why gestures are never coalesced — one keypress is one transaction and one
// undo entry, so a burst of them is undone the way it was typed.
//
// The zero value is ready to use and every method tolerates a nil receiver.
type DueStepQueue struct {
	// sequence stamps the in-flight gesture. It is monotonic rather than a
	// boolean pair because a superseded gesture's result still arrives: the stamp
	// is what tells a late delivery from the current one.
	sequence int
	counter  int
	inflight bool
	pending  []DueStepGesture
}

// Enqueue records one keypress. It returns the sequence to stamp the dispatch
// with, or dispatch=false when a gesture is already in flight and this one has
// joined the tail instead.
func (q *DueStepQueue) Enqueue(gesture DueStepGesture) (int, bool) {
	if q == nil {
		return 0, false
	}
	if q.inflight {
		q.pending = append(q.pending, gesture)
		return 0, false
	}
	return q.Arm(), true
}

// Arm marks one gesture as the in-flight one and returns its sequence. It is
// called both for the keypress that found the queue idle and for each successor
// the settle below hands back.
func (q *DueStepQueue) Arm() int {
	if q == nil {
		return 0
	}
	q.counter++
	q.sequence = q.counter
	q.inflight = true
	return q.sequence
}

// Settle closes one dispatched gesture and hands back the successor to run.
//
// current reports whether the delivered sequence is still the in-flight one. A
// stale delivery changes nothing at all: the gesture it belonged to was
// superseded, and reviving the queue from it would run work against a premise
// that no longer holds.
//
// The successor is taken out of the queue here but armed separately, so the
// caller can complete the committed reload — which supersedes the queue, as
// every committed transaction does — before arming the next one.
func (q *DueStepQueue) Settle(sequence int) (next DueStepGesture, hasNext bool, current bool) {
	if q == nil || !q.inflight || sequence != q.sequence {
		return DueStepGesture{}, false, false
	}
	q.inflight = false
	q.sequence = 0
	if len(q.pending) == 0 {
		return DueStepGesture{}, false, true
	}
	next = q.pending[0]
	q.pending = q.pending[1:]
	return next, true, true
}

// Abandon drops the whole pending sequence after the in-flight gesture was
// refused, and reports how many gestures never ran. A refusal is not retried:
// the tail was frozen against targets and a deadline the refusal proves the
// workspace never accepted, so running it would be acting on a premise the
// application layer has already rejected.
func (q *DueStepQueue) Abandon(sequence int) (dropped int, current bool) {
	if q == nil || !q.inflight || sequence != q.sequence {
		return 0, false
	}
	q.inflight = false
	q.sequence = 0
	dropped = len(q.pending)
	q.pending = nil
	return dropped, true
}

// Supersede abandons queued deadline work whose premise has moved. It detaches
// the in-flight gesture — whose result, when it arrives, is then delivered and
// reloaded but contributes no clock memory — and drops the tail with it.
//
// It says nothing about what moved the premise, because every caller means the
// same thing: some other committed transaction, ledger restore, or route change
// happened between this queue being filled and being drained, so the "preceding
// committed state" each queued step was to be computed from is no longer the one
// the previous step left behind.
func (q *DueStepQueue) Supersede() {
	if q == nil {
		return
	}
	q.inflight = false
	q.sequence = 0
	q.pending = nil
}

// Pending is how many gestures are waiting behind the in-flight one.
func (q *DueStepQueue) Pending() int {
	if q == nil {
		return 0
	}
	return len(q.pending)
}

// StepDueDates commits one deadline gesture as one transaction and reports what
// landed. It is a seam of its own rather than a call to ExecuteOperation because
// only the committed outcomes can say which deadlines this step wrote and which
// it cleared: the generic summary counts tasks, and a step's whole answer is the
// dates.
func (c TaskItemCoordinator) StepDueDates(req app.TaskOperationRequest, hidden int) tea.Cmd {
	return func() tea.Msg {
		result, err := c.task.ExecuteTaskOperation(req)
		if err != nil {
			return TransactionErrorMsg{Err: err}
		}
		summary := DueStepSummary(result.DueSteps, hidden)
		return BatchTransactionResultMsg{
			Action:            "Step Due Date",
			TouchedProjectIDs: result.TouchedProjectIDs,
			Summary:           summary,
			RecoveryContext:   summary,
			TransactionID:     result.TransactionID,
			DueEffect:         CommittedDueEffect{Steps: result.DueSteps},
		}
	}
}

// DueStepSummary renders the status line one committed gesture earns. One task
// reports the deadline it landed on, because that is the only thing the user
// wants to read back and the row may have moved out of view; a selection reports
// how many deadlines were written and how many cleared, plus the hidden members
// that changed without ever being on screen.
//
// It stays terse deliberately: the strip is one line at every supported width,
// so the counts are what survive truncation rather than a sentence around them.
func DueStepSummary(outcomes []app.DueStepOutcome, hidden int) string {
	if len(outcomes) == 0 {
		return "Step Due Date"
	}
	if len(outcomes) == 1 {
		return dueStepDeadlineText(outcomes[0].After)
	}
	set := 0
	for _, outcome := range outcomes {
		if outcome.After != nil {
			set++
		}
	}
	summary := fmt.Sprintf("%d stepped · %d set · %d cleared", len(outcomes), set, len(outcomes)-set)
	if hidden > 0 {
		summary = fmt.Sprintf("%s · %d hidden", summary, hidden)
	}
	return summary
}

// dueStepDeadlineText spells one landed deadline. Local midnight is the durable
// date-only signal the whole deadline domain reads, so a deadline that fell on
// it is named by date alone rather than given a 00:00 the task does not carry.
func dueStepDeadlineText(due *time.Time) string {
	if due == nil {
		return "Due date cleared"
	}
	if due.Hour() == 0 && due.Minute() == 0 {
		return "Due " + due.Format("2006-01-02")
	}
	return "Due " + due.Format("2006-01-02 15:04")
}

// DueStepRefusal names the gestures a refusal cancelled. The error itself is
// wrapped rather than restated, so the application layer keeps saying why, and
// the count is added only when something actually went unexecuted: a refusal
// with an empty queue behind it has nothing extra to report.
func DueStepRefusal(err error, dropped int) error {
	if err == nil || dropped <= 0 {
		return err
	}
	return fmt.Errorf("%w · %d queued deadline step(s) not applied", err, dropped)
}
