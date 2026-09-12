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
// Two of the three tiers are emphasized, and emphasis is a pair: bold, and ink
// pushed away from the fill rather than toward it. A deadline that has arrived
// or passed is the one date a row cannot afford to let a glance miss, so overdue
// and today take the loudest ink each paper allows — near-white on the dark
// theme, near-black on the light one, which is why the light fills are pale
// rather than the saturated mid-tones white text needed. Upcoming keeps the
// quieter pairing on both, so the tier a user still has time for reads as tone
// while the two that demand an answer read as weight.
//
// The pairs live in these constructors alone: rebuildTaskStyles repaints the
// same vars on every ApplyTheme, and a second copy of the hex would let the
// startup look and the repainted look drift apart.
func newOverdueBadgeStyle() lipgloss.Style {
	return makeDueBadgeStyle(dueBadgeTone{
		darkFill: "#0a5c54", darkInk: "#f0fdfa",
		lightFill: "#5eead4", lightInk: "#042f2e",
		emphasized: true,
	})
}

func newDueTodayBadgeStyle() lipgloss.Style {
	return makeDueBadgeStyle(dueBadgeTone{
		darkFill: "#0c453f", darkInk: "#f0fdfa",
		lightFill: "#99f6e4", lightInk: "#042f2e",
		emphasized: true,
	})
}

func newDueDateBadgeStyle() lipgloss.Style {
	return makeDueBadgeStyle(dueBadgeTone{
		darkFill: "#243c3a", darkInk: "#a9b1d6",
		lightFill: "#dbeeeb", lightInk: "#3f5f5b",
	})
}

// Due date styles.
var (
	OverdueStyle = newOverdueBadgeStyle()

	DueSoonStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	DueDateStyle = newDueDateBadgeStyle()

	DueTodayStyle = newDueTodayBadgeStyle()
)
