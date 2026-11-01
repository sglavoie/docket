package outliner

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/tui/theme"
)

func TestSearchHistoryInputUsesTheCurrentPalette(t *testing.T) {
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })
	theme.ApplyTheme(theme.ThemeConfig{Name: "light"})
	m := scopeTestOutliner(scopeTestProject("alpha", "Alpha", "task"))
	m.ConfigureSearchHistory([]string{"needle"}, nil)
	m.ActivateSearch()
	m.SetSearchInputValue("needle")
	m.SetSearchQuery("needle")
	m.openSearchHistoryPicker()
	picker := m.Search.historyPicker
	caret := picker.input.Position()
	for _, name := range []string{"light", "dark", "light"} {
		theme.ApplyTheme(theme.ThemeConfig{Name: name})
		want := theme.ThemedTextInputStyles.Focused.Text.Inline(true).Render("needle")
		got := m.buildOutlinerSearchIndicator(100)
		if !strings.Contains(got, want) {
			t.Fatalf("history input does not use %s ink:\ngot %q\nwant %q", name, got, want)
		}
		if picker.input.Value() != "needle" || picker.input.Position() != caret || !picker.input.Focused() {
			t.Fatal("repainting history changed its text, caret, or focus")
		}
	}
}
