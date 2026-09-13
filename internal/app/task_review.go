package app

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
)

// MarkReviewed acknowledges one task by name: the review flag comes off, the
// review history is stamped, a scheduled check-in is answered, and the touch
// clock moves, in one transaction carrying one undo entry. Nothing else is
// written — the lifecycle state and its clock, completion data, deadlines,
// deferral, planning membership, pin, resume note, and body all survive — so a
// Waiting task stays Waiting with its check-back date and its waiting age
// unreset.
//
// Admission is models.TaskAllowsExplicitReview, which is the wider of the two
// review questions on purpose: someone who has navigated to a task and pressed
// the key may review scheduled work early, before its day arrives. It is
// deliberately not models.TaskNeedsAttention — that one is what a pass navigates
// by — and it is no longer the retired flagged-stale-or-Waiting predicate: a
// task nobody flagged and nothing scheduled has nothing here to acknowledge, and
// staleness is cleanup `fs` and `?stale` answer.
//
// Eligibility is rechecked here against this service's clock rather than trusted
// from the caller. A surface admits the operation from what it drew, and what it
// drew can be stale: the flag may have been cleared elsewhere, the task may have
// been completed, or a schedule may have moved.
//
// The clock is read once and used for the admission test, the successor date,
// and the write alike. A check-in that arrives at midnight must not be admitted
// by one reading of "now" and then rescheduled from another.
func (s *Service) MarkReviewed(locator TaskLocator) (TaskResult, error) {
	ref, _, revision, err := s.loadTaskSelection("", locator)
	if err != nil {
		return TaskResult{}, err
	}
	now := s.clock.Now()
	if !models.TaskAllowsExplicitReview(ref.Task, now) {
		return TaskResult{}, invalidInput("task", "task has no review to acknowledge")
	}
	undoContext := fmt.Sprintf("#%d %s", ref.Task.ShortID, ref.Task.Title)
	// The acknowledgement also answers a scheduled check-in, and a cadence that
	// cannot land on a storable date is refused before anything is written, so
	// the task is left exactly as it was loaded rather than committed half done.
	if err := ref.Task.AcknowledgeReview(now); err != nil {
		return TaskResult{}, invalidInput("task", err.Error())
	}
	return s.commitProject(ref, revision, "Mark Reviewed", undoContext)
}
