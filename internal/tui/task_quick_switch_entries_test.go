package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

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

		app.quickSwitchTogglePin()

		if app.recentTaskVisits[0].Task.ID != "t2" || !app.recentTaskVisits[0].Pinned {
			t.Errorf("expected t2 pinned at index 0, got %s pinned=%v", app.recentTaskVisits[0].Task.ID, app.recentTaskVisits[0].Pinned)
		}
		if app.quickSwitchCursor != 0 {
			t.Errorf("expected cursor at 0, got %d", app.quickSwitchCursor)
		}

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

		app.quickSwitchCursor = maxPinnedTaskVisits
		app.quickSwitchTogglePin()

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

	expectedOrder := []string{"t2", "t4", "t1", "t3"}
	for i, id := range expectedOrder {
		if sorted[i].Task.ID != id {
			t.Errorf("index %d: expected %s, got %s", i, id, sorted[i].Task.ID)
		}
	}
}

func TestRenderQuickSwitchPopupShowsParkedSnippet(t *testing.T) {
	app := newTestAppForQuickSwitch()
	app.width = 110
	app.showQuickSwitch = true
	p := &models.Project{ID: "p1", Name: "Project 1"}
	task := &models.Task{ID: "t1", Title: "Task 1", Status: models.StatusTodo, Type: models.TaskTypeTask}
	parkedAt := time.Now().Add(-time.Hour)
	task.ParkedAt = &parkedAt
	task.ParkedNote = "waiting for schema review"
	app.recentTaskVisits = []RecentTaskVisitEntry{{Task: task, Project: p}}
	initQuickSwitchFilter(app)

	plain := ansi.Strip(app.renderQuickSwitchPopup())
	if !strings.Contains(plain, "⏸ waiting for schema review") {
		t.Fatalf("expected parked snippet in quick-switch popup\n\n%s", plain)
	}
	for _, line := range strings.Split(plain, "\n") {
		if got, want := lipgloss.Width(line), max(app.width-4, 30); got > want {
			t.Fatalf("quick-switch line width = %d, want <= %d\n\n%s", got, want, plain)
		}
	}
}

func TestRenderQuickSwitchPopupParkedSnippetUsesRowWidthPolicy(t *testing.T) {
	renderWithWidth := func(width int) string {
		app := newTestAppForQuickSwitch()
		app.width = width
		app.showQuickSwitch = true
		p := &models.Project{ID: "p1", Name: "Project 1"}
		task := &models.Task{ID: "t1", Title: "Task 1", Status: models.StatusTodo, Type: models.TaskTypeTask}
		parkedAt := time.Now().Add(-time.Hour)
		task.ParkedAt = &parkedAt
		task.ParkedNote = "waiting for schema review before rollout resumes"
		app.recentTaskVisits = []RecentTaskVisitEntry{{Task: task, Project: p}}
		initQuickSwitchFilter(app)
		return ansi.Strip(app.renderQuickSwitchPopup())
	}

	narrow := renderWithWidth(34)
	wide := renderWithWidth(160)

	if strings.Contains(narrow, "schema review") {
		t.Fatalf("narrow quick-switch popup exposed too much parked note\n\n%s", narrow)
	}
	if !strings.Contains(wide, "before rollout") {
		t.Fatalf("wide quick-switch popup did not expose expanded parked note\n\n%s", wide)
	}
	for _, tc := range []struct {
		name       string
		rendered   string
		popupWidth int
	}{
		{name: "narrow", rendered: narrow, popupWidth: 30},
		{name: "wide", rendered: wide, popupWidth: 156},
	} {
		for _, line := range strings.Split(tc.rendered, "\n") {
			if got := lipgloss.Width(line); got > tc.popupWidth {
				t.Fatalf("%s quick-switch line width = %d, want <= %d\n\n%s", tc.name, got, tc.popupWidth, tc.rendered)
			}
		}
	}
}
