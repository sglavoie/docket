package outliner

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/chrome"
)

func TestSearchHeaderExplainsItsScopeFromFirstKeystroke(t *testing.T) {
	out := matchCountOutliner()
	out.PrimaryMode = chrome.ModeToday
	out.ActivateSearch()
	for _, query := range []string{"", "needle", "needle ?completed", "needle ?deferred"} {
		out.SetSearchInputValue(query)
		out.SetSearchQuery(query)
		for _, width := range []int{60, 80, 120} {
			title := ansi.Strip(out.BuildOutlinerTitleLine(width))
			if !strings.Contains(title, "Search · all modes") || strings.Contains(title, "Today") || lipgloss.Width(title) > width {
				t.Fatalf("%d-column search title misstates its scope: %q", width, title)
			}
		}
		context := ansi.Strip(out.BuildOutlinerContextRow(160))
		want := "Open tasks · not deferred"
		if strings.Contains(query, "?completed") {
			want = "Includes completed · not deferred"
		} else if strings.Contains(query, "?deferred") {
			want = "Open tasks · includes deferred"
		}
		if !strings.Contains(context, want) {
			t.Fatalf("query %q context = %q, want %q", query, context, want)
		}
	}
	out.ToggleRankedList()
	if title := ansi.Strip(out.BuildOutlinerTitleLine(80)); !strings.Contains(title, "Today") || strings.Contains(title, "all modes") {
		t.Fatalf("mode-narrowing title = %q", title)
	}
}
