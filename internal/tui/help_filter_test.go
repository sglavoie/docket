package tui

import (
	"slices"
	"testing"
)

func helpSmartCaseModel() *HelpViewModel {
	return NewHelpViewModel([]HelpItem{
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
