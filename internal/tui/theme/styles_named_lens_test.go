package theme

import (
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// The lens accents are a palette, so what has to hold of them is a palette
// property: each is a color and nothing else, no two read alike, and none lands
// on a priority title color. Which definition earns which accent is the lens
// layer's question and is asserted there, against namedLensPrimaryStyle.
//
// A mode borrows its group's accent and Today shares Backlog's the way their
// badges already do, so the accents themselves — not the definitions that reach
// them — are what must read apart.
func TestLensAccentsReadApart(t *testing.T) {
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	for _, themeName := range []string{"dark", "light"} {
		ApplyTheme(ThemeConfig{Name: themeName})

		accents := map[string]lipgloss.Style{
			"plan":      lensAccentPlanStyle,
			"work":      lensAccentWorkStyle,
			"inbox":     lensAccentInboxStyle,
			"next":      lensAccentNextStyle,
			"waiting":   lensAccentWaitingStyle,
			"paused":    lensAccentPausedStyle,
			"someday":   lensAccentSomedayStyle,
			"complete":  lensAccentCompleteStyle,
			"canceled":  lensAccentCanceledStyle,
			"overdue":   LensAccentOverdueStyle,
			"due today": LensAccentDueTodayStyle,
			"due":       LensAccentDueStyle,
			"deferred":  LensAccentDeferredStyle,
			"review":    LensAccentReviewStyle,
		}

		seen := make(map[string]string, len(accents))
		for name, style := range accents {
			assertForegroundOnlyStyle(t, style)
			key := lipgloss.NewStyle().Foreground(style.GetForeground()).Render("x")
			if previous, clash := seen[key]; clash {
				t.Errorf("%s theme: %s wears the same accent as %s", themeName, name, previous)
				continue
			}
			seen[key] = name
		}

		// An accent that landed exactly on a priority title color would make a
		// state lens and a priority lens indistinguishable.
		for _, priority := range []int{models.PriorityHigh, models.PriorityMedium, models.PriorityLow} {
			title := GetTitlePriorityStyle(priority).GetForeground()
			for name, style := range accents {
				if style.GetForeground() == title {
					t.Errorf("%s theme: %s accent equals the priority %d title color", themeName, name, priority)
				}
			}
		}
	}
}

// A lens that selects a priority wears that priority's title color, so the
// accent has to be read off the rebuilt style rather than captured once. An
// override that reaches GetTitlePriorityStyle is what makes that observable
// without a theme switch.
func TestPriorityTitleStylesFollowThePalette(t *testing.T) {
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	ApplyTheme(ThemeConfig{Name: "dark"})
	rebuildPalette(config.PaletteColors{Priority1Title: "#123456"})
	rebuildPriorityTitleStyles()

	if got := GetTitlePriorityStyle(models.PriorityHigh).GetForeground(); got != lipgloss.Color("#123456") {
		t.Errorf("priority 1 title color = %v, want the overridden #123456", got)
	}
}

// assertForegroundOnlyStyle holds a lens accent to the shared design contract: a
// color and nothing else, so the row's width, weight, and fill stay the list's.
func assertForegroundOnlyStyle(t *testing.T, style lipgloss.Style) {
	t.Helper()

	if _, unset := style.GetForeground().(lipgloss.NoColor); unset {
		t.Errorf("style carries no foreground")
	}
	if _, unset := style.GetBackground().(lipgloss.NoColor); !unset {
		t.Errorf("style carries background %v", style.GetBackground())
	}
	if style.GetBold() {
		t.Errorf("style is bold")
	}
	if style.GetItalic() {
		t.Errorf("style is italic")
	}
	if style.GetFaint() {
		t.Errorf("style is faint")
	}
	if style.GetUnderline() {
		t.Errorf("style is underlined")
	}
	if style.GetStrikethrough() {
		t.Errorf("style is struck through")
	}
	if style.GetWidth() != 0 {
		t.Errorf("style sets width %d", style.GetWidth())
	}
	top, right, bottom, left := style.GetPaddingTop(), style.GetPaddingRight(), style.GetPaddingBottom(), style.GetPaddingLeft()
	if top != 0 || right != 0 || bottom != 0 || left != 0 {
		t.Errorf("style pads %d/%d/%d/%d", top, right, bottom, left)
	}
}
