package theme

import "charm.land/lipgloss/v2"

// The three live due tiers are background badges rather than colored text, so a
// date reads as one bounded object in a row that already tints titles, tags and
// context. Their fills are curated pairs, not palette derivations, so a user
// palette override cannot mint an unreadable badge.
//
// The family lives in the teal band because every section badge is spoken for:
// the twelve ViewMode fills already claim green, orange, blue, indigo, purple,
// violet, slate, mauve and maroon, and a due chip that borrowed one of those
// hues read as a section header rather than as a date. Teal sits at least 26°
// from all of them in both themes, so a due chip is its own object on sight;
// `TestDueBadgeFillsAvoidSectionBadges` holds that apart. Urgency is carried by
// saturation within the one hue rather than by hue, keeping lightness inside
// the band the section badges already occupy so a row never glares.
//
// The pairs live in these constructors alone: rebuildTaskStyles repaints the
// same vars on every ApplyTheme, and a second copy of the hex would let the
// startup look and the repainted look drift apart.
func newOverdueBadgeStyle() lipgloss.Style  { return makeDueBadgeStyle("#0a5c54", "#0f766e") }
func newDueTodayBadgeStyle() lipgloss.Style { return makeDueBadgeStyle("#0c453f", "#146e64") }
func newDueDateBadgeStyle() lipgloss.Style  { return makeDueBadgeStyle("#243c3a", "#4a6764") }

// Due date styles.
var (
	OverdueStyle = newOverdueBadgeStyle()

	DueSoonStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	DueDateStyle = newDueDateBadgeStyle()

	DueTodayStyle = newDueTodayBadgeStyle()
)
