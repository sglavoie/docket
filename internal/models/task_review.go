package models

import "time"

// TaskAwaitsReview reports whether a task is one the review pass is for: work
// that is open and has asked, one way or another, to be looked at again. The
// three ways are independent and deliberately unranked — a task flagged with
// `=`, a task the stale predicate has stopped hearing from, and a task parked in
// Waiting on someone else — so a task qualifying twice is no more a candidate
// than one qualifying once.
//
// Deferral needs no clause of its own. A currently deferred task cannot be stale
// (QueryTaskIsStale refuses it) and Waiting is a state a deferral does not
// claim, so the only deferred task that answers here is one explicitly flagged,
// which is the caller having said out loud that it wants this one back.
//
// Terminal work is refused outright: a done or canceled task has nothing left to
// acknowledge, and admitting one would let a review pass touch completion data.
func TaskAwaitsReview(task *Task, now time.Time) bool {
	if task == nil || task.IsTerminalState() {
		return false
	}
	return task.Review || task.IsInState(StateWaiting) || QueryTaskIsStale(task, now)
}

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
// TaskAwaitsReview still answers the older question for the consumers that have
// not moved; the two predicates are kept apart on purpose.
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
