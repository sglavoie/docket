package chrome

import (
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// HeroKind identifies which attention signal a row is allowed to shout with.
// At most one attribute per row is the hero; the supporting badges beside it
// demote to MutedStyle so the eye lands on the most time-sensitive thing first.
//
// Dense-row hierarchy (top to bottom of visual priority):
//  1. Attention hero — at most one per row, picked from the precedence ladder
//     below. Supporting badges demote to MutedStyle.
//  2. Title — the primary text of the row, always readable and always neutral.
//     TaskTitleStyleFor colors a title for lifecycle reasons alone; no attention
//     signal reaches it.
//  3. Identity — the workflow-state badge. It says what the row *is* rather than
//     how loudly it is asking, so it is never demoted and never the hero.
//  4. Quiet supporting metadata — the non-hero attention badges.
//
// Hero precedence is attention-only (first match wins):
//  1. HeroOverdue   — the deadline has passed
//  2. HeroReview    — the task is asking to be looked at (models.TaskNeedsAttention)
//  3. HeroDueToday  — the deadline is today (and not yet overdue)
//  4. HeroP1        — Priority == 1
//
// Waiting is deliberately absent. An ordinary Waiting row is quiet work parked
// on someone else's desk: its state badge and its `waiting <age>` segment say so
// without the row claiming urgency it does not have. A Waiting task earns a hero
// the same way any other task does — by being overdue, by owing a review, or by
// its deadline and priority — never by its state.
//
// Quiet states (always return HeroNone so no badge competes for attention):
// terminal tasks (done, canceled).
type HeroKind int

const (
	HeroNone HeroKind = iota
	HeroOverdue
	HeroReview
	HeroDueToday
	HeroP1
)

// PickHeroAttribute returns the highest-precedence attention attribute for a
// task at now. Returns HeroNone when no attribute qualifies (the row keeps
// current badge styling; nothing is demoted) and for quiet states — completed
// and canceled tasks never claim the hero slot.
//
// now is a parameter rather than a call to time.Now inside, because the review
// half of the ladder is a calendar-day question that models.TaskNeedsAttention
// answers against a supplied clock: one row-render pass therefore reads one
// instant for its hero, its review segment, and its ages.
func PickHeroAttribute(t *models.Task, now time.Time) HeroKind {
	if t == nil {
		return HeroNone
	}
	if t.IsTerminalState() {
		return HeroNone
	}
	if t.IsOverdueAt(now) {
		return HeroOverdue
	}
	if models.TaskOwesReviewAttention(nil, t, now) {
		return HeroReview
	}
	if t.IsDueTodayAt(now) {
		return HeroDueToday
	}
	if t.Priority == 1 {
		return HeroP1
	}
	return HeroNone
}

// DemoteBadge re-renders a pre-styled badge with MutedStyle, preserving the
// glyph and the visual width (trailing padding from PadBadge is untouched).
// Returns the input unchanged when empty.
func DemoteBadge(badge string) string {
	if badge == "" {
		return ""
	}
	return theme.MutedStyle.Render(ansi.Strip(badge))
}

// PriorityIsHero reports whether the priority badge is the hero. It is the one
// projection of HeroKind a row renderer needs: the state badge is identity and
// keeps its own colors on every row, while the deadline and review segments
// carry their own tiers, so priority is the only badge whose salience the ladder
// decides.
func PriorityIsHero(h HeroKind) bool { return h == HeroP1 }
