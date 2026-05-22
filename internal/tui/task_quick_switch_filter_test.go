package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

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
		app.recentTaskVisits = []RecentTaskVisitEntry{{Task: &models.Task{ID: "t1", Title: "Task 1"}, Project: p}}
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

	t.Run("equal-score matches preserve recency order deterministically", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		p := &models.Project{ID: "p1", Name: "Project 1"}
		// All titles match "fix" with identical fuzzy scores; ensure the
		// rebuilt filter preserves recency (the order in recentTaskVisits)
		// across runs rather than non-deterministically reordering.
		app.recentTaskVisits = []RecentTaskVisitEntry{
			{Task: &models.Task{ID: "t1", Title: "fix"}, Project: p},
			{Task: &models.Task{ID: "t2", Title: "fix"}, Project: p},
			{Task: &models.Task{ID: "t3", Title: "fix"}, Project: p},
		}
		app.quickSwitchFilter = "fix"

		var firstOrder []string
		for run := range 5 {
			app.rebuildQuickSwitchFilter()
			order := make([]string, 0, len(app.quickSwitchFiltered))
			for _, idx := range app.quickSwitchFiltered {
				order = append(order, app.recentTaskVisits[idx].Task.ID)
			}
			if run == 0 {
				firstOrder = order
				continue
			}
			for i, id := range order {
				if id != firstOrder[i] {
					t.Errorf("equal-score order shuffled across runs: run0=%v run%d=%v", firstOrder, run, order)
					return
				}
			}
		}
		// Recency (input order) must be preserved when scores tie.
		want := []string{"t1", "t2", "t3"}
		for i, id := range firstOrder {
			if id != want[i] {
				t.Errorf("expected recency order %v, got %v", want, firstOrder)
				break
			}
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
		app.recentTaskVisits = []RecentTaskVisitEntry{{Task: &models.Task{ID: "t1", Title: "Task 1"}, Project: p}}
		initQuickSwitchFilter(app)
		app.showQuickSwitch = true

		msg := syntheticKey('a')
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
		app.recentTaskVisits = []RecentTaskVisitEntry{{Task: &models.Task{ID: "t1", Title: "Task 1"}, Project: p}}
		initQuickSwitchFilter(app)
		app.showQuickSwitch = true

		msg := syntheticKey('/')
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
		app.recentTaskVisits = []RecentTaskVisitEntry{{Task: &models.Task{ID: "t1", Title: "Task 1"}, Project: p}}
		app.quickSwitchFiltering = true
		app.quickSwitchFilter = "a"
		initQuickSwitchFilter(app)

		msg := syntheticSpecialKey(tea.KeyBackspace)
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
		app.recentTaskVisits = []RecentTaskVisitEntry{{Task: &models.Task{ID: "t1", Title: "Task 1"}, Project: p}}
		initQuickSwitchFilter(app)
		app.showQuickSwitch = true

		msg := syntheticKey(',')
		app.handleQuickSwitchKey(msg)

		if app.quickSwitchPendingPrefix != "," {
			t.Errorf("expected pending prefix ',', got %q", app.quickSwitchPendingPrefix)
		}
	})

	t.Run("exclamation enters priority prefix mode", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		p := &models.Project{ID: "p1", Name: "Project 1"}
		app.recentTaskVisits = []RecentTaskVisitEntry{{Task: &models.Task{ID: "t1", Title: "Task 1"}, Project: p}}
		initQuickSwitchFilter(app)
		app.showQuickSwitch = true

		msg := syntheticKey('!')
		app.handleQuickSwitchKey(msg)

		if app.quickSwitchPendingPrefix != "!" {
			t.Errorf("expected pending prefix '!', got %q", app.quickSwitchPendingPrefix)
		}
	})

	t.Run("comma in filter mode is typed as filter text", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		p := &models.Project{ID: "p1", Name: "Project 1"}
		app.recentTaskVisits = []RecentTaskVisitEntry{{Task: &models.Task{ID: "t1", Title: "Task 1"}, Project: p}}
		app.quickSwitchFiltering = true
		app.quickSwitchFilter = "task"
		initQuickSwitchFilter(app)
		app.showQuickSwitch = true

		msg := syntheticKey(',')
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
		app.recentTaskVisits = []RecentTaskVisitEntry{{Task: &models.Task{ID: "t1", Title: "Task 1"}, Project: p}}
		app.showQuickSwitch = true
		app.quickSwitchFiltering = true
		app.quickSwitchFilter = "abc"
		initQuickSwitchFilter(app)

		msg := syntheticSpecialKey(tea.KeyEscape)
		app.handleQuickSwitchKey(msg)

		if app.quickSwitchFilter != "" {
			t.Errorf("expected filter cleared, got %q", app.quickSwitchFilter)
		}
		if !app.showQuickSwitch {
			t.Error("popup should still be open after clearing filter")
		}

		app.handleQuickSwitchKey(msg)

		if app.showQuickSwitch {
			t.Error("expected popup to close on second esc")
		}
	})
}
