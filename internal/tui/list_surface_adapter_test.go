package tui

import (
	"testing"
)

// testItem implements PickerItem for testing.
type testItem struct {
	id    string
	text  string
	count int
}

// Compile-time interface satisfaction check.
var _ PickerItem = testItem{}

func (t testItem) FilterText() string  { return t.text }
func (t testItem) DisplayText() string { return t.text }
func (t testItem) CountValue() int     { return t.count }
func (t testItem) ItemID() string      { return t.id }

func TestNewListSurfaceAdapter(t *testing.T) {
	items := []testItem{
		{id: "1", text: "Alpha", count: 5},
		{id: "2", text: "Beta", count: 3},
		{id: "3", text: "Gamma", count: 1},
	}

	picker := NewListSurfaceAdapter(items, PickerConfig{
		Title:          "Test Picker",
		EmptyMessage:   "No items",
		NoMatchMessage: "No matches",
		MultiSelect:    false,
		ShowCounts:     true,
	})

	if len(picker.SourceItems()) != 3 {
		t.Errorf("expected 3 items, got %d", len(picker.SourceItems()))
	}
	if len(picker.FilteredItems()) != 3 {
		t.Errorf("expected 3 filtered items, got %d", len(picker.FilteredItems()))
	}
	if picker.CursorIndex() != 0 {
		t.Errorf("expected cursor at 0, got %d", picker.CursorIndex())
	}
}

func TestListSurfaceAdapterFilter(t *testing.T) {
	items := []testItem{
		{id: "1", text: "Alpha", count: 5},
		{id: "2", text: "Beta", count: 3},
		{id: "3", text: "Alphabet", count: 1},
	}

	picker := NewListSurfaceAdapter(items, PickerConfig{
		Title:       "Test",
		MultiSelect: false,
	})

	tests := []struct {
		query    string
		expected int
	}{
		{"", 3},
		{"alpha", 2},
		{"ALPHA", 2},
		{"bet", 2},
		{"gamma", 0},
		{"a", 3},
	}

	for _, tt := range tests {
		picker.Filter(tt.query)
		if len(picker.FilteredItems()) != tt.expected {
			t.Errorf("Filter(%q): expected %d items, got %d", tt.query, tt.expected, len(picker.FilteredItems()))
		}
	}
}

func TestListSurfaceAdapterNavigation(t *testing.T) {
	items := []testItem{
		{id: "1", text: "Alpha"},
		{id: "2", text: "Beta"},
		{id: "3", text: "Gamma"},
	}

	picker := NewListSurfaceAdapter(items, PickerConfig{MultiSelect: false})

	tests := []struct {
		key      string
		expected int
	}{
		{"j", 1},
		{"j", 2},
		{"j", 2},
		{"k", 1},
		{"k", 0},
		{"down", 1},
		{"up", 0},
	}

	for _, tt := range tests {
		picker.HandleKeyMsg(syntheticKeyStr(tt.key))
		if picker.CursorIndex() != tt.expected {
			t.Errorf("after key %q: expected cursor at %d, got %d", tt.key, tt.expected, picker.CursorIndex())
		}
	}
}

func TestListSurfaceAdapterMultiSelect(t *testing.T) {
	items := []testItem{
		{id: "1", text: "Alpha"},
		{id: "2", text: "Beta"},
		{id: "3", text: "Gamma"},
	}

	picker := NewListSurfaceAdapter(items, PickerConfig{MultiSelect: true})

	// Toggle first item
	picker.HandleKeyMsg(syntheticKey(' '))
	if !picker.SelectionMap()["1"] {
		t.Error("expected item 1 to be selected")
	}

	// Move down and toggle second item
	picker.HandleKeyMsg(syntheticKey('j'))
	picker.HandleKeyMsg(syntheticKey(' '))
	if !picker.SelectionMap()["2"] {
		t.Error("expected item 2 to be selected")
	}

	// Verify both are selected
	selected := picker.SelectedItems()
	if len(selected) != 2 {
		t.Errorf("expected 2 selected items, got %d", len(selected))
	}

	// Clear selections
	picker.HandleKeyMsg(syntheticKey('c'))
	if len(picker.SelectionMap()) != 0 {
		t.Errorf("expected 0 selected after clear, got %d", len(picker.SelectionMap()))
	}
}

func TestListSurfaceAdapterSingleSelect(t *testing.T) {
	items := []testItem{
		{id: "1", text: "Alpha"},
		{id: "2", text: "Beta"},
	}

	picker := NewListSurfaceAdapter(items, PickerConfig{MultiSelect: false})

	// Space should not toggle in single-select mode
	picker.HandleKeyMsg(syntheticKey(' '))
	if len(picker.SelectionMap()) != 0 {
		t.Error("space should not select in single-select mode")
	}

	// SelectedItem should return current cursor position
	selected := picker.SelectedItem()
	if selected.ItemID() != "1" {
		t.Errorf("expected selected item id '1', got %q", selected.ItemID())
	}

	picker.SetCursorIndex(1)
	selected = picker.SelectedItem()
	if selected.ItemID() != "2" {
		t.Errorf("expected selected item id '2', got %q", selected.ItemID())
	}
}

func TestListSurfaceAdapterPreSelect(t *testing.T) {
	items := []testItem{
		{id: "1", text: "Alpha"},
		{id: "2", text: "Beta"},
		{id: "3", text: "Gamma"},
	}

	picker := NewListSurfaceAdapter(items, PickerConfig{MultiSelect: true})

	picker.PreSelectByID("2")
	picker.PreSelectByID("3")

	if !picker.SelectionMap()["2"] || !picker.SelectionMap()["3"] {
		t.Error("expected items 2 and 3 to be pre-selected")
	}
	if picker.SelectionMap()["1"] {
		t.Error("item 1 should not be selected")
	}
}

func TestListSurfaceAdapterSetCursorToID(t *testing.T) {
	items := []testItem{
		{id: "1", text: "Alpha"},
		{id: "2", text: "Beta"},
		{id: "3", text: "Gamma"},
	}

	picker := NewListSurfaceAdapter(items, PickerConfig{})

	picker.SetCursorToID("2")
	if picker.CursorIndex() != 1 {
		t.Errorf("expected cursor at 1, got %d", picker.CursorIndex())
	}

	picker.SetCursorToID("3")
	if picker.CursorIndex() != 2 {
		t.Errorf("expected cursor at 2, got %d", picker.CursorIndex())
	}

	// Non-existent ID should not change cursor
	picker.SetCursorToID("999")
	if picker.CursorIndex() != 2 {
		t.Errorf("expected cursor to stay at 2, got %d", picker.CursorIndex())
	}
}

func TestListSurfaceAdapterFilterResetsCursor(t *testing.T) {
	items := []testItem{
		{id: "1", text: "Alpha"},
		{id: "2", text: "Beta"},
		{id: "3", text: "Gamma"},
	}

	picker := NewListSurfaceAdapter(items, PickerConfig{})
	picker.SetCursorIndex(2)

	// Filter that returns fewer items than cursor position
	picker.Filter("Alpha")
	if picker.CursorIndex() != 0 {
		t.Errorf("expected cursor reset to 0, got %d", picker.CursorIndex())
	}
}

func TestListSurfaceAdapterSelectedIDs(t *testing.T) {
	items := []testItem{
		{id: "a", text: "Alpha"},
		{id: "b", text: "Beta"},
		{id: "c", text: "Gamma"},
	}

	picker := NewListSurfaceAdapter(items, PickerConfig{MultiSelect: true})
	picker.PreSelectByID("a")
	picker.PreSelectByID("c")

	ids := picker.SelectedIDs()
	if len(ids) != 2 {
		t.Errorf("expected 2 selected IDs, got %d", len(ids))
	}

	// Check that both IDs are present (order not guaranteed)
	found := make(map[string]bool)
	for _, id := range ids {
		found[id] = true
	}
	if !found["a"] || !found["c"] {
		t.Error("expected IDs 'a' and 'c' to be in selected IDs")
	}
}

func TestListSurfaceAdapterRenderInBoxFitsTitleHeaderToWidth(t *testing.T) {
	items := []testItem{{id: "1", text: "Alpha"}}
	picker := NewListSurfaceAdapter(items, PickerConfig{
		EmptyMessage:   "No items",
		NoMatchMessage: "No matches",
	})
	picker.SetSize(60, 20)

	rendered := picker.RenderInBox("Filter by Attribute", MutedStyle.Render("/ search · Enter select · x clear · Esc cancel"), 5)
	_, issues := normalizeScreen(rendered, 60, 20)
	for _, issue := range issues {
		if issue.Kind == "width_overflow" {
			t.Fatalf("RenderInBox title/header overflowed width: %+v\n%s", issue, rendered)
		}
	}
}

func TestPluralize(t *testing.T) {
	tests := []struct {
		count    int
		singular string
		plural   string
		expected string
	}{
		{0, "item", "items", "items"},
		{1, "item", "items", "item"},
		{2, "item", "items", "items"},
		{1, "y", "ies", "y"},
		{5, "y", "ies", "ies"},
	}

	for _, tt := range tests {
		result := pluralize(tt.count, tt.singular, tt.plural)
		if result != tt.expected {
			t.Errorf("pluralize(%d, %q, %q) = %q, expected %q",
				tt.count, tt.singular, tt.plural, result, tt.expected)
		}
	}
}
