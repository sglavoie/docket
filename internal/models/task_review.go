package models

import "time"

// This file owns the two review questions and keeps them apart. One is
// automatic — which tasks are asking to be looked at, without anyone having gone
// looking — and it is what a review pass navigates by. The other is explicit —
// which tasks a person may acknowledge when they name one — and it is strictly
// wider, because an early review of scheduled work is allowed.
//
// Deliberately absent: a single predicate spanning both. The retired
// TaskAwaitsReview answered "flagged, stale, or Waiting" for every consumer at
// once, which is what made the old pass a list of everything: staleness and
// Waiting are cleanup material a person goes looking for with `fs` and `?stale`,
// not work that raised its hand. Neither predicate here admits them.

// TaskNeedsAttention reports whether a task is asking to be looked at right
// now, without anyone having gone looking for it. There are exactly two ways to
// ask: the task carries the stored review flag, or it carries a scheduled
// check-in whose day has arrived and is eligible to be raised — open,
// undeferred, and not terminal, which is what Task.CheckInIsDueAt decides.
//
// Staleness and Waiting are deliberately absent. An untouched task and a task
// parked on someone else are cleanup material a person goes looking for, not
// work that raised its hand, and admitting them automatically is what made the
// old review pass a list of everything rather than a list of what is owed.
//
// This is the predicate a review pass navigates by, so acknowledging a task
// always takes it out of the sequence: the flag comes off and the schedule moves
// to its next occurrence or clears. A pass therefore ends rather than circling.
func TaskNeedsAttention(task *Task, now time.Time) bool {
	if task == nil || task.IsTerminalState() {
		return false
	}
	return task.Review || task.CheckInIsDueAt(now)
}

// TaskAllowsExplicitReview reports whether a task may be reviewed when someone
// asks for it by name. It is TaskNeedsAttention plus one case: scheduled work
// whose day has not come yet, which is what makes an early review possible.
//
// A deferred task with a schedule answers here even though it never raises
// itself. Deferral suppresses the reminder, which is the automatic half; a
// person who has navigated to the task and pressed the key has already said out
// loud that they want this one now, and refusing them would be the deferral
// deciding something it was never asked about. Terminal work is refused: it has
// no schedule and nothing left to acknowledge.
func TaskAllowsExplicitReview(task *Task, now time.Time) bool {
	if task == nil || task.IsTerminalState() {
		return false
	}
	return TaskNeedsAttention(task, now) || task.NextCheckIn != nil
}
