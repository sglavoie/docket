package tui

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// RecommendationKind classifies the category of next action the engine
// suggests to the user. The Today cockpit (sessions 4-6) renders one kind at
// a time and chooses copy/affordances based on the kind.
type RecommendationKind string

const (
	// RecommendationKindNone is the zero value: no recommendation is available
	// (no projects, no tasks, or the engine has not produced output yet).
	RecommendationKindNone RecommendationKind = ""

	// RecommendationKindWorkOnTask suggests resuming or starting work on the
	// task carried in RecommendedAction.Task. Most populated cockpit states
	// land here.
	RecommendationKindWorkOnTask RecommendationKind = "work_on_task"

	// RecommendationKindTriageInbox suggests reviewing inbox-section tasks
	// because nothing is planned for today and the inbox has actionable items.
	RecommendationKindTriageInbox RecommendationKind = "triage_inbox"

	// RecommendationKindReviewBlocked suggests revisiting a blocked task to
	// either unblock it or move it elsewhere.
	RecommendationKindReviewBlocked RecommendationKind = "review_blocked"

	// RecommendationKindPlanToday suggests planning the day because tasks
	// exist but none are scheduled for today.
	RecommendationKindPlanToday RecommendationKind = "plan_today"

	// RecommendationKindAllDone signals every planned task for today is
	// complete and nothing else needs immediate attention.
	RecommendationKindAllDone RecommendationKind = "all_done"
)

// IsValid reports whether k is one of the known recommendation kinds.
// RecommendationKindNone is considered valid because callers may legitimately
// hold a zero-value recommendation while loading.
func (k RecommendationKind) IsValid() bool {
	switch k {
	case RecommendationKindNone,
		RecommendationKindWorkOnTask,
		RecommendationKindTriageInbox,
		RecommendationKindReviewBlocked,
		RecommendationKindPlanToday,
		RecommendationKindAllDone:
		return true
	}
	return false
}

// String returns the string form of the kind.
func (k RecommendationKind) String() string { return string(k) }

// AllRecommendationKinds returns every known recommendation kind in the order
// the engine evaluates them. RecommendationKindNone is omitted because it is
// the zero value, not a state the engine actively produces.
func AllRecommendationKinds() []RecommendationKind {
	return []RecommendationKind{
		RecommendationKindWorkOnTask,
		RecommendationKindTriageInbox,
		RecommendationKindReviewBlocked,
		RecommendationKindPlanToday,
		RecommendationKindAllDone,
	}
}

// RecommendationReason is an explicit, user-facing rationale attached to a
// RecommendedAction. The Today cockpit renders these verbatim as the
// "because…" line under the next-action strip, so values must read as
// English copy and stay short enough for a single visual line.
type RecommendationReason string

const (
	// ReasonPlannedToday — task is on the daily plan for today.
	ReasonPlannedToday RecommendationReason = "planned today"

	// ReasonOverdue — task has a due date that has already passed.
	ReasonOverdue RecommendationReason = "overdue"

	// ReasonDueToday — task is due today (same calendar day as the engine clock).
	ReasonDueToday RecommendationReason = "due today"

	// ReasonHighPriority — task carries priority 1 or 2 (critical/high).
	ReasonHighPriority RecommendationReason = "high priority"

	// ReasonInboxNeedsTriage — actionable tasks sit in the Inbox section
	// awaiting GTD-style triage.
	ReasonInboxNeedsTriage RecommendationReason = "inbox needs triage"

	// ReasonBlocked — task status is models.StatusBlocked or at least one
	// dependency is incomplete or missing.
	ReasonBlocked RecommendationReason = "blocked"

	// ReasonStale — task has not been updated for a long stretch (engine
	// chooses the threshold; the reason copy is stable).
	ReasonStale RecommendationReason = "stale"

	// ReasonCarriedOver — task was planned for a previous day and remains
	// open today.
	ReasonCarriedOver RecommendationReason = "carried over"

	// ReasonRecentlyActive — task has had user activity inside the engine's
	// recent-momentum window.
	ReasonRecentlyActive RecommendationReason = "recently active"
)

// IsValid reports whether r is one of the known recommendation reasons.
func (r RecommendationReason) IsValid() bool {
	switch r {
	case ReasonPlannedToday,
		ReasonOverdue,
		ReasonDueToday,
		ReasonHighPriority,
		ReasonInboxNeedsTriage,
		ReasonBlocked,
		ReasonStale,
		ReasonCarriedOver,
		ReasonRecentlyActive:
		return true
	}
	return false
}

// String returns the string form of the reason — the same value the cockpit
// renders. Label is provided as an alias for callers that prefer the
// presentation-oriented name without committing future copy changes to the
// String() contract.
func (r RecommendationReason) String() string { return string(r) }

// Label returns the user-facing label for the reason. Identical to String()
// today; kept separate so the rendering layer can swap copy without touching
// every comparison site.
func (r RecommendationReason) Label() string { return string(r) }

// AllRecommendationReasons returns every known recommendation reason in
// engine-evaluation order. The cockpit copy and contract tests depend on the
// full set being enumerable.
func AllRecommendationReasons() []RecommendationReason {
	return []RecommendationReason{
		ReasonPlannedToday,
		ReasonOverdue,
		ReasonDueToday,
		ReasonHighPriority,
		ReasonInboxNeedsTriage,
		ReasonBlocked,
		ReasonStale,
		ReasonCarriedOver,
		ReasonRecentlyActive,
	}
}

// RecommendedAction is a deterministic snapshot produced by the recommendation
// engine. Task and Project are populated for per-task kinds
// (WorkOnTask, ReviewBlocked); they are nil for cockpit-wide kinds
// (TriageInbox, PlanToday, AllDone). GeneratedAt records the engine clock so
// the cockpit can show "as of HH:MM" without recomputing.
//
// Reasons is the ordered list of rationales the cockpit will render. It must
// contain only RecommendationReason values that pass IsValid(); duplicates are
// preserved so the engine can intentionally emphasize a rationale (e.g.,
// "overdue" twice when both due-date and carry-over windows match).
type RecommendedAction struct {
	Kind        RecommendationKind
	Task        *models.Task
	Project     *models.Project
	Reasons     []RecommendationReason
	GeneratedAt time.Time
}

// HasReason reports whether the action carries the given reason. Nil-receiver
// safe so callers can chain on engine output without intermediate guards.
func (a *RecommendedAction) HasReason(r RecommendationReason) bool {
	if a == nil {
		return false
	}
	for _, candidate := range a.Reasons {
		if candidate == r {
			return true
		}
	}
	return false
}

// IsEmpty reports whether the action is the zero value or carries no kind
// and no task. Nil-receiver safe.
func (a *RecommendedAction) IsEmpty() bool {
	if a == nil {
		return true
	}
	return a.Kind == RecommendationKindNone && a.Task == nil && a.Project == nil && len(a.Reasons) == 0
}
