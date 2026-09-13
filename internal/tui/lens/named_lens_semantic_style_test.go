package lens

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
		{"stored review", namedLensQueryFixture("=review")},
		{"review attention", namedLensQueryFixture("?review")},
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
					style, ok := NamedLensPrimaryStyle(tc.lens)
					if !ok {
						t.Fatalf("no style for a definition with a semantic role")
					}
					assertForegroundOnlyStyle(t, style)
				})
			}

			for _, tc := range neutral {
				t.Run("neutral "+tc.name, func(t *testing.T) {
					if style, ok := NamedLensPrimaryStyle(tc.lens); ok {
						t.Fatalf("neutral definition wears %v", style.GetForeground())
					}
				})
			}
		})
	}

	t.Run("accents follow the theme", func(t *testing.T) {
		lens := namedLensQueryFixture("@overdue")

		theme.ApplyTheme(theme.ThemeConfig{Name: "dark"})
		dark, ok := NamedLensPrimaryStyle(lens)
		if !ok {
			t.Fatalf("dark theme resolved no style")
		}
		theme.ApplyTheme(theme.ThemeConfig{Name: "light"})
		light, ok := NamedLensPrimaryStyle(lens)
		if !ok {
			t.Fatalf("light theme resolved no style")
		}
		if dark.GetForeground() == light.GetForeground() {
			t.Errorf("overdue accent is %v in both themes", dark.GetForeground())
		}
	})

	// A priority definition borrows the priority title color rather than owning an
	// accent of its own, so the accent has to be whatever GetTitlePriorityStyle
	// currently answers — under either theme, and never a hex captured here. That
	// the accents themselves read apart is theme's own invariant, asserted by
	// TestLensAccentsReadApart beside the palette that holds them.
	t.Run("priority accents are the priority title colors", func(t *testing.T) {
		for _, themeName := range []string{"dark", "light"} {
			theme.ApplyTheme(theme.ThemeConfig{Name: themeName})

			for _, priority := range []int{models.PriorityHigh, models.PriorityMedium, models.PriorityLow} {
				style, ok := NamedLensPrimaryStyle(
					namedLensQueryFixture(fmt.Sprintf("!%d", priority)))
				if !ok {
					t.Fatalf("%s theme: priority %d definition resolved no style", themeName, priority)
				}
				if got, want := style.GetForeground(), theme.GetTitlePriorityStyle(priority).GetForeground(); got != want {
					t.Errorf("%s theme: priority %d accent = %v, want the priority title color %v", themeName, priority, got, want)
				}
				assertForegroundOnlyStyle(t, style)
			}
		}
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
