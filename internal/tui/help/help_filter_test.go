package help

import (
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/operation"
)

func TestHelpEmptyCategoryNamesMatchesAvailableInAll(t *testing.T) {
	for _, query := range []string{"cursor", "CURSOR", "jump"} {
		m := NewHelpViewModel([]operation.HelpItem{
			{Key: "e", Description: "Edit task", Category: "Tasks"},
			{Key: "gg", Description: "Cursor to start", Category: "Navigation", Aliases: []string{"jump"}},
			{Key: "G", Description: "Cursor to end", Category: "Navigation", Aliases: []string{"jump"}},
		}, "Outliner")
		m.SetSize(60, 20)
		m.activeTab = slices.Index(m.categories, "Tasks") + 1
		m.searchQuery = query
		m.filterItems()
		if len(m.filteredItems) != 0 || m.otherCategoryMatches != 2 {
			t.Fatalf("%q: visible = %v, elsewhere = %d", query, m.filteredItems, m.otherCategoryMatches)
		}
		rendered := ansi.Strip(m.renderContent())
		if !strings.Contains(rendered, "No matches in Tasks; 2 matches in other categories.") || !strings.Contains(rendered, "Choose the All tab") {
			t.Fatalf("missing category guidance:\n%s", rendered)
		}
		for _, line := range strings.Split(rendered, "\n") {
			if lipgloss.Width(line) > m.viewport.Width() {
				t.Fatalf("empty guidance overflows viewport: %q", line)
			}
		}
		m.activeTab = 0
		m.filterItems()
		if len(m.filteredItems) != 2 || m.otherCategoryMatches != 0 || m.searchQuery != query {
			t.Fatalf("All did not reveal the promised matches: %+v", m.filteredItems)
		}
	}
}

func TestHelpOtherCategoryCountClearsWhenSearchChanges(t *testing.T) {
	m := helpSmartCaseModel()
	m.SetSize(60, 20)
	m.activeTab = slices.Index(m.categories, "Actions") + 1
	m.searchQuery = "cursor"
	m.filterItems()
	if got := ansi.Strip(m.renderContent()); !strings.Contains(got, "1 match in other categories") {
		t.Fatalf("singular guidance = %q", got)
	}
	for _, query := range []string{"remove", "zebra", ""} {
		m.searchQuery = query
		m.filterItems()
		if m.otherCategoryMatches != 0 || strings.Contains(m.renderContent(), "Choose the All tab") {
			t.Fatalf("%q left a stale other-category notice", query)
		}
	}
}

func helpSmartCaseModel() *HelpViewModel {
	return NewHelpViewModel([]operation.HelpItem{
		{Key: "P", Description: "Pin task", Category: "Actions"},
		{Key: "p", Description: "insert below", Category: "Actions"},
		{Key: "x", Description: "remove task", Category: "Actions"},
		{Key: "gg", Description: "Cursor to start", Category: "Navigation"},
	}, "Outliner")
}

func helpFilteredKeys(m *HelpViewModel) []string {
	keys := make([]string, len(m.filteredItems))
	for i, item := range m.filteredItems {
		keys[i] = item.Key
	}
	return keys
}

func TestHelpFilterSmartCaseNarrowsThenFallsBack(t *testing.T) {
	m := helpSmartCaseModel()

	m.searchQuery = "p"
	m.filterItems()
	if got := helpFilteredKeys(m); !slices.Equal(got, []string{"P", "p"}) {
		t.Fatalf("lowercase filter = %v, want both casings", got)
	}
	if m.searchCaseRelaxed {
		t.Fatal("lowercase filter reported a relaxed pass")
	}

	m.searchQuery = "P"
	m.filterItems()
	if got := helpFilteredKeys(m); !slices.Equal(got, []string{"P"}) {
		t.Fatalf("uppercase filter = %v, want only the uppercase key", got)
	}
	if m.searchCaseRelaxed {
		t.Fatal("narrowing filter reported a relaxed pass")
	}

	m.searchQuery = "Remove"
	m.filterItems()
	if got := helpFilteredKeys(m); !slices.Equal(got, []string{"x"}) {
		t.Fatalf("fallback filter = %v, want the case-insensitive retry to match", got)
	}
	if !m.searchCaseRelaxed {
		t.Fatal("fallback filter did not record the relaxed pass")
	}

	m.searchQuery = "Zebra"
	m.filterItems()
	if got := helpFilteredKeys(m); len(got) != 0 {
		t.Fatalf("unmatchable filter = %v, want no items", got)
	}
	if m.searchCaseRelaxed {
		t.Fatal("an empty insensitive retry still claimed the case was ignored")
	}

	m.searchQuery = ""
	m.filterItems()
	if m.searchCaseRelaxed {
		t.Fatal("clearing the query left the relaxed flag set")
	}
}

func TestHelpTabSearchFallsBackToCaseInsensitiveMatch(t *testing.T) {
	m := helpSmartCaseModel()
	navigationTab := slices.Index(m.categories, "Navigation") + 1
	if navigationTab == 0 {
		t.Fatalf("categories = %v, want a Navigation section", m.categories)
	}

	m.matchTab("Navigation")
	if m.activeTab != navigationTab {
		t.Fatalf("exact-case tab = %d, want %d", m.activeTab, navigationTab)
	}

	m.matchTab("NAVIGATION")
	if m.activeTab != navigationTab {
		t.Fatalf("relaxed tab = %d, want the case-insensitive retry to land on %d", m.activeTab, navigationTab)
	}

	m.matchTab("Zebra")
	if m.activeTab != 0 {
		t.Fatalf("unmatchable tab = %d, want the All tab", m.activeTab)
	}
}

func TestHelpCategoryCountsPredictEachTabIncludingCaseFallback(t *testing.T) {
	m := NewHelpViewModel([]operation.HelpItem{
		{Key: "e", Description: "Edit API task", Category: "Tasks"},
		{Key: "g", Description: "Navigate api", Category: "Navigation"},
		{Key: "n", Description: "Open notes", Category: "Notes"},
	}, "Outliner")
	m.searchQuery = "API"
	m.filterItems()
	counts := slices.Clone(m.tabMatchCounts)
	for tab, count := range counts {
		m.activeTab = tab
		m.filterItems()
		if len(m.filteredItems) != count {
			t.Fatalf("tab %d promised %d matches, found %d", tab, count, len(m.filteredItems))
		}
	}
	m.activeTab = slices.Index(m.categories, "Tasks") + 1
	m.filterItems()
	m.SetSize(60, 20)
	bar := ansi.Strip(m.renderHelpTabBar())
	for _, label := range []string{"All (1)", "Tasks (1)", "Navigation (1)", "Notes (0)"} {
		if !strings.Contains(bar, label) {
			t.Fatalf("missing %q in %s", label, bar)
		}
	}
	if m.searchQuery != "API" || len(m.filteredItems) != 1 || m.filteredItems[0].Key != "e" {
		t.Fatal("counts changed the active query or category")
	}
	m.searchQuery = ""
	m.filterItems()
	if strings.Contains(m.renderHelpTabBar(), "(") {
		t.Fatal("clearing search retained match counts")
	}
}

func TestHelpSearchTermsCanSpanFieldsButNotEntries(t *testing.T) {
	items := []operation.HelpItem{
		{Key: "e", Description: "Modify title", Category: "Tasks", Aliases: []string{"rename", "edit"}},
		{Key: "x", Description: "Remove entry", Category: "Notes", Aliases: []string{"delete"}},
	}
	for _, query := range []string{"tasks modify", "e title", "tasks rename", "rename edit", "TASKS MODIFY"} {
		matches, relaxed := matchHelpItems(items, query)
		if len(matches) != 1 || matches[0].Key != "e" {
			t.Fatalf("%q: %+v", query, matches)
		}
		if relaxed != (query == "TASKS MODIFY") {
			t.Fatalf("%q: relaxed=%v", query, relaxed)
		}
	}
	if matches, _ := matchHelpItems(items, "tasks delete"); len(matches) != 0 {
		t.Fatalf("matched across entries: %+v", matches)
	}
	matches, _ := matchHelpItems(items, "e title title")
	if !slices.Equal(matches[0].keyPositions, []int{0}) || !slices.Equal(matches[0].descPositions, []int{7, 8, 9, 10, 11}) {
		t.Fatalf("field-relative highlights: %+v", matches[0])
	}
	matches, _ = matchHelpItems(items, "tasks rename edit")
	if !slices.Equal(matches[0].matchedAliases, []string{"rename", "edit"}) {
		t.Fatalf("aliases: %+v", matches[0])
	}
}
