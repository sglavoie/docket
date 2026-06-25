package tui

import (
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

// HeroKind identifies which row attribute earns full saturation under the
// Von Restorff rule: at most one badge per row is rendered at full saturation
// (the "hero"); the rest demote to MutedStyle so the eye can target the most
// urgent attribute.
//
// Dense-row hierarchy (top to bottom of visual priority):
//  1. Urgency hero — at most one badge per row, picked from the precedence
//     ladder below. All other supporting badges demote to MutedStyle so the
//     eye lands on the hero first.
//  2. Title — the primary text of the row, always readable. Title styling is
//     handled by TaskTitleStyleFor; the hero policy does not touch the title.
//  3. Quiet supporting metadata — non-hero badges (status, type, priority,
//     due) are demoted so they read as secondary context.
//
// Hero precedence is urgency-only (first match wins):
//  1. HeroOverdue   — task is past due
//  2. HeroBlocked   — explicit StatusBlocked OR incomplete dependencies
//  3. HeroDueToday  — task is due today (and not yet overdue)
//  4. HeroP1        — Priority == 1
//  5. HeroReview    — StatusReview
//
// Quiet states (always return HeroNone so no badge competes for attention):
// completed tasks (StatusDone, StatusCanceled) and explicitly dimmed rows.
type HeroKind int

const (
	HeroNone HeroKind = iota
	HeroOverdue
	HeroBlocked
	HeroDueToday
	HeroP1
	HeroReview
)

// pickHeroAttribute returns the highest-precedence hero attribute for a task.
// Returns HeroNone when no attribute qualifies (the row keeps current badge
// styling; nothing is demoted) and for quiet states — completed, canceled,
// and dimmed tasks never claim the hero slot. The project is required to
// evaluate blocked-by-dependencies; pass nil to skip that branch.
func pickHeroAttribute(t *models.Task, p *models.Project) HeroKind {
	if t == nil {
		return HeroNone
	}
	if t.IsComplete() || t.Dimmed {
		return HeroNone
	}
	if t.IsOverdue() {
		return HeroOverdue
	}
	if t.Status == models.StatusBlocked || (p != nil && p.HasIncompleteDependencies(t)) {
		return HeroBlocked
	}
	if t.IsDueToday() {
		return HeroDueToday
	}
	if t.Priority == 1 {
		return HeroP1
	}
	if t.Status == models.StatusReview {
		return HeroReview
	}
	return HeroNone
}

// dueIsInlineHero reports whether a task's due date may be rendered as the
// dense-row urgency anchor. It mirrors the quiet-state guard used by
// pickHeroAttribute so completed, canceled, and dimmed rows never claim due
// urgency inline.
func dueIsInlineHero(t *models.Task) bool {
	return t != nil && !t.IsComplete() && !t.Dimmed && (t.IsOverdue() || t.IsDueToday())
}

// demoteBadge re-renders a pre-styled badge with MutedStyle, preserving the
// glyph and the visual width (trailing padding from PadBadge is untouched).
// Returns the input unchanged when empty.
func demoteBadge(badge string) string {
	if badge == "" {
		return ""
	}
	return MutedStyle.Render(ansi.Strip(badge))
}

// statusIsHero reports whether the status badge is the hero for a given kind.
func statusIsHero(h HeroKind) bool { return h == HeroBlocked || h == HeroReview }

// priorityIsHero reports whether the priority badge is the hero.
func priorityIsHero(h HeroKind) bool { return h == HeroP1 }

// dueIsHero reports whether the due-date string is the hero.
func dueIsHero(h HeroKind) bool { return h == HeroOverdue || h == HeroDueToday }
