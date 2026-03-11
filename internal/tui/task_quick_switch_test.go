package tui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func newTestAppForQuickSwitch() *AppModel {
	cfg := &config.Config{}
	manager := &config.Manager{}
	return &AppModel{
		config:        cfg,
		configManager: manager,
	}
}

// initQuickSwitchFilter initializes the filter index for tests that
// call methods depending on quickSwitchFiltered (pin, remove, etc.).
func initQuickSwitchFilter(app *AppModel) {
	app.rebuildQuickSwitchFilter()
}

func TestRecordTaskVisit(t *testing.T) {
	t.Run("records a visit", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		task := &models.Task{ID: "t1", Title: "Task 1"}
		project := &models.Project{ID: "p1", Name: "Project 1"}

		app.recordTaskVisit(task, project)

		if len(app.recentTaskVisits) != 1 {
			t.Fatalf("expected 1 visit, got %d", len(app.recentTaskVisits))
		}
		if app.recentTaskVisits[0].Task.ID != "t1" {
			t.Errorf("expected task t1, got %s", app.recentTaskVisits[0].Task.ID)
		}
	})

	t.Run("dedup moves revisit to front", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		t1 := &models.Task{ID: "t1", Title: "Task 1"}
		t2 := &models.Task{ID: "t2", Title: "Task 2"}
		t3 := &models.Task{ID: "t3", Title: "Task 3"}
		p := &models.Project{ID: "p1", Name: "Project 1"}

		app.recordTaskVisit(t1, p)
		app.recordTaskVisit(t2, p)
		app.recordTaskVisit(t3, p)
		// Revisit t1 — should move to front
		app.recordTaskVisit(t1, p)

		if len(app.recentTaskVisits) != 3 {
			t.Fatalf("expected 3 visits, got %d", len(app.recentTaskVisits))
		}
		if app.recentTaskVisits[0].Task.ID != "t1" {
			t.Errorf("expected t1 at front, got %s", app.recentTaskVisits[0].Task.ID)
		}
		if app.recentTaskVisits[1].Task.ID != "t3" {
			t.Errorf("expected t3 at index 1, got %s", app.recentTaskVisits[1].Task.ID)
		}
		if app.recentTaskVisits[2].Task.ID != "t2" {
			t.Errorf("expected t2 at index 2, got %s", app.recentTaskVisits[2].Task.ID)
		}
	})

	t.Run("caps at config.DefaultMaxRecentTaskVisits", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		p := &models.Project{ID: "p1", Name: "Project 1"}

		for i := range config.DefaultMaxRecentTaskVisits + 5 {
			task := &models.Task{ID: fmt.Sprintf("t%d", i), Title: fmt.Sprintf("Task %d", i)}
			app.recordTaskVisit(task, p)
		}

		if len(app.recentTaskVisits) != config.DefaultMaxRecentTaskVisits {
			t.Fatalf("expected %d visits, got %d", config.DefaultMaxRecentTaskVisits, len(app.recentTaskVisits))
		}
	})

	t.Run("persists to config", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		t1 := &models.Task{ID: "t1", Title: "Task 1"}
		t2 := &models.Task{ID: "t2", Title: "Task 2"}
		p := &models.Project{ID: "p1", Name: "Project 1"}

		app.recordTaskVisit(t1, p)
		app.recordTaskVisit(t2, p)

		visits := app.config.GetRecentTaskVisits()
		if len(visits) != 2 {
			t.Fatalf("expected 2 persisted visits, got %d", len(visits))
		}
		if visits[0].TaskID != "t2" || visits[0].ProjectID != "p1" {
			t.Errorf("expected t2/p1 at front, got %s/%s", visits[0].TaskID, visits[0].ProjectID)
		}
	})

	t.Run("preserves pinned on dedup", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		t1 := &models.Task{ID: "t1", Title: "Task 1"}
		p := &models.Project{ID: "p1", Name: "Project 1"}

		app.recordTaskVisit(t1, p)
		app.recentTaskVisits[0].Pinned = true
		// Revisit same task — should preserve Pinned
		app.recordTaskVisit(t1, p)

		if !app.recentTaskVisits[0].Pinned {
			t.Error("expected pinned to be preserved on revisit")
		}
	})

	t.Run("pinned entries exempt from eviction", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		p := &models.Project{ID: "p1", Name: "Project 1"}

		// Add a pinned entry first
		pinnedTask := &models.Task{ID: "pinned1", Title: "Pinned Task"}
		app.recordTaskVisit(pinnedTask, p)
		app.recentTaskVisits[0].Pinned = true

		// Fill remaining slots with unpinned tasks
		for i := range config.DefaultMaxRecentTaskVisits + 5 {
			task := &models.Task{ID: fmt.Sprintf("t%d", i), Title: fmt.Sprintf("Task %d", i)}
			app.recordTaskVisit(task, p)
		}

		// Pinned entry should still be present
		found := false
		for _, entry := range app.recentTaskVisits {
			if entry.Task.ID == "pinned1" {
				found = true
				if !entry.Pinned {
					t.Error("pinned entry lost its Pinned flag")
				}
				break
			}
		}
		if !found {
			t.Error("pinned entry was evicted")
		}
	})

	t.Run("persists pinned to config", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		t1 := &models.Task{ID: "t1", Title: "Task 1"}
		p := &models.Project{ID: "p1", Name: "Project 1"}

		app.recordTaskVisit(t1, p)
		app.recentTaskVisits[0].Pinned = true
		app.persistRecentTaskVisits()

		visits := app.config.GetRecentTaskVisits()
		if len(visits) != 1 {
			t.Fatalf("expected 1 visit, got %d", len(visits))
		}
		if !visits[0].Pinned {
			t.Error("expected pinned flag to be persisted")
		}
	})

	t.Run("skips nil task or project", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		task := &models.Task{ID: "t1"}
		project := &models.Project{ID: "p1"}

		app.recordTaskVisit(nil, project)
		app.recordTaskVisit(task, nil)

		if len(app.recentTaskVisits) != 0 {
			t.Fatalf("expected 0 visits, got %d", len(app.recentTaskVisits))
		}
	})
}

func TestQuickSwitchTogglePin(t *testing.T) {
	t.Run("pin and unpin", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		p := &models.Project{ID: "p1", Name: "Project 1"}
		app.recentTaskVisits = []RecentTaskVisitEntry{
			{Task: &models.Task{ID: "t1", Title: "Task 1"}, Project: p},
			{Task: &models.Task{ID: "t2", Title: "Task 2"}, Project: p},
		}
		initQuickSwitchFilter(app)
		app.quickSwitchCursor = 1

		// Pin t2
		app.quickSwitchTogglePin()

		// After sorting, pinned t2 should be first
		if app.recentTaskVisits[0].Task.ID != "t2" || !app.recentTaskVisits[0].Pinned {
			t.Errorf("expected t2 pinned at index 0, got %s pinned=%v", app.recentTaskVisits[0].Task.ID, app.recentTaskVisits[0].Pinned)
		}
		// Cursor should follow t2 to index 0
		if app.quickSwitchCursor != 0 {
			t.Errorf("expected cursor at 0, got %d", app.quickSwitchCursor)
		}

		// Unpin t2
		app.quickSwitchTogglePin()
		if app.recentTaskVisits[0].Pinned {
			t.Error("expected t2 to be unpinned")
		}
	})

	t.Run("respects max pinned cap", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		p := &models.Project{ID: "p1", Name: "Project 1"}
		for i := range maxPinnedTaskVisits + 1 {
			app.recentTaskVisits = append(app.recentTaskVisits, RecentTaskVisitEntry{
				Task:    &models.Task{ID: fmt.Sprintf("t%d", i), Title: fmt.Sprintf("Task %d", i)},
				Project: p,
				Pinned:  i < maxPinnedTaskVisits,
			})
		}
		initQuickSwitchFilter(app)

		// Try to pin the last (unpinned) entry
		app.quickSwitchCursor = maxPinnedTaskVisits
		app.quickSwitchTogglePin()

		// Should still be unpinned
		lastEntry := app.recentTaskVisits[app.quickSwitchCursor]
		if lastEntry.Pinned {
			t.Error("should not be able to pin beyond max")
		}
	})
}

func TestQuickSwitchRemoveEntry(t *testing.T) {
	t.Run("removes entry and clamps cursor", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		p := &models.Project{ID: "p1", Name: "Project 1"}
		app.recentTaskVisits = []RecentTaskVisitEntry{
			{Task: &models.Task{ID: "t1", Title: "Task 1"}, Project: p},
			{Task: &models.Task{ID: "t2", Title: "Task 2"}, Project: p},
			{Task: &models.Task{ID: "t3", Title: "Task 3"}, Project: p},
		}
		initQuickSwitchFilter(app)
		app.quickSwitchCursor = 2

		app.quickSwitchRemoveEntry()

		if len(app.recentTaskVisits) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(app.recentTaskVisits))
		}
		if app.quickSwitchCursor != 1 {
			t.Errorf("expected cursor clamped to 1, got %d", app.quickSwitchCursor)
		}
	})

	t.Run("closes popup when last entry removed", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		p := &models.Project{ID: "p1", Name: "Project 1"}
		app.recentTaskVisits = []RecentTaskVisitEntry{
			{Task: &models.Task{ID: "t1", Title: "Task 1"}, Project: p},
		}
		app.showQuickSwitch = true
		app.quickSwitchCursor = 0
		initQuickSwitchFilter(app)

		app.quickSwitchRemoveEntry()

		if app.showQuickSwitch {
			t.Error("expected popup to close when list becomes empty")
		}
	})
}

func TestSortedQuickSwitchEntries(t *testing.T) {
	app := newTestAppForQuickSwitch()
	p := &models.Project{ID: "p1", Name: "Project 1"}
	app.recentTaskVisits = []RecentTaskVisitEntry{
		{Task: &models.Task{ID: "t1", Title: "Task 1"}, Project: p, Pinned: false},
		{Task: &models.Task{ID: "t2", Title: "Task 2"}, Project: p, Pinned: true},
		{Task: &models.Task{ID: "t3", Title: "Task 3"}, Project: p, Pinned: false},
		{Task: &models.Task{ID: "t4", Title: "Task 4"}, Project: p, Pinned: true},
	}

	sorted := app.sortedQuickSwitchEntries()

	// Pinned first: t2, t4, then unpinned: t1, t3
	expectedOrder := []string{"t2", "t4", "t1", "t3"}
	for i, id := range expectedOrder {
		if sorted[i].Task.ID != id {
			t.Errorf("index %d: expected %s, got %s", i, id, sorted[i].Task.ID)
		}
	}
}

func newTestAppWithPersisterForQuickSwitch() (*AppModel, *MockPersister) {
	cfg := &config.Config{}
	manager := &config.Manager{}
	mp := NewMockPersister()
	return &AppModel{
		config:        cfg,
		configManager: manager,
		persister:     mp,
	}, mp
}

func TestQuickSwitchPrefixDispatch(t *testing.T) {
	t.Run("status prefix changes task status", func(t *testing.T) {
		app, _ := newTestAppWithPersisterForQuickSwitch()
		task := &models.Task{ID: "t1", Title: "Task 1", ShortID: 1, Status: models.StatusTodo}
		p := &models.Project{ID: "p1", Name: "Project 1", Tasks: []*models.Task{task}}
		app.recentTaskVisits = []RecentTaskVisitEntry{
			{Task: task, Project: p},
		}
		initQuickSwitchFilter(app)
		app.showQuickSwitch = true
		app.quickSwitchPendingPrefix = ","

		// Press 's' for in_progress
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
		app.handleQuickSwitchKey(msg)

		if app.quickSwitchPendingPrefix != "" {
			t.Error("expected pending prefix to be cleared")
		}
		if task.Status != models.StatusInProgress {
			t.Errorf("expected status in_progress, got %s", task.Status)
		}
		if app.statusMessage == "" {
			t.Error("expected status message to be set")
		}
	})

	t.Run("priority prefix changes task priority", func(t *testing.T) {
		app, _ := newTestAppWithPersisterForQuickSwitch()
		task := &models.Task{ID: "t1", Title: "Task 1", ShortID: 1, Priority: 3}
		p := &models.Project{ID: "p1", Name: "Project 1", Tasks: []*models.Task{task}}
		app.recentTaskVisits = []RecentTaskVisitEntry{
			{Task: task, Project: p},
		}
		initQuickSwitchFilter(app)
		app.showQuickSwitch = true
		app.quickSwitchPendingPrefix = "!"

		// Press '1' for priority 1
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}}
		app.handleQuickSwitchKey(msg)

		if app.quickSwitchPendingPrefix != "" {
			t.Error("expected pending prefix to be cleared")
		}
		if task.Priority != 1 {
			t.Errorf("expected priority 1, got %d", task.Priority)
		}
	})

	t.Run("esc cancels pending prefix", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		p := &models.Project{ID: "p1", Name: "Project 1"}
		app.recentTaskVisits = []RecentTaskVisitEntry{
			{Task: &models.Task{ID: "t1", Title: "Task 1"}, Project: p},
		}
		initQuickSwitchFilter(app)
		app.showQuickSwitch = true
		app.quickSwitchPendingPrefix = ","

		msg := tea.KeyMsg{Type: tea.KeyEscape}
		app.handleQuickSwitchKey(msg)

		if app.quickSwitchPendingPrefix != "" {
			t.Error("expected pending prefix to be cleared on esc")
		}
		if !app.showQuickSwitch {
			t.Error("popup should still be open after canceling prefix")
		}
	})

	t.Run("invalid prefix key is ignored", func(t *testing.T) {
		app, _ := newTestAppWithPersisterForQuickSwitch()
		task := &models.Task{ID: "t1", Title: "Task 1", ShortID: 1, Status: models.StatusTodo, Priority: 3}
		p := &models.Project{ID: "p1", Name: "Project 1", Tasks: []*models.Task{task}}
		app.recentTaskVisits = []RecentTaskVisitEntry{
			{Task: task, Project: p},
		}
		initQuickSwitchFilter(app)
		app.showQuickSwitch = true
		app.quickSwitchPendingPrefix = ","

		// Press 'z' which is not a valid status key
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}}
		app.handleQuickSwitchKey(msg)

		if app.quickSwitchPendingPrefix != "" {
			t.Error("expected pending prefix to be cleared")
		}
		// Status should be unchanged
		if task.Status != models.StatusTodo {
			t.Errorf("expected status unchanged (todo), got %s", task.Status)
		}
	})
}

func TestQuickSwitchFilter(t *testing.T) {
	t.Run("filter narrows results", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		p := &models.Project{ID: "p1", Name: "Project Alpha"}
		app.recentTaskVisits = []RecentTaskVisitEntry{
			{Task: &models.Task{ID: "t1", Title: "Fix login bug"}, Project: p},
			{Task: &models.Task{ID: "t2", Title: "Add signup page"}, Project: p},
			{Task: &models.Task{ID: "t3", Title: "Login redesign"}, Project: p},
		}
		app.quickSwitchFilter = "login"
		app.rebuildQuickSwitchFilter()

		if len(app.quickSwitchFiltered) != 2 {
			t.Fatalf("expected 2 filtered results, got %d", len(app.quickSwitchFiltered))
		}
		// Both entries with "login" should be present
		ids := map[string]bool{}
		for _, idx := range app.quickSwitchFiltered {
			ids[app.recentTaskVisits[idx].Task.ID] = true
		}
		if !ids["t1"] || !ids["t3"] {
			t.Errorf("expected t1 and t3 in results, got %v", ids)
		}
	})

	t.Run("empty filter shows all", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		p := &models.Project{ID: "p1", Name: "Project 1"}
		app.recentTaskVisits = []RecentTaskVisitEntry{
			{Task: &models.Task{ID: "t1", Title: "Task 1"}, Project: p},
			{Task: &models.Task{ID: "t2", Title: "Task 2"}, Project: p},
		}
		app.quickSwitchFilter = ""
		app.rebuildQuickSwitchFilter()

		if len(app.quickSwitchFiltered) != 2 {
			t.Fatalf("expected 2 results, got %d", len(app.quickSwitchFiltered))
		}
	})

	t.Run("no matches returns empty", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		p := &models.Project{ID: "p1", Name: "Project 1"}
		app.recentTaskVisits = []RecentTaskVisitEntry{
			{Task: &models.Task{ID: "t1", Title: "Task 1"}, Project: p},
		}
		app.quickSwitchFilter = "zzzzz"
		app.rebuildQuickSwitchFilter()

		if len(app.quickSwitchFiltered) != 0 {
			t.Fatalf("expected 0 results, got %d", len(app.quickSwitchFiltered))
		}
	})

	t.Run("filter matches project name", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		p1 := &models.Project{ID: "p1", Name: "Backend API"}
		p2 := &models.Project{ID: "p2", Name: "Frontend App"}
		app.recentTaskVisits = []RecentTaskVisitEntry{
			{Task: &models.Task{ID: "t1", Title: "Fix bug"}, Project: p1},
			{Task: &models.Task{ID: "t2", Title: "Fix bug"}, Project: p2},
		}
		app.quickSwitchFilter = "backend"
		app.rebuildQuickSwitchFilter()

		if len(app.quickSwitchFiltered) != 1 {
			t.Fatalf("expected 1 result, got %d", len(app.quickSwitchFiltered))
		}
		if app.recentTaskVisits[app.quickSwitchFiltered[0]].Project.Name != "Backend API" {
			t.Error("expected match on Backend API project")
		}
	})

	t.Run("pinned entries rank first in filtered results", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		p := &models.Project{ID: "p1", Name: "Project 1"}
		app.recentTaskVisits = []RecentTaskVisitEntry{
			{Task: &models.Task{ID: "t1", Title: "Fix login"}, Project: p, Pinned: false},
			{Task: &models.Task{ID: "t2", Title: "Login page"}, Project: p, Pinned: true},
		}
		app.quickSwitchFilter = "login"
		app.rebuildQuickSwitchFilter()

		if len(app.quickSwitchFiltered) != 2 {
			t.Fatalf("expected 2 results, got %d", len(app.quickSwitchFiltered))
		}
		// Pinned entry should come first
		firstEntry := app.recentTaskVisits[app.quickSwitchFiltered[0]]
		if !firstEntry.Pinned {
			t.Error("expected pinned entry to rank first in filtered results")
		}
	})
}

func TestQuickSwitchFilterKeyHandling(t *testing.T) {
	t.Run("typing letter in normal mode is a no-op", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		p := &models.Project{ID: "p1", Name: "Project 1"}
		app.recentTaskVisits = []RecentTaskVisitEntry{
			{Task: &models.Task{ID: "t1", Title: "Task 1"}, Project: p},
		}
		initQuickSwitchFilter(app)
		app.showQuickSwitch = true

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
		app.handleQuickSwitchKey(msg)

		if app.quickSwitchFiltering {
			t.Error("expected filter mode to be inactive")
		}
		if !app.showQuickSwitch {
			t.Error("expected popup to remain open")
		}
	})

	t.Run("slash enters filter mode", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		p := &models.Project{ID: "p1", Name: "Project 1"}
		app.recentTaskVisits = []RecentTaskVisitEntry{
			{Task: &models.Task{ID: "t1", Title: "Task 1"}, Project: p},
		}
		initQuickSwitchFilter(app)
		app.showQuickSwitch = true

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
		app.handleQuickSwitchKey(msg)

		if !app.quickSwitchFiltering {
			t.Error("expected filter mode to be active")
		}
		if app.quickSwitchFilter != "" {
			t.Errorf("expected empty filter, got %q", app.quickSwitchFilter)
		}
	})

	t.Run("backspace removes filter char and exits filter mode when empty", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		p := &models.Project{ID: "p1", Name: "Project 1"}
		app.recentTaskVisits = []RecentTaskVisitEntry{
			{Task: &models.Task{ID: "t1", Title: "Task 1"}, Project: p},
		}
		app.quickSwitchFiltering = true
		app.quickSwitchFilter = "a"
		initQuickSwitchFilter(app)

		msg := tea.KeyMsg{Type: tea.KeyBackspace}
		app.handleQuickSwitchKey(msg)

		if app.quickSwitchFiltering {
			t.Error("expected filter mode to be deactivated")
		}
		if app.quickSwitchFilter != "" {
			t.Errorf("expected empty filter, got %q", app.quickSwitchFilter)
		}
	})

	t.Run("comma enters status prefix mode", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		p := &models.Project{ID: "p1", Name: "Project 1"}
		app.recentTaskVisits = []RecentTaskVisitEntry{
			{Task: &models.Task{ID: "t1", Title: "Task 1"}, Project: p},
		}
		initQuickSwitchFilter(app)
		app.showQuickSwitch = true

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}}
		app.handleQuickSwitchKey(msg)

		if app.quickSwitchPendingPrefix != "," {
			t.Errorf("expected pending prefix ',', got %q", app.quickSwitchPendingPrefix)
		}
	})

	t.Run("exclamation enters priority prefix mode", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		p := &models.Project{ID: "p1", Name: "Project 1"}
		app.recentTaskVisits = []RecentTaskVisitEntry{
			{Task: &models.Task{ID: "t1", Title: "Task 1"}, Project: p},
		}
		initQuickSwitchFilter(app)
		app.showQuickSwitch = true

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'!'}}
		app.handleQuickSwitchKey(msg)

		if app.quickSwitchPendingPrefix != "!" {
			t.Errorf("expected pending prefix '!', got %q", app.quickSwitchPendingPrefix)
		}
	})

	t.Run("comma in filter mode is typed as filter text", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		p := &models.Project{ID: "p1", Name: "Project 1"}
		app.recentTaskVisits = []RecentTaskVisitEntry{
			{Task: &models.Task{ID: "t1", Title: "Task 1"}, Project: p},
		}
		app.quickSwitchFiltering = true
		app.quickSwitchFilter = "task"
		initQuickSwitchFilter(app)
		app.showQuickSwitch = true

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}}
		app.handleQuickSwitchKey(msg)

		if app.quickSwitchPendingPrefix != "" {
			t.Errorf("expected no pending prefix, got %q", app.quickSwitchPendingPrefix)
		}
		if app.quickSwitchFilter != "task," {
			t.Errorf("expected filter 'task,', got %q", app.quickSwitchFilter)
		}
	})

	t.Run("esc clears filter first, then closes popup", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		p := &models.Project{ID: "p1", Name: "Project 1"}
		app.recentTaskVisits = []RecentTaskVisitEntry{
			{Task: &models.Task{ID: "t1", Title: "Task 1"}, Project: p},
		}
		app.showQuickSwitch = true
		app.quickSwitchFiltering = true
		app.quickSwitchFilter = "abc"
		initQuickSwitchFilter(app)

		// First esc: clears filter
		msg := tea.KeyMsg{Type: tea.KeyEscape}
		app.handleQuickSwitchKey(msg)

		if app.quickSwitchFilter != "" {
			t.Errorf("expected filter cleared, got %q", app.quickSwitchFilter)
		}
		if !app.showQuickSwitch {
			t.Error("popup should still be open after clearing filter")
		}

		// Second esc: closes popup (now in normal mode since filter was cleared)
		app.handleQuickSwitchKey(msg)

		if app.showQuickSwitch {
			t.Error("expected popup to close on second esc")
		}
	})
}
