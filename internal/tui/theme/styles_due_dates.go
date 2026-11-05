package theme

import "charm.land/lipgloss/v2"

// The three live due tiers are background badges rather than colored text, so a
// date reads as one bounded object in a row that already tints titles, tags and
// context. Their fills are curated pairs, not palette derivations, so a user
// palette override cannot mint an unreadable badge.
//
// Hue says what a date means. A deadline still ahead, or arriving today, lives in
// the teal band: every section badge is spoken for — the twelve ViewMode fills
// already claim green, orange, blue, indigo, purple, violet, slate, mauve and
// maroon — and teal sits at least 26° from all of them, so a due chip is its own
// object on sight; `TestDueBadgeFillsAvoidSectionBadges` holds that apart. Within
// teal, today and upcoming differ by saturation and weight. A deadline that has
// passed leaves the band for the palette's danger red, because lateness is the
// one date reading that is a problem rather than a plan, and a teal "overdue"
// read as calm. Its crimson is brighter and more saturated than the Canceled
// section's dark maroon, so the two never read as one object.
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
		darkFill: "#8a1c30", darkInk: "#fff1f2",
		lightFill: "#fecdd3", lightInk: "#4c0519",
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

// newReviewLateBadgeStyle is the one chip outside the teal family: a check-in
// whose day has passed is a warning rather than a deadline, so it takes amber at
// the emphasized weight. Its fill is held apart from the P2 title amber and the
// Waiting section fill, so lateness never reads as a priority or a header.
func newReviewLateBadgeStyle() lipgloss.Style {
	return makeDueBadgeStyle(dueBadgeTone{
		darkFill: "#6b4a06", darkInk: "#fffbeb",
		lightFill: "#fde68a", lightInk: "#422006",
		emphasized: true,
	})
}

// Due date styles.
var (
	OverdueStyle = newOverdueBadgeStyle()

	DueSoonStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	DueDateStyle = newDueDateBadgeStyle()

	DueTodayStyle = newDueTodayBadgeStyle()

	ReviewLateStyle = newReviewLateBadgeStyle()
)
