package tui

import (
	"fmt"
	"testing"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func newGlobalSearchHistoryTestModel(history []string, results []models.SearchResult) *GlobalSearchModel {
	input := textinput.New()
	m := &GlobalSearchModel{
		config:             config.NewConfig(),
		allTasks:           results,
		searchInput:        input,
		tabs:               TabState{Names: globalSearchTabNames},
		hideCompleted:      false,
		searchHistory:      history,
		searchHistoryIndex: -1,
	}
	m.filterTasks()
	return m
}

func TestGlobalSearchHistoryNavigation(t *testing.T) {
	results := []models.SearchResult{
		{Task: &models.Task{ID: "1", Title: "Alpha task", Status: models.StatusTodo}},
		{Task: &models.Task{ID: "2", Title: "Beta task", Status: models.StatusTodo}},
	}
	m := newGlobalSearchHistoryTestModel([]string{"alpha", "beta"}, results)

	model, _ := m.Update(syntheticAltKey(tea.KeyUp))
	m = model.(*GlobalSearchModel)
	if got := m.searchInput.Value(); got != "alpha" {
		t.Fatalf("Alt+Up should recall most recent query, got %q", got)
	}
	if m.searchHistoryIndex != 0 {
		t.Fatalf("expected history index 0, got %d", m.searchHistoryIndex)
	}
	if len(m.filtered) != 1 || m.filtered[0].result.Task.ID != "1" {
		t.Fatalf("expected alpha query to refresh filtered results, got %#v", m.filtered)
	}

	model, _ = m.Update(syntheticAltKey(tea.KeyUp))
	m = model.(*GlobalSearchModel)
	if got := m.searchInput.Value(); got != "beta" {
		t.Fatalf("second Alt+Up should recall older query, got %q", got)
	}
	if m.searchHistoryIndex != 1 {
		t.Fatalf("expected history index 1, got %d", m.searchHistoryIndex)
	}
	if len(m.filtered) != 1 || m.filtered[0].result.Task.ID != "2" {
		t.Fatalf("expected beta query to refresh filtered results, got %#v", m.filtered)
	}

	model, _ = m.Update(syntheticAltKey(tea.KeyDown))
	m = model.(*GlobalSearchModel)
	if got := m.searchInput.Value(); got != "alpha" {
		t.Fatalf("Alt+Down should move to newer query, got %q", got)
	}
	if m.searchHistoryIndex != 0 {
		t.Fatalf("expected history index 0, got %d", m.searchHistoryIndex)
	}

	model, _ = m.Update(syntheticAltKey(tea.KeyDown))
	m = model.(*GlobalSearchModel)
	if got := m.searchInput.Value(); got != "" {
		t.Fatalf("Alt+Down from newest should clear input, got %q", got)
	}
	if m.searchHistoryIndex != -1 {
		t.Fatalf("expected history mode to exit, got index %d", m.searchHistoryIndex)
	}
	if len(m.filtered) != 0 {
		t.Fatalf("expected empty query to clear filtered results, got %d", len(m.filtered))
	}
}

func TestGlobalSearchEnterRecordsTaskQuery(t *testing.T) {
	m := newGlobalSearchHistoryTestModel(nil, nil)
	task := &models.Task{ID: "1", Title: "Alpha task", Status: models.StatusTodo}
	m.filtered = []filteredSearchResult{{result: models.SearchResult{Task: task}}}
	m.searchInput.SetValue("alpha")

	_, cmd := m.Update(syntheticSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("enter on a task result should return a command")
	}
	if history := m.config.GetGlobalSearchHistory(); len(history) != 1 || history[0] != "alpha" {
		t.Fatalf("expected task query to be recorded, got %#v", history)
	}
}

func TestGlobalSearchEnterRecordsNoteQuery(t *testing.T) {
	m := newGlobalSearchHistoryTestModel(nil, nil)
	note := &models.Note{Title: "Alpha note", Filename: "alpha.md"}
	m.filtered = []filteredSearchResult{{result: models.SearchResult{Note: note}}}
	m.searchInput.SetValue("alpha")

	_, cmd := m.Update(syntheticSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("enter on a note result should return a command")
	}
	if history := m.config.GetGlobalSearchHistory(); len(history) != 1 || history[0] != "alpha" {
		t.Fatalf("expected note query to be recorded, got %#v", history)
	}
}

func TestGlobalSearchEnterDoesNotRecordWithoutSelectableQuery(t *testing.T) {
	m := newGlobalSearchHistoryTestModel(nil, nil)

	_, cmd := m.Update(syntheticSpecialKey(tea.KeyEnter))
	if cmd != nil {
		t.Fatal("enter without a selectable result should not return a command")
	}
	if history := m.config.GetGlobalSearchHistory(); len(history) != 0 {
		t.Fatalf("expected no recorded history, got %#v", history)
	}

	m.searchInput.SetValue("   ")
	m.filtered = []filteredSearchResult{{result: models.SearchResult{Task: &models.Task{ID: "1"}}}}
	_, cmd = m.Update(syntheticSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("enter on a result should still return a command")
	}
	if history := m.config.GetGlobalSearchHistory(); len(history) != 0 {
		t.Fatalf("expected blank query not to be recorded, got %#v", history)
	}
}

func TestGlobalSearchRecordHistoryDedupesAndCaps(t *testing.T) {
	m := newGlobalSearchHistoryTestModel(nil, nil)
	for i := range 11 {
		m.searchHistory = append(m.searchHistory, fmt.Sprintf("q%d", i))
	}
	m.searchInput.SetValue("q5")

	m.recordGlobalSearchHistory()

	history := m.config.GetGlobalSearchHistory()
	if len(history) != 10 {
		t.Fatalf("expected capped history length 10, got %d: %#v", len(history), history)
	}
	if history[0] != "q5" {
		t.Fatalf("expected duplicate query moved to front, got %#v", history)
	}
	for i, query := range history[1:] {
		if query == "q5" {
			t.Fatalf("history should be deduplicated; found duplicate at index %d in %#v", i+1, history)
		}
	}
}
