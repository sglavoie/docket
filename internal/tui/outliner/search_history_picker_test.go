package outliner

import (
	"errors"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/status"
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

func TestSearchHistoryForgetPersistsOnlySelectedQueryAndPreservesDraft(t *testing.T) {
	m := searchHistoryOutliner()
	entries := []string{"$waiting #client", "unrelated", "$doing #client"}
	var saved []string
	fail := true
	m.ConfigureSearchHistory(entries, func(next []string) error {
		if fail {
			return errors.New("disk unavailable")
		}
		saved = slices.Clone(next)
		return nil
	})
	m.ActivateSearch()
	m.SetSearchInputValue("client")
	m.SetSearchInputCursor(2)
	query := m.SearchQuery()
	m.HandleKeyMsg(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	m.HandleKeyMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	_, cmd := m.HandleKeyMsg(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("missing failure feedback")
	}
	if _, ok := cmd().(status.ErrorMsg); !ok {
		t.Fatal("failed persistence was not reported")
	}
	if !slices.Equal(m.Search.history.entries, entries) || m.Search.historyPicker.choices.Selected().Value != entries[0] {
		t.Fatal("failed save changed retained history or selection")
	}
	fail = false
	_, cmd = m.HandleKeyMsg(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("missing success feedback")
	}
	if _, ok := cmd().(status.InfoStatusMsg); !ok {
		t.Fatal("missing success status")
	}
	if !slices.Equal(saved, entries[1:]) || !slices.Equal(m.Search.history.entries, saved) {
		t.Fatalf("forgot wrong query: %v", saved)
	}
	if m.Search.historyPicker.input.Value() != "client" || len(m.Search.historyPicker.choices.Items) != 1 {
		t.Fatal("filter or remaining matches lost")
	}
	m.HandleKeyMsg(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl})
	if len(m.Search.historyPicker.choices.Items) != 0 {
		t.Fatal("last matching query survived")
	}
	_, cmd = m.HandleKeyMsg(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl})
	if cmd != nil || !slices.Equal(saved, []string{"unrelated"}) {
		t.Fatal("empty selection changed history")
	}
	m.HandleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.SearchInputValue() != "client" || m.SearchInputCursor() != 2 || m.SearchQuery() != query {
		t.Fatal("forgetting changed the original search")
	}
	reopened := searchHistoryOutliner()
	reopened.ConfigureSearchHistory(saved, nil)
	if !slices.Equal(reopened.Search.history.entries, []string{"unrelated"}) {
		t.Fatal("forgotten query returned")
	}
}
