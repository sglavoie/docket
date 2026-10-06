package outliner

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestSearchHistoryPickerFindsNonPrefixAndReturnsEditableQuery(t *testing.T) {
	m := searchHistoryOutliner()
	saves := 0
	m.ConfigureSearchHistory([]string{"$waiting #client", "$doing #client", "Other"}, func([]string) error { saves++; return nil })
	m.ActivateSearch()
	m.SetSearchInputValue("client")
	m.SetSearchInputCursor(2)
	m.HandleKeyMsg(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	if m.Search.historyPicker == nil {
		t.Fatal("history picker did not open")
	}
	if got := len(m.Search.historyPicker.choices.Items); got != 2 {
		t.Fatalf("matches = %d", got)
	}
	if view := ansi.Strip(m.renderSearchHistoryPicker(60)); !strings.Contains(view, "$waiting #client") {
		t.Fatal(view)
	}
	m.HandleKeyMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	m.HandleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.Search.historyPicker != nil || !m.IsSearchMode() || m.SearchInputValue() != "$waiting #client" || saves != 0 {
		t.Fatalf("accept submitted or lost query: %q, saves %d", m.SearchInputValue(), saves)
	}
	m.HandleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	if saves != 1 || m.IsSearchMode() {
		t.Fatal("explicit submit did not record query")
	}
}

func TestSearchHistoryPickerCancelPreservesDraftAndHandlesEmptyResults(t *testing.T) {
	m := searchHistoryOutliner()
	m.ConfigureSearchHistory([]string{"$waiting #CLIENT", "café"}, nil)
	m.ActivateSearch()
	m.SetSearchInputValue("client")
	m.SetSearchInputCursor(2)
	originalQuery := m.SearchQuery()
	m.HandleKeyMsg(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	if len(m.Search.historyPicker.choices.Items) != 1 {
		t.Fatal("case-insensitive match missing")
	}
	m.Update(tea.PasteMsg{Content: " missing"})
	if view := m.renderSearchHistoryPicker(60); !strings.Contains(view, "No history matches") {
		t.Fatal(view)
	}
	m.HandleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.Search.historyPicker == nil {
		t.Fatal("accepted empty match set")
	}
	m.HandleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.SearchInputValue() != "client" || m.SearchInputCursor() != 2 || m.SearchQuery() != originalQuery {
		t.Fatal("cancel changed draft, caret, or projection")
	}
	if err := m.ClearSearchHistory(); err != nil {
		t.Fatal(err)
	}
	m.HandleKeyMsg(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	if view := m.renderSearchHistoryPicker(60); !strings.Contains(view, "No search history yet") {
		t.Fatal(view)
	}
}
