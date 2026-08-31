package models

import (
	"time"

	"github.com/sglavoie/tt/internal/util"
)

// DerivedProperty names one computed question about a task: something the task
// does not store, so no stored-field compare can answer it. The names live here
// rather than in the grammar because the answers are task invariants;
// internal/textgrammar owns only the spelling that reaches them.
type DerivedProperty string

const (
	// DerivedStale is the same threshold the Stale lane admits on.
	DerivedStale DerivedProperty = "stale"
	// DerivedDeferred matches a deferral that has not expired yet.
	DerivedDeferred DerivedProperty = "deferred"
	// DerivedLeaf and DerivedParent split the population on the open direct child
	// census, which is the same count a row's child marker reads.
	DerivedLeaf   DerivedProperty = "leaf"
	DerivedParent DerivedProperty = "parent"
	// DerivedCompleted matches a usable completion instant, optionally at least a
	// given number of days old.
	DerivedCompleted DerivedProperty = "completed"
	// DerivedBlocked and DerivedBlocking are structural rather than readiness
	// verdicts: one reports that the task names an outgoing reference and the
	// other that something names it. A soft reference decides no readiness,
	// ordering, ranking, or completion, so neither reading looks at the other
	// end's state.
	DerivedBlocked  DerivedProperty = "blocked"
	DerivedBlocking DerivedProperty = "blocking"
)

// DerivedProperties is the closed set, in the order a surface teaches it.
var DerivedProperties = []DerivedProperty{
	DerivedStale,
	DerivedDeferred,
	DerivedLeaf,
	DerivedParent,
	DerivedCompleted,
	DerivedBlocked,
	DerivedBlocking,
}

// DerivedCensus carries the two facts a task cannot answer alone: how many open
// direct children it was handed over with, and whether anything in the loaded
// population references it. A caller builds only the halves its query asked for,
// which is what DerivedNeedsChildCensus and DerivedNeedsDependentCensus report.
type DerivedCensus struct {
	OpenChildren  int
	HasDependents bool
}

// TaskMatchesDerived answers one derived property for one task. days is read by
// DerivedCompleted alone and means "at least that many days ago"; zero asks the
// unqualified question.
func TaskMatchesDerived(
	property DerivedProperty,
	days int,
	task *Task,
	census DerivedCensus,
	now time.Time,
) bool {
	if task == nil {
		return false
	}
	switch property {
	case DerivedStale:
		return QueryTaskIsStale(task, now)
	case DerivedDeferred:
		return task.IsDeferredAt(now)
	case DerivedLeaf:
		return census.OpenChildren == 0
	case DerivedParent:
		return census.OpenChildren > 0
	case DerivedCompleted:
		return taskCompletedAtLeastDaysAgo(task, days, now)
	case DerivedBlocked:
		return len(task.Dependencies) > 0
	case DerivedBlocking:
		return census.HasDependents
	default:
		return false
	}
}

// DerivedNeedsChildCensus reports whether a property reads the open direct child
// count, so a query that asks nothing about the tree walks none of it.
func DerivedNeedsChildCensus(property DerivedProperty) bool {
	return property == DerivedLeaf || property == DerivedParent
}

// DerivedNeedsDependentCensus reports whether a property reads the incoming
// reference census, which is the one derived answer that has to scan the other
// projects rather than the task in hand.
func DerivedNeedsDependentCensus(property DerivedProperty) bool {
	return property == DerivedBlocking
}

// DerivedOpensTerminalPopulation names the one property whose positive form asks
// about finished work and therefore admits the terminal states the way a done
// destination does. The negation deliberately does not: "not completed recently"
// is a question about live work.
func DerivedOpensTerminalPopulation(property DerivedProperty) bool {
	return property == DerivedCompleted
}

// DerivedOpensDeferredPopulation names the one property whose positive form asks
// about work the base population hides behind a live deferral. It is the same
// rule DerivedOpensTerminalPopulation states, read on the other axis, and the
// negation is again a question about work already on screen.
func DerivedOpensDeferredPopulation(property DerivedProperty) bool {
	return property == DerivedDeferred
}

// taskCompletedAtLeastDaysAgo keeps the live cleanup facet's shape: a task with
// no usable completion timestamp is excluded even when no age bound is set, so a
// malformed record can never answer a completion question.
func taskCompletedAtLeastDaysAgo(task *Task, days int, now time.Time) bool {
	if task.CompletedAt == nil || task.CompletedAt.IsZero() {
		return false
	}
	if days <= 0 {
		return true
	}
	return util.DaysBetween(task.CompletedAt.In(now.Location()), now) >= days
}
