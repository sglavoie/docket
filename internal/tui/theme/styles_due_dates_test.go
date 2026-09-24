package theme

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"
)

// dueBadgeTier names one due chip. The three are asserted together because the
// badge contract is a family property: a tier that lost its fill would read as
// bare text beside two chips rather than as a quieter chip, and a tier that lost
// or gained emphasis would put the deadline that is now and the one that is
// later at the same weight.
type dueBadgeTier struct {
	name       string
	style      lipgloss.Style
	emphasized bool
}

func dueBadgeTiers() []dueBadgeTier {
	return []dueBadgeTier{
		{"overdue", OverdueStyle, true},
		{"today", DueTodayStyle, true},
		{"upcoming", DueDateStyle, false},
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
				// Emphasis is the one attribute that survives --no-color, where
				// every fill drops away, so the two tiers that demand an answer
				// spend it and the tier the user still has time for does not.
				if got := tier.style.GetBold(); got != tier.emphasized {
					t.Fatalf("%s due badge bold = %v, want %v", tier.name, got, tier.emphasized)
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

// The emphasized tiers are the ones a glance must not miss, so their ink is held
// to the enhanced ratio rather than to the ordinary one, and the quiet tier is
// held below both: a chip that out-contrasted an arrived deadline would make
// "later" the loudest thing on the row.
func TestDueBadgeInkContrastIsTieredInBothThemes(t *testing.T) {
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	const (
		emphasizedMinimum = 7.0
		quietMinimum      = 4.5
	)

	for _, themeName := range []string{"dark", "light"} {
		t.Run(themeName, func(t *testing.T) {
			ApplyTheme(ThemeConfig{Name: themeName})

			ratios := map[string]float64{}
			for _, tier := range dueBadgeTiers() {
				ratio := dueBadgeContrast(tier.style.GetForeground(), tier.style.GetBackground())
				ratios[tier.name] = ratio
				minimum := quietMinimum
				if tier.emphasized {
					minimum = emphasizedMinimum
				}
				if ratio < minimum {
					t.Errorf("%s due badge contrast = %.2f, want >= %.1f", tier.name, ratio, minimum)
				}
			}
			for _, emphasized := range []string{"overdue", "today"} {
				if ratios["upcoming"] >= ratios[emphasized] {
					t.Errorf("upcoming due badge contrast %.2f is not quieter than %s's %.2f",
						ratios["upcoming"], emphasized, ratios[emphasized])
				}
			}
		})
	}
}

// Which way emphasis points depends on the paper: an emphasized chip takes the
// loudest ink the theme allows, which is lighter than its fill on the dark theme
// and darker than its fill on the light one. Asserting the direction rather than
// the hex is what keeps a future repaint from quietly inverting a tier.
func TestEmphasizedDueBadgeInkFollowsThePaper(t *testing.T) {
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	for _, variant := range []struct {
		themeName    string
		inkIsLighter bool
	}{
		{"dark", true},
		{"light", false},
	} {
		t.Run(variant.themeName, func(t *testing.T) {
			ApplyTheme(ThemeConfig{Name: variant.themeName})

			for _, tier := range dueBadgeTiers() {
				if !tier.emphasized {
					continue
				}
				ink := projectColorLuminance(dueBadgeRGB(tier.style.GetForeground()))
				fill := projectColorLuminance(dueBadgeRGB(tier.style.GetBackground()))
				if got := ink > fill; got != variant.inkIsLighter {
					t.Errorf("%s due badge ink lighter than its fill = %v, want %v (ink %.4f, fill %.4f)",
						tier.name, got, variant.inkIsLighter, ink, fill)
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

// dueBadgeRGB reads a resolved chip color back as 0-255 channels. It goes
// through color.Color's own RGBA rather than through the hex the constructor
// spelled, so the assertions above measure what a terminal is handed.
func dueBadgeRGB(value color.Color) [3]float64 {
	red, green, blue, _ := value.RGBA()
	return [3]float64{float64(red >> 8), float64(green >> 8), float64(blue >> 8)}
}

// dueBadgeContrast is the WCAG ratio between one chip's ink and its own fill,
// over the same luminance the project-color palette is held to.
func dueBadgeContrast(foreground, background color.Color) float64 {
	first := projectColorLuminance(dueBadgeRGB(foreground))
	second := projectColorLuminance(dueBadgeRGB(background))
	if first < second {
		first, second = second, first
	}
	return (first + 0.05) / (second + 0.05)
}

// The late-review chip is amber, so it is held apart from every amber the row
// already speaks: the P2 title ink, the Waiting section fill, and the due chips
// it sits beside. It is emphasized, so its ink meets the enhanced ratio.
func TestReviewLateBadgeIsItsOwnAmber(t *testing.T) {
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	for _, themeName := range []string{"dark", "light"} {
		t.Run(themeName, func(t *testing.T) {
			ApplyTheme(ThemeConfig{Name: themeName})
			fill := ReviewLateStyle.GetBackground()
			if fill == (lipgloss.NoColor{}) || !ReviewLateStyle.GetBold() {
				t.Fatal("late-review badge lost its fill or its emphasis")
			}
			if fill == TitlePriority2Style.GetForeground() {
				t.Fatalf("late-review fill %v is the P2 title ink", fill)
			}
			if fill == ViewModeWaitingStyle.GetBackground() {
				t.Fatalf("late-review fill %v is the Waiting section fill", fill)
			}
			for _, tier := range dueBadgeTiers() {
				if fill == tier.style.GetBackground() {
					t.Fatalf("late-review fill %v is the %s due fill", fill, tier.name)
				}
			}
			if ratio := dueBadgeContrast(ReviewLateStyle.GetForeground(), fill); ratio < 7.0 {
				t.Fatalf("late-review ink contrast = %.2f, want at least 7", ratio)
			}
		})
	}
}
