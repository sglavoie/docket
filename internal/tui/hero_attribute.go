package tui

import (
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

// HeroKind identifies which row attribute earns full saturation under the
// Von Restorff rule: at most one badge per row is rendered at full saturation
// (the "hero"); the rest demote to MutedStyle so the eye can target the most
// urgent attribute. Precedence is overdue > blocked > P1 > in_review.
type HeroKind int

const (
	HeroNone HeroKind = iota
	HeroOverdue
	HeroBlocked
	HeroP1
	HeroReview
)

// pickHeroAttribute returns the highest-precedence hero attribute for a task.
// Returns HeroNone when no attribute qualifies (the row keeps current badge
// styling; nothing is demoted). The project is required to evaluate blocked-by-
// dependencies; pass nil to skip that branch.
func pickHeroAttribute(t *models.Task, p *models.Project) HeroKind {
	if t == nil {
		return HeroNone
	}
	if t.IsOverdue() {
		return HeroOverdue
	}
	if p != nil && p.HasIncompleteDependencies(t) {
		return HeroBlocked
	}
	if t.Priority == 1 {
		return HeroP1
	}
	if t.Status == models.StatusReview {
		return HeroReview
	}
	return HeroNone
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
func dueIsHero(h HeroKind) bool { return h == HeroOverdue }
