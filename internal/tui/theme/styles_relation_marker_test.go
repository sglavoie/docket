package theme

import (
	"math"
	"testing"

	"charm.land/lipgloss/v2"
)

// The relation marker is a chip so it reads as a relation at a glance, which
// holds only while its fill is a hue nothing else on the row is filled with:
// no section badge, no due tier, no late review. It is emphasized, so its ink
// meets the enhanced ratio the arrived deadlines do.
func TestRelationMarkerIsItsOwnBoldChip(t *testing.T) {
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	const minimumHueGap = 26.0
	for _, themeName := range []string{"dark", "light"} {
		t.Run(themeName, func(t *testing.T) {
			ApplyTheme(ThemeConfig{Name: themeName})
			chip := RelationMarkerStyle
			if !chip.GetBold() || chip.GetBackground() == nil {
				t.Fatal("relation marker is not a bold filled chip")
			}
			if ratio := dueBadgeContrast(chip.GetForeground(), chip.GetBackground()); ratio < 7.0 {
				t.Fatalf("relation marker ink contrast %.2f, want at least 7", ratio)
			}
			hue := projectColorHue(dueBadgeRGB(chip.GetBackground()))
			others := map[string]lipgloss.Style{
				"active": ViewModeActiveStyle, "work": ViewModeWorkStyle,
				"next": ViewModeNextStyle, "inbox": ViewModeInboxStyle,
				"waiting": ViewModeWaitingStyle, "someday": ViewModeSomedayStyle,
				"completed": ViewModeCompletedStyle, "standby": ViewModeStandbyStyle,
				"deferred": ViewModeDeferredStyle, "canceled": ViewModeCanceledStyle,
				"all": ViewModeAllStyle, "review": ViewModeReviewStyle,
				"overdue": OverdueStyle, "today": DueTodayStyle,
				"upcoming": DueDateStyle, "review-late": ReviewLateStyle,
			}
			for name, other := range others {
				if other.GetBackground() == nil {
					continue
				}
				gap := math.Abs(hue - projectColorHue(dueBadgeRGB(other.GetBackground())))
				gap = math.Min(gap, 360-gap)
				if gap < minimumHueGap {
					t.Fatalf("relation marker fill sits %.0f° from the %s fill, want at least %.0f°", gap, name, minimumHueGap)
				}
			}
		})
	}
}
