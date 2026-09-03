package tui

import (
	"fmt"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// This file covers the style a role reaches for: that every role wears one, that
// a neutral definition wears none, that the accents follow the theme and the
// palette, and that no two of them read alike. Which role a definition earns is
// in named_lens_semantic_role_test.go and the fixtures are in
// named_lens_semantic_color_test.go.

func TestNamedLensPrimaryStyle(t *testing.T) {
	// The theme is global mutable state, so these cases run in sequence and hand
	// the package back its default dark palette.
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	styled := []struct {
		name string
		lens config.NamedLens
	}{
		{"overdue", namedLensQueryFixture("@overdue")},
		{"waiting", namedLensStatesFixture(string(models.StateWaiting))},
		{"due today", namedLensQueryFixture("@today")},
		{"priority 1", namedLensQueryFixture(fmt.Sprintf("!%d", models.PriorityHigh))},
		{"priority 2", namedLensQueryFixture(fmt.Sprintf("!%d", models.PriorityMedium))},
		{"priority 3", namedLensQueryFixture(fmt.Sprintf("!%d", models.PriorityLow))},
		{"due week", namedLensQueryFixture("@week")},
		{"deferred", namedLensQueryFixture("?deferred")},
		{"review", namedLensQueryFixture("=review")},
		{"completed", namedLensStatesFixture(string(models.StateDone), string(models.StateCanceled))},
		{"inbox group", namedLensStatesFixture(string(models.StateInbox))},
		{"doing group", namedLensStatesFixture(string(models.StateDoing))},
		{"backlog group", namedLensStatesFixture(string(models.StateBacklog))},
		{"paused group", namedLensStatesFixture(string(models.StatePaused))},
		{"someday group", namedLensStatesFixture(string(models.StateSomeday))},
		{"warning", namedLensQueryFixture("?stale")},
		{"today mode", namedLensFixture(func(l *config.NamedLens) { l.Mode = config.LensModeToday })},
		{"recent mode", namedLensFixture(func(l *config.NamedLens) { l.Mode = config.LensModeRecent })},
	}

	neutral := []struct {
		name string
		lens config.NamedLens
	}{
		{"zero value", config.NamedLens{}},
		{"all mode only", namedLensFixture(nil)},
		{"descriptive tags", namedLensQueryFixture("#ops")},
		{"unknown mode", namedLensFixture(func(l *config.NamedLens) { l.Mode = "dashboard" })},
		{"unknown state", namedLensStatesFixture("almost-done")},
	}

	for _, themeName := range []string{"dark", "light"} {
		t.Run(themeName, func(t *testing.T) {
			theme.ApplyTheme(theme.ThemeConfig{Name: themeName})

			for _, tc := range styled {
				t.Run(tc.name, func(t *testing.T) {
					style, ok := namedLensPrimaryStyle(tc.lens)
					if !ok {
						t.Fatalf("no style for a definition with a semantic role")
					}
					assertForegroundOnlyStyle(t, style)
				})
			}

			for _, tc := range neutral {
				t.Run("neutral "+tc.name, func(t *testing.T) {
					if style, ok := namedLensPrimaryStyle(tc.lens); ok {
						t.Fatalf("neutral definition wears %v", style.GetForeground())
					}
				})
			}
		})
	}

	t.Run("accents follow the theme", func(t *testing.T) {
		lens := namedLensQueryFixture("@overdue")

		theme.ApplyTheme(theme.ThemeConfig{Name: "dark"})
		dark, ok := namedLensPrimaryStyle(lens)
		if !ok {
			t.Fatalf("dark theme resolved no style")
		}
		theme.ApplyTheme(theme.ThemeConfig{Name: "light"})
		light, ok := namedLensPrimaryStyle(lens)
		if !ok {
			t.Fatalf("light theme resolved no style")
		}
		if dark.GetForeground() == light.GetForeground() {
			t.Errorf("overdue accent is %v in both themes", dark.GetForeground())
		}
	})

	// A mode borrows its group's accent and Today shares Backlog's the way their
	// badges already do, so the accents themselves — not the definitions that
	// reach them — are what must read apart.
	t.Run("accents read apart", func(t *testing.T) {
		for _, themeName := range []string{"dark", "light"} {
			theme.ApplyTheme(theme.ThemeConfig{Name: themeName})

			accents := map[string]lipgloss.Style{
				"plan":      theme.LensAccentPlanStyle,
				"work":      theme.LensAccentWorkStyle,
				"inbox":     theme.LensAccentInboxStyle,
				"next":      theme.LensAccentNextStyle,
				"waiting":   theme.LensAccentWaitingStyle,
				"paused":    theme.LensAccentPausedStyle,
				"someday":   theme.LensAccentSomedayStyle,
				"complete":  theme.LensAccentCompleteStyle,
				"canceled":  theme.LensAccentCanceledStyle,
				"overdue":   theme.LensAccentOverdueStyle,
				"due today": theme.LensAccentDueTodayStyle,
				"due":       theme.LensAccentDueStyle,
				"deferred":  theme.LensAccentDeferredStyle,
				"review":    theme.LensAccentReviewStyle,
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
				title := theme.GetTitlePriorityStyle(priority).GetForeground()
				for name, style := range accents {
					if style.GetForeground() == title {
						t.Errorf("%s theme: %s accent equals the priority %d title color", themeName, name, priority)
					}
				}
			}
		}
	})

	t.Run("priority accents follow the palette", func(t *testing.T) {
		theme.ApplyTheme(theme.ThemeConfig{Name: "dark"})
		theme.RebuildPalette(config.PaletteColors{Priority1Title: "#123456"})
		theme.RebuildPriorityTitleStyles()

		style, ok := namedLensPrimaryStyle(
			namedLensQueryFixture(fmt.Sprintf("!%d", models.PriorityHigh)))
		if !ok {
			t.Fatalf("priority definition resolved no style")
		}
		if got := style.GetForeground(); got != lipgloss.Color("#123456") {
			t.Errorf("priority 1 accent = %v, want the overridden #123456", got)
		}
		assertForegroundOnlyStyle(t, style)
	})
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
