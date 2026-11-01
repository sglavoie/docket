package listsurface

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/tui/theme"
)

func TestSearchInputTextTracksTheActiveTheme(t *testing.T) {
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })
	for _, initial := range []string{"light", "dark"} {
		t.Run(initial, func(t *testing.T) {
			theme.ApplyTheme(theme.ThemeConfig{Name: initial})
			var view BaseSearchableView
			view.InitSearchInput("Search…", 100, 30)
			view.ActivateSearch()
			view.SetSearchInputValue("needle")
			view.SetSearchQuery("needle")
			next := "dark"
			if initial == "dark" {
				next = "light"
			}
			theme.ApplyTheme(theme.ThemeConfig{Name: next})
			before := view.CaptureSearchInput()
			caret := view.SearchInputCursor()
			for _, focused := range []bool{true, false} {
				if !focused {
					view.ConfirmSearch()
					before = view.CaptureSearchInput()
				}
				styles := theme.ThemedTextInputStyles.Focused
				if !focused {
					styles = theme.ThemedTextInputStyles.Blurred
				}
				want := styles.Text.Inline(true).Render("needle")
				got := view.SearchInputView()
				if !strings.Contains(got, want) {
					t.Errorf("%s input retained the %s colors after switching to %s:\ngot %q\nwant %q", map[bool]string{true: "focused", false: "blurred"}[focused], initial, next, got, want)
				}
				if !strings.Contains(ansi.Strip(got), "needle") || view.CaptureSearchInput() != before || view.SearchInputCursor() != caret {
					t.Fatal("render changed search text, focus, or caret")
				}
			}
		})
	}
}
