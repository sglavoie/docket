package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewSelectorModel(t *testing.T) {
	items := []SelectorItem{
		{ID: "1", Columns: []string{"Item 1", "Description 1"}, Headers: []string{"Name", "Description"}},
		{ID: "2", Columns: []string{"Item 2", "Description 2"}, Headers: []string{"Name", "Description"}},
	}

	m := NewSelectorModel("Test Selector", items)

	if m.title != "Test Selector" {
		t.Errorf("title = %q, want 'Test Selector'", m.title)
	}
	if len(m.items) != 2 {
		t.Errorf("items length = %d, want 2", len(m.items))
	}
	if len(m.filteredItems) != 2 {
		t.Errorf("filteredItems length = %d, want 2", len(m.filteredItems))
	}
	if m.done {
		t.Error("done should be false initially")
	}
	if m.canceled {
		t.Error("canceled should be false initially")
	}
	if m.searchMode {
		t.Error("searchMode should be false initially")
	}
	if !m.showHelp {
		t.Error("showHelp should be true initially")
	}
}

func TestNewSelectorModel_EmptyItems(t *testing.T) {
	m := NewSelectorModel("Empty Selector", []SelectorItem{})

	if len(m.items) != 0 {
		t.Errorf("items length = %d, want 0", len(m.items))
	}
	if len(m.filteredItems) != 0 {
		t.Errorf("filteredItems length = %d, want 0", len(m.filteredItems))
	}
}

func TestSelectorModel_FilterItems_EmptyQuery(t *testing.T) {
	items := []SelectorItem{
		{ID: "1", Columns: []string{"Apple"}},
		{ID: "2", Columns: []string{"Banana"}},
		{ID: "3", Columns: []string{"Cherry"}},
	}

	m := NewSelectorModel("Fruits", items)
	m.filterItems("")

	if len(m.filteredItems) != 3 {
		t.Errorf("filteredItems length = %d, want 3", len(m.filteredItems))
	}
}

func TestSelectorModel_FilterItems_MatchingQuery(t *testing.T) {
	items := []SelectorItem{
		{ID: "1", Columns: []string{"Apple"}},
		{ID: "2", Columns: []string{"Banana"}},
		{ID: "3", Columns: []string{"Apricot"}},
	}

	m := NewSelectorModel("Fruits", items)
	m.filterItems("ap")

	// Should match Apple and Apricot
	if len(m.filteredItems) < 2 {
		t.Errorf("filteredItems length = %d, want at least 2", len(m.filteredItems))
	}

	// Check that Apple and Apricot are in results
	foundApple := false
	foundApricot := false
	for _, item := range m.filteredItems {
		if item.Columns[0] == "Apple" {
			foundApple = true
		}
		if item.Columns[0] == "Apricot" {
			foundApricot = true
		}
	}
	if !foundApple {
		t.Error("expected Apple in filtered results")
	}
	if !foundApricot {
		t.Error("expected Apricot in filtered results")
	}
}

func TestSelectorModel_FilterItems_NoMatch(t *testing.T) {
	items := []SelectorItem{
		{ID: "1", Columns: []string{"Apple"}},
		{ID: "2", Columns: []string{"Banana"}},
	}

	m := NewSelectorModel("Fruits", items)
	m.filterItems("xyz")

	if len(m.filteredItems) != 0 {
		t.Errorf("filteredItems length = %d, want 0 for non-matching query", len(m.filteredItems))
	}
}

func TestSelectorModel_FilterItems_MultipleColumns(t *testing.T) {
	items := []SelectorItem{
		{ID: "1", Columns: []string{"John", "Engineer"}},
		{ID: "2", Columns: []string{"Jane", "Designer"}},
		{ID: "3", Columns: []string{"Bob", "Engineer"}},
	}

	m := NewSelectorModel("People", items)
	m.filterItems("engineer")

	// Should match John and Bob (both are Engineers)
	if len(m.filteredItems) != 2 {
		t.Errorf("filteredItems length = %d, want 2", len(m.filteredItems))
	}
}

func TestSelectorModel_Update_Escape(t *testing.T) {
	items := []SelectorItem{
		{ID: "1", Columns: []string{"Item 1"}},
	}
	m := NewSelectorModel("Test", items)

	keyMsg := tea.KeyMsg{Type: tea.KeyEsc}
	newModel, _ := m.Update(keyMsg)
	sm := newModel.(SelectorModel)

	if !sm.done {
		t.Error("done should be true after escape")
	}
	if !sm.canceled {
		t.Error("canceled should be true after escape")
	}
}

func TestSelectorModel_Update_CtrlC(t *testing.T) {
	items := []SelectorItem{
		{ID: "1", Columns: []string{"Item 1"}},
	}
	m := NewSelectorModel("Test", items)

	keyMsg := tea.KeyMsg{Type: tea.KeyCtrlC}
	newModel, _ := m.Update(keyMsg)
	sm := newModel.(SelectorModel)

	if !sm.done {
		t.Error("done should be true after ctrl+c")
	}
	if !sm.canceled {
		t.Error("canceled should be true after ctrl+c")
	}
}

func TestSelectorModel_Update_EnterSearch(t *testing.T) {
	items := []SelectorItem{
		{ID: "1", Columns: []string{"Item 1"}},
	}
	m := NewSelectorModel("Test", items)

	// Press / to enter search mode
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")}
	newModel, _ := m.Update(keyMsg)
	sm := newModel.(SelectorModel)

	if !sm.searchMode {
		t.Error("searchMode should be true after pressing /")
	}
	if !sm.searchFocused {
		t.Error("searchFocused should be true after pressing /")
	}
}

func TestSelectorModel_Update_EnterSearchWithF(t *testing.T) {
	items := []SelectorItem{
		{ID: "1", Columns: []string{"Item 1"}},
	}
	m := NewSelectorModel("Test", items)

	// Press f to enter search mode
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")}
	newModel, _ := m.Update(keyMsg)
	sm := newModel.(SelectorModel)

	if !sm.searchMode {
		t.Error("searchMode should be true after pressing f")
	}
}

func TestSelectorModel_Update_ToggleHelp(t *testing.T) {
	items := []SelectorItem{
		{ID: "1", Columns: []string{"Item 1"}},
	}
	m := NewSelectorModel("Test", items)

	// Initially help is shown
	if !m.showHelp {
		t.Error("showHelp should be true initially")
	}

	// Press ? to toggle
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")}
	newModel, _ := m.Update(keyMsg)
	sm := newModel.(SelectorModel)

	if sm.showHelp {
		t.Error("showHelp should be false after toggle")
	}

	// Press h to toggle back
	keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")}
	newModel, _ = sm.Update(keyMsg)
	sm = newModel.(SelectorModel)

	if !sm.showHelp {
		t.Error("showHelp should be true after second toggle")
	}
}

func TestSelectorModel_Update_SelectItem(t *testing.T) {
	items := []SelectorItem{
		{ID: "1", Columns: []string{"Item 1"}},
		{ID: "2", Columns: []string{"Item 2"}},
	}
	m := NewSelectorModel("Test", items)

	// Press enter to select
	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
	newModel, _ := m.Update(keyMsg)
	sm := newModel.(SelectorModel)

	if !sm.done {
		t.Error("done should be true after enter")
	}
	if sm.canceled {
		t.Error("canceled should be false after enter")
	}
	if sm.selectedItem == nil {
		t.Error("selectedItem should not be nil")
	}
	if sm.selectedItem.ID != "1" {
		t.Errorf("selectedItem.ID = %q, want '1'", sm.selectedItem.ID)
	}
}

func TestSelectorModel_Update_SearchModeEscape(t *testing.T) {
	items := []SelectorItem{
		{ID: "1", Columns: []string{"Item 1"}},
	}
	m := NewSelectorModel("Test", items)
	m.searchMode = true
	m.searchFocused = true
	m.searchInput.SetValue("test")

	// Press escape to exit search mode
	keyMsg := tea.KeyMsg{Type: tea.KeyEsc}
	newModel, _ := m.Update(keyMsg)
	sm := newModel.(SelectorModel)

	if sm.searchMode {
		t.Error("searchMode should be false after escape")
	}
	if sm.searchFocused {
		t.Error("searchFocused should be false after escape")
	}
	if sm.searchInput.Value() != "" {
		t.Error("search input should be cleared after escape")
	}
}

func TestSelectorModel_Update_SearchModeTab(t *testing.T) {
	items := []SelectorItem{
		{ID: "1", Columns: []string{"Item 1"}},
	}
	m := NewSelectorModel("Test", items)
	m.searchMode = true
	m.searchFocused = true

	// Press tab to toggle focus
	keyMsg := tea.KeyMsg{Type: tea.KeyTab}
	newModel, _ := m.Update(keyMsg)
	sm := newModel.(SelectorModel)

	if sm.searchFocused {
		t.Error("searchFocused should be false after tab")
	}

	// Tab again to focus back
	newModel, _ = sm.Update(keyMsg)
	sm = newModel.(SelectorModel)

	if !sm.searchFocused {
		t.Error("searchFocused should be true after second tab")
	}
}

func TestSelectorModel_View_EmptyItems(t *testing.T) {
	m := NewSelectorModel("Empty", []SelectorItem{})

	view := m.View()
	if !strings.Contains(view, "No items to display") {
		t.Error("view should show 'No items to display' for empty list")
	}
}

func TestSelectorModel_View_WithItems(t *testing.T) {
	items := []SelectorItem{
		{ID: "1", Columns: []string{"Item 1"}, Headers: []string{"Name"}},
	}
	m := NewSelectorModel("Test Selector", items)

	view := m.View()
	if !strings.Contains(view, "Test Selector") {
		t.Error("view should contain title")
	}
}

func TestSelectorModel_View_SearchMode(t *testing.T) {
	items := []SelectorItem{
		{ID: "1", Columns: []string{"Item 1"}},
	}
	m := NewSelectorModel("Test", items)
	m.searchMode = true

	view := m.View()
	if !strings.Contains(view, "Search:") {
		t.Error("view should show 'Search:' in search mode")
	}
}

func TestSelectorModel_View_NoResults(t *testing.T) {
	items := []SelectorItem{
		{ID: "1", Columns: []string{"Item 1"}},
	}
	m := NewSelectorModel("Test", items)
	m.searchMode = true
	m.filterItems("nonexistent")

	view := m.View()
	if !strings.Contains(view, "No matching items found") {
		t.Error("view should show 'No matching items found' when filter has no results")
	}
}

func TestSelectorModel_View_FilteredCount(t *testing.T) {
	items := []SelectorItem{
		{ID: "1", Columns: []string{"Apple"}},
		{ID: "2", Columns: []string{"Banana"}},
		{ID: "3", Columns: []string{"Apricot"}},
	}
	m := NewSelectorModel("Fruits", items)
	m.searchMode = true
	m.filterItems("ap")

	view := m.View()
	if !strings.Contains(view, "Showing") {
		t.Error("view should show filtered count when items are filtered")
	}
}

func TestSelectorModel_GetSelectedItem(t *testing.T) {
	items := []SelectorItem{
		{ID: "1", Columns: []string{"Item 1"}},
	}
	m := NewSelectorModel("Test", items)

	// Initially nil
	if m.GetSelectedItem() != nil {
		t.Error("GetSelectedItem should return nil initially")
	}

	// After selection
	m.selectedItem = &items[0]
	if m.GetSelectedItem() == nil {
		t.Error("GetSelectedItem should not be nil after selection")
	}
	if m.GetSelectedItem().ID != "1" {
		t.Errorf("GetSelectedItem().ID = %q, want '1'", m.GetSelectedItem().ID)
	}
}

func TestSelectorModel_GetError(t *testing.T) {
	m := NewSelectorModel("Test", []SelectorItem{})

	// Initially nil
	if m.GetError() != nil {
		t.Error("GetError should return nil initially")
	}
}

func TestSelectorModel_IsCanceled(t *testing.T) {
	m := NewSelectorModel("Test", []SelectorItem{})

	if m.IsCanceled() {
		t.Error("IsCanceled should return false initially")
	}

	m.canceled = true
	if !m.IsCanceled() {
		t.Error("IsCanceled should return true after setting canceled")
	}
}

func TestSelectorItem_Structure(t *testing.T) {
	item := SelectorItem{
		ID:      "test-id",
		Data:    "some data",
		Columns: []string{"col1", "col2"},
		Headers: []string{"Header 1", "Header 2"},
	}

	if item.ID != "test-id" {
		t.Errorf("ID = %q, want 'test-id'", item.ID)
	}
	if item.Data != "some data" {
		t.Errorf("Data = %v, want 'some data'", item.Data)
	}
	if len(item.Columns) != 2 {
		t.Errorf("Columns length = %d, want 2", len(item.Columns))
	}
	if len(item.Headers) != 2 {
		t.Errorf("Headers length = %d, want 2", len(item.Headers))
	}
}

func TestSelectorModel_Init(t *testing.T) {
	m := NewSelectorModel("Test", []SelectorItem{})

	cmd := m.Init()
	if cmd == nil {
		t.Error("Init should return a command (textinput.Blink)")
	}
}
