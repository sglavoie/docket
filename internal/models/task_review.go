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
