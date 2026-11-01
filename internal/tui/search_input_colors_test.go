package tui

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/control"
	"github.com/sglavoie/docket/internal/tui/theme"
)

func TestSearchTextStaysVisibleWhenHostPaletteReplacesItsOldForeground(t *testing.T) {
	driver, out := menuBarDriver(t)
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })
	pressMenuBarKey(t, driver, "/")
	if _, err := driver.ApplyAction(UITestAction{Type: "text", Text: "needle"}, ObserveOptions{}); err != nil {
		t.Fatal(err)
	}
	oldForeground := theme.ColorNormalText
	// A host palette arrives after the long-lived Outliner was constructed.
	// Its paper exactly matches the input's cached ink, reproducing invisible
	// characters that remain present when selected or copied in the terminal.
	palette := theme.GhosttyPalette{
		Name: "Search contrast regression", Background: oldForeground, Foreground: "#111827",
		Cursor: "#111827", CursorText: oldForeground,
		SelectionBackground: "#111827", SelectionForeground: oldForeground,
		Palette: []string{"#111827", "#9a1529", "#286536", "#856018", "#244d9a", "#753785", "#086778", "#d6d0c2",
			"#554d60", "#b93747", "#408045", "#a3782a", "#4770b1", "#98559e", "#34808e", "#fffaf0"},
	}
	data, err := json.Marshal(palette)
	if err != nil {
		t.Fatal(err)
	}
	replies := make(chan control.Reply, 1)
	driver.App().controlThemePalette(string(data), replies)
	if reply := <-replies; !reply.OK {
		t.Fatalf("host palette: %+v", reply)
	}
	want := theme.ThemedTextInputStyles.Focused.Text.Inline(true).Render("needle")
	if got := out.SearchInputView(); !strings.Contains(got, want) {
		t.Fatalf("input paints its text in the host's background color:\ngot %q\nwant %q", got, want)
	}
	if got := driver.App().View().Content; !strings.Contains(got, want) {
		t.Fatalf("visible search header lost the current input color:\n%q", got)
	}
	if out.SearchInputValue() != "needle" || !out.IsSearchMode() {
		t.Fatal("palette refresh changed the search input")
	}
	assertHelpScreen(t, driver, "needle")
}
