package app

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
)

// MarkReviewed acknowledges one task that the review pass is for: the review
// flag comes off and the touch clock moves, in one transaction carrying one undo
// entry. Nothing else is written — the lifecycle state and its clock, completion
// data, dates, deferral, planning membership, pin, resume note, and body all
// survive — so a Waiting task stays Waiting with its check-back date and its
// waiting age unreset.
//
// Eligibility is rechecked here against this service's clock rather than trusted
// from the caller. A surface admits the operation from what it drew, and what it
// drew can be stale: the flag may have been cleared elsewhere, the task may have
// been completed, or a deferral may have moved. models.TaskAwaitsReview is the
// one predicate both readings use, so the surface and the commit cannot disagree
// about what a candidate is.
//
// The clock is read once and used for the admission test and the write alike. A
// task that is stale by a second must not be admitted by one reading of "now"
// and then stamped with another.
func (s *Service) MarkReviewed(locator TaskLocator) (TaskResult, error) {
	ref, _, revision, err := s.loadTaskSelection("", locator)
	if err != nil {
		return TaskResult{}, err
	}
	now := s.clock.Now()
	if !models.TaskAwaitsReview(ref.Task, now) {
		return TaskResult{}, invalidInput("task", "task is not awaiting review")
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
