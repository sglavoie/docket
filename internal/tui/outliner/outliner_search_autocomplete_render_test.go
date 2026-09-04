package outliner

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
)

// searchACWideOutliner builds a scope with enough tags to overflow the row
// budget in both directions.
func searchACWideOutliner(count int) *OutlinerModel {
	tasks := make([]*models.Task, 0, count)
	for i := range count {
		tasks = append(tasks, searchACTask(fmt.Sprintf("t%d", i), []string{fmt.Sprintf("tag%02d", i)}, ""))
	}
	return searchACOutliner(primaryRowsProject("alpha", tasks...))
}

// TestSearchAutocompleteRendersAtMostFiveRowsIncludingOverflow holds the pane to
// its total budget at the top, the middle, and the bottom of a long list, where
// the number of rows spent on hints differs.
func TestSearchAutocompleteRendersAtMostFiveRowsIncludingOverflow(t *testing.T) {
	outliner := searchACWideOutliner(12)
	typeSearchAC(outliner, "#")

	cases := []struct {
		name       string
		cursor     int
		wantTop    bool
		wantBottom bool
	}{
		{"top", 0, false, true},
		{"middle", 6, true, true},
		{"bottom", 11, true, false},
	}
	for _, tc := range cases {
		outliner.Search.Autocomplete.Ac.Cursor = tc.cursor
		outliner.Search.Autocomplete.Ac.ClampCursor()
		rendered := ansi.Strip(outliner.renderSearchAutocompleteDropdown(120))
		lines := strings.Split(rendered, "\n")
		if len(lines) > outlinerSearchSuggestionLimit {
			t.Errorf("%s: rendered %d rows, want at most %d", tc.name, len(lines), outlinerSearchSuggestionLimit)
		}
		if got := strings.Contains(rendered, "↑"); got != tc.wantTop {
			t.Errorf("%s: top overflow hint = %v, want %v:\n%s", tc.name, got, tc.wantTop, rendered)
		}
		if got := strings.Contains(rendered, "↓"); got != tc.wantBottom {
			t.Errorf("%s: bottom overflow hint = %v, want %v:\n%s", tc.name, got, tc.wantBottom, rendered)
		}
		if !strings.Contains(rendered, outliner.Search.Autocomplete.Ac.Selected().Display) {
			t.Errorf("%s: the selected row scrolled out of its own window:\n%s", tc.name, rendered)
		}
	}
}

// TestSearchAutocompleteViewportSubtractsTheRenderedHeight is the layout
// invariant: the rows the pane occupies are taken from the list, so the cursor
// row and the footer stay on screen.
func TestSearchAutocompleteViewportSubtractsTheRenderedHeight(t *testing.T) {
	outliner := searchACWideOutliner(12)
	outliner.SetSize(120, 24)

	outliner.RenderOutlinerTreeContent(120)
	baseline := outliner.LastViewport.VisibleRows

	typeSearchAC(outliner, "#")
	if !outliner.Search.Autocomplete.Active() {
		t.Fatal("no pane to reserve for")
	}
	content := outliner.RenderOutlinerTreeContent(120)
	withPane := outliner.LastViewport.VisibleRows

	// Measure the pane exactly as the render did: whatever height it drew is the
	// height the list lost.
	dropdownHeight := lipgloss.Height(outliner.Search.Autocomplete.Ac.RenderDropdownWithLimit(
		outlinerSearchDropdownIndent, outlinerSearchSuggestionLimit))
	if got, want := baseline-withPane, dropdownHeight; got != want {
		t.Fatalf("rows reserved = %d, want the pane's own height %d", got, want)
	}
	if withPane < 1 {
		t.Fatalf("viewport collapsed to %d rows", withPane)
	}
	// The whole frame must still fit the terminal it was sized to.
	if got := lipgloss.Height(content); got > outliner.ContentHeight()+1 {
		t.Errorf("frame height = %d, want at most %d", got, outliner.ContentHeight()+1)
	}
}

// TestSearchAutocompleteRenderFitsNarrowAndMinimumLayouts covers the two ends of
// the size range: a width the rows must be cut to, and a height with room for a
// single row and no hint at all.
func TestSearchAutocompleteRenderFitsNarrowAndMinimumLayouts(t *testing.T) {
	outliner := searchACWideOutliner(12)

	outliner.SetSize(24, 40)
	typeSearchAC(outliner, "#")
	rendered := ansi.Strip(outliner.renderSearchAutocompleteDropdown(14))
	for _, line := range strings.Split(rendered, "\n") {
		if got := lipgloss.Width(line); got > 14 {
			t.Errorf("row %q is %d cells, want at most 14", line, got)
		}
	}

	// contentHeight()-2 is the pane's ceiling, so a three-row terminal leaves room
	// for exactly one row and it must be the selected one rather than a hint.
	outliner.SetSize(120, 4)
	outliner.Search.Autocomplete.Ac.Cursor = 6
	outliner.Search.Autocomplete.Ac.ClampCursor()
	minimum := ansi.Strip(outliner.renderSearchAutocompleteDropdown(120))
	lines := strings.Split(minimum, "\n")
	if len(lines) != 1 {
		t.Fatalf("minimum-height pane rendered %d rows, want 1:\n%s", len(lines), minimum)
	}
	if !strings.Contains(minimum, outliner.Search.Autocomplete.Ac.Selected().Display) {
		t.Errorf("minimum-height pane dropped the selection for a hint: %q", minimum)
	}

	// Even at the smallest usable terminal the list keeps a row.
	outliner.SetSize(40, 3)
	outliner.RenderOutlinerTreeContent(40)
	if got := outliner.LastViewport.VisibleRows; got < 1 {
		t.Errorf("minimum-height viewport = %d rows, want at least 1", got)
	}
}

// TestSearchAutocompleteGuidanceSwapsOnlyWhileTheDropdownIsOpen keeps the footer
// honest about which Esc the next press is.
func TestSearchAutocompleteGuidanceSwapsOnlyWhileTheDropdownIsOpen(t *testing.T) {
	task := searchACTask("t1", []string{"infra"}, "")
	outliner := searchACOutliner(primaryRowsProject("alpha", task))

	typeSearchAC(outliner, "auth")
	spec, ok := outliner.ContextualGuidance()
	if !ok {
		t.Fatal("search mode published no guidance")
	}
	if !strings.Contains(searchACGuidanceText(spec), "Esc clear search") {
		t.Errorf("plain search guidance = %q, want the ordinary search hints", searchACGuidanceText(spec))
	}

	typeSearchAC(outliner, "#")
	spec, ok = outliner.ContextualGuidance()
	if !ok {
		t.Fatal("an open pane published no guidance")
	}
	text := searchACGuidanceText(spec)
	for _, want := range []string{"Tab accept", "Esc close suggestions"} {
		if !strings.Contains(text, want) {
			t.Errorf("dropdown guidance = %q, want it to name %q", text, want)
		}
	}
	if strings.Contains(text, "Esc clear search") {
		t.Errorf("dropdown guidance = %q, want it to stop promising Esc clears the search", text)
	}
	if !strings.Contains(text, "Enter apply search") {
		t.Errorf("dropdown guidance = %q, want the search's own key still named", text)
	}
}

func searchACGuidanceText(spec chrome.GuidanceSpec) string {
	parts := make([]string, 0, len(spec.Hints))
	for _, hint := range spec.Hints {
		parts = append(parts, chrome.GuidanceHintText(hint))
	}
	return strings.Join(parts, " | ")
}
