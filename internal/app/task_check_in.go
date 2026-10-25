package app

import (
	"fmt"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

// This file owns the durable half of scheduling a check-in: one typed schedule
// written across a frozen selection of any size, as one transaction carrying one
// undo entry.
//
// Scheduling is deliberately not a planning gesture. A check-in due today says
// "look at this again", not "work on this today", so nothing here builds a
// planning artifact, names a PlanningReconcileTarget, or touches Today or Next —
// which is also why it needs no planning artifact builder to run at all. It is
// deliberately not an acknowledgement either: models.Task.SetCheckIn leaves the
// review flag, LastReviewedAt, the lifecycle state, and the state clock exactly
// as it found them, because deciding when to look at something again says
// nothing about having looked at it now.

// BulkSetCheckIn writes one schedule onto every selected task, committing every
// touched project as ONE transaction. An empty schedule clears, which is what
// makes this one operation rather than two paths a caller has to choose between:
// models.CheckInSchedule's zero value already means "no schedule", so the clear
// is the set of nothing and both halves obey the same gate.
//
// The whole target set is judged before any task is touched. The value is
// validated once — it is uniform across the batch — and every resolved target is
// checked against the workspace the commit will write, so an unresolvable
// locator or a terminal target refuses the batch entire, with no partial
// schedule written, no planning moved, and no undo entry recorded. A selection
// that half-succeeded would be worse than one that refused: the caller would
// have no way to tell which half landed.
//
// Scheduling is always explicit, so it takes no eligibility guard of its own: a
// caller naming a task has said which task, and refusing because that task was
// scheduled or left active work in the meantime would refuse the very correction
// the user is making.
func (s *Service) BulkSetCheckIn(locators []TaskLocator, schedule models.CheckInSchedule) (BulkResult, error) {
	if err := validateOperationTargets(locators); err != nil {
		return BulkResult{}, err
	}
	if err := models.ValidateCheckInSchedule(schedule); err != nil {
		return BulkResult{}, invalidInput("check-in", err.Error())
	}
	action, context := checkInUndoEntry(schedule)
	return s.bulkAttributeOverSnapshot(locators, action, context, func(_ Snapshot, refs []TaskRef) (func(TaskRef) error, error) {
		if err := checkInTargetsAccept(refs, schedule); err != nil {
			return nil, err
		}
		// One clock read for the whole batch: a schedule written across midnight
		// must not touch half its targets on one day and half on the next.
		now := s.clock.Now()
		return func(ref TaskRef) error {
			return applyCheckInSchedule(ref.Task, schedule, now)
		}, nil
	})
}

// checkInTargetsAccept is the whole workspace half of the validation, run
// against the very snapshot the commit writes and before any task is mutated.
//
// A terminal target refuses a schedule by name. Terminal work owes no further
// look — the lifecycle primitive drops the schedule the moment a task finishes,
// the canonical writer refuses a document carrying one, and the merge audit
// records a conflict when a merge composes one — so admitting it here would mint
// exactly the state three other layers exist to refuse.
//
// Clearing is the one thing a terminal target does not refuse. It can only be a
// no-op, since a terminal task holds no schedule to begin with, and refusing
// would mean a selection containing one finished task could never have its
// check-ins cleared.
func checkInTargetsAccept(refs []TaskRef, schedule models.CheckInSchedule) error {
	if schedule.IsEmpty() {
		return nil
	}
	for _, ref := range refs {
		if ref.Task.IsTerminalState() {
			return invalidInput("check-in", fmt.Sprintf(
				"#%d is %s; a finished task owes no check-in", ref.Task.ShortID, ref.Task.State()))
		}
	}
	return nil
}

// applyCheckInSchedule routes one target through the model's own gate. The
// schedule was validated once for the batch, so a refusal here would be a
// programming error rather than a caller's; it is still returned rather than
// dropped, because the alternative is a task silently keeping the schedule the
// commit claims to have replaced.
func applyCheckInSchedule(task *models.Task, schedule models.CheckInSchedule, now time.Time) error {
	if schedule.IsEmpty() {
		task.ClearCheckIn(now)
		return nil
	}
	if err := task.SetCheckIn(schedule, now); err != nil {
		return invalidInput("check-in", err.Error())
	}
	return nil
}

// checkInUndoEntry names the ledger entry after what the transaction did rather
// than after the operation that produced it, so `u` offers "Clear Check-In" for
// a clear and names the date and cadence a schedule actually wrote. The context
// is the committed value, which is the one thing a caller reading the ledger
// cannot recover from the entry's action alone.
func checkInUndoEntry(schedule models.CheckInSchedule) (action, context string) {
	if schedule.IsEmpty() {
		return "Clear Check-In", "check-in"
	}
	context = schedule.Next.Format(checkInUndoDateLayout)
	if schedule.Repeats() {
		context = fmt.Sprintf("%s every %d days", context, schedule.RepeatDays)
	}
	return "Set Check-In", context
}

// checkInUndoDateLayout is the canonical calendar spelling the durable document
// stores a check-in in, reused for the ledger context so an undo entry and the
// stored value read alike.
const checkInUndoDateLayout = "2006-01-02"
