package theme

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"
)

// dueBadgeTier names one due chip. The three are asserted together because the
// badge contract is a family property: a tier that lost its fill would read as
// bare text beside two chips rather than as a quieter chip.
type dueBadgeTier struct {
	name  string
	style lipgloss.Style
}

func dueBadgeTiers() []dueBadgeTier {
	return []dueBadgeTier{
		{"overdue", OverdueStyle},
		{"today", DueTodayStyle},
		{"upcoming", DueDateStyle},
	}
}

func TestDueBadgesKeepDistinctFillsAcrossThemes(t *testing.T) {
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	for _, themeName := range []string{"dark", "light"} {
		t.Run(themeName, func(t *testing.T) {
			ApplyTheme(ThemeConfig{Name: themeName})

			tiers := dueBadgeTiers()
			for _, tier := range tiers {
				// An unset color reads back as NoColor{}, never nil, so that is
				// what "this tier lost its fill" actually looks like.
				if tier.style.GetBackground() == (lipgloss.NoColor{}) {
					t.Fatalf("%s due badge has no background, so the chip reads as bare text", tier.name)
				}
				if tier.style.GetForeground() == (lipgloss.NoColor{}) {
					t.Fatalf("%s due badge has no foreground, so the fill has no contrast contract", tier.name)
				}
				// The section badge's two padding cells are what bound the chip;
				// outlinerNonTitleWidth measures them like any other segment.
				if left, right := tier.style.GetPaddingLeft(), tier.style.GetPaddingRight(); left != 1 || right != 1 {
					t.Fatalf("%s due badge padding = %d/%d, want 1/1", tier.name, left, right)
				}
				// Bold stays the title's selection channel, exactly as chips hold
				// elsewhere.
				if tier.style.GetBold() {
					t.Fatalf("%s due badge is bold, which spends the title's selection channel", tier.name)
				}
			}

			for i := range tiers {
				for j := i + 1; j < len(tiers); j++ {
					a, b := tiers[i], tiers[j]
					if a.style.GetBackground() == b.style.GetBackground() {
						t.Fatalf("%s and %s due badges share fill %v, so urgency stops reading as tone",
							a.name, b.name, a.style.GetBackground())
					}
				}
			}
		})
	}
}

// TestDueBadgeFillsAvoidSectionBadges is the guard the first badge palette
// lacked: it borrowed ViewModeWaitingStyle's amber and ViewModeCompletedStyle's
// slate outright, so a due chip read as a section header. A due date is not a
// section, and no fill may say otherwise.
func TestDueBadgeFillsAvoidSectionBadges(t *testing.T) {
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	sections := func() []struct {
		name  string
		style lipgloss.Style
	} {
		return []struct {
			name  string
			style lipgloss.Style
		}{
			{"active", ViewModeActiveStyle},
			{"work", ViewModeWorkStyle},
			{"next", ViewModeNextStyle},
			{"inbox", ViewModeInboxStyle},
			{"waiting", ViewModeWaitingStyle},
			{"someday", ViewModeSomedayStyle},
			{"completed", ViewModeCompletedStyle},
			{"standby", ViewModeStandbyStyle},
			{"deferred", ViewModeDeferredStyle},
			{"canceled", ViewModeCanceledStyle},
			{"all", ViewModeAllStyle},
			{"review", ViewModeReviewStyle},
		}
	}

	for _, themeName := range []string{"dark", "light"} {
		t.Run(themeName, func(t *testing.T) {
			ApplyTheme(ThemeConfig{Name: themeName})
			for _, tier := range dueBadgeTiers() {
				for _, section := range sections() {
					if tier.style.GetBackground() == section.style.GetBackground() {
						t.Fatalf("%s due badge reuses the %s section fill %v, so a date reads as a section header",
							tier.name, section.name, tier.style.GetBackground())
					}
				}
			}
		})
	}
}

func TestDueBadgeFillsTrackTheActiveTheme(t *testing.T) {
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	ApplyTheme(ThemeConfig{Name: "dark"})
	dark := map[string]color.Color{}
	for _, tier := range dueBadgeTiers() {
		dark[tier.name] = tier.style.GetBackground()
	}

	ApplyTheme(ThemeConfig{Name: "light"})
	for _, tier := range dueBadgeTiers() {
		if got := tier.style.GetBackground(); got == dark[tier.name] {
			t.Fatalf("%s due badge kept its dark fill %v under the light theme", tier.name, got)
		}
	}
}
