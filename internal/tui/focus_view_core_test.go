package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestFocusViewSorting(t *testing.T) {
	t.Run("sorts by priority ascending", func(t *testing.T) {
		project := newTestFocusProject(t, "Test")
		low := newTestFocusTask(t, "Low priority", 5, true)
		high := newTestFocusTask(t, "High priority", 1, true)
		medium := newTestFocusTask(t, "Medium priority", 3, true)
		project.Tasks = []*models.Task{low, high, medium}

		cfg := newTestFocusConfig([]*models.Project{project})
		m := newTestFocusViewModel(cfg)

		for _, task := range project.Tasks {
			if !task.Status.IsComplete() {
				m.tasks = append(m.tasks, FocusTask{Task: task, Project: project})
			}
		}

		m.sortTasks()

		if len(m.tasks) != 3 {
			t.Fatalf("expected 3 tasks, got %d", len(m.tasks))
		}
		if m.tasks[0].Task.Priority != 1 {
			t.Errorf("expected first task priority 1, got %d", m.tasks[0].Task.Priority)
		}
		if m.tasks[1].Task.Priority != 3 {
			t.Errorf("expected second task priority 3, got %d", m.tasks[1].Task.Priority)
		}
		if m.tasks[2].Task.Priority != 5 {
			t.Errorf("expected third task priority 5, got %d", m.tasks[2].Task.Priority)
		}
	})

	t.Run("sorts by UpdatedAt within same priority", func(t *testing.T) {
		project := newTestFocusProject(t, "Test")
		older := newTestFocusTask(t, "Older task", 3, true)
		older.UpdatedAt = time.Now().Add(-1 * time.Hour)

		newer := newTestFocusTask(t, "Newer task", 3, true)
		newer.UpdatedAt = time.Now()

		project.Tasks = []*models.Task{older, newer}

		cfg := newTestFocusConfig([]*models.Project{project})
		m := newTestFocusViewModel(cfg)

		for _, task := range project.Tasks {
			if !task.Status.IsComplete() {
				m.tasks = append(m.tasks, FocusTask{Task: task, Project: project})
			}
		}

		m.sortTasks()

		if len(m.tasks) != 2 {
			t.Fatalf("expected 2 tasks, got %d", len(m.tasks))
		}
		if m.tasks[0].Task.Title != "Newer task" {
			t.Errorf("expected 'Newer task' first, got %q", m.tasks[0].Task.Title)
		}
	})
}

func TestFocusViewGetTaskAtCursor(t *testing.T) {
	t.Run("returns correct task", func(t *testing.T) {
		project := newTestFocusProject(t, "Test")
		task1 := newTestFocusTask(t, "Task 1", 1, true)
		task2 := newTestFocusTask(t, "Task 2", 2, true)
		project.Tasks = []*models.Task{task1, task2}

		cfg := newTestFocusConfig([]*models.Project{project})
		m := newTestFocusViewModel(cfg)
		m.cursor = 0
		m.tasks = []FocusTask{
			{Task: task1, Project: project},
			{Task: task2, Project: project},
		}
		m.tabs.Active = focusTabByProject
		m.applyFilter()

		ft := m.getTaskAtCursor()
		if ft == nil || ft.Task.Title != "Task 1" {
			t.Errorf("cursor 0: expected 'Task 1', got %v", ft)
		}

		m.cursor = 1
		ft = m.getTaskAtCursor()
		if ft == nil || ft.Task.Title != "Task 2" {
			t.Errorf("cursor 1: expected 'Task 2', got %v", ft)
		}
	})

	t.Run("returns nil for out of bounds cursor", func(t *testing.T) {
		cfg := newTestFocusConfig(nil)
		m := newTestFocusViewModel(cfg)
		m.cursor = 10
		m.tasks = []FocusTask{}
		m.tabs.Active = focusTabByProject
		m.applyFilter()

		if ft := m.getTaskAtCursor(); ft != nil {
			t.Errorf("expected nil for out of bounds, got %v", ft)
		}
	})

	t.Run("returns nil for negative cursor", func(t *testing.T) {
		cfg := newTestFocusConfig(nil)
		m := newTestFocusViewModel(cfg)
		m.cursor = -1
		m.tasks = []FocusTask{}
		m.tabs.Active = focusTabByProject
		m.applyFilter()

		if ft := m.getTaskAtCursor(); ft != nil {
			t.Errorf("expected nil for negative cursor, got %v", ft)
		}
	})
}

func TestFocusViewClampCursor(t *testing.T) {
	tests := []struct {
		name       string
		taskCount  int
		cursor     int
		wantCursor int
	}{
		{"cursor exceeds total", 5, 10, 4},
		{"cursor at total", 5, 5, 4},
		{"cursor negative", 5, -1, 0},
		{"cursor within bounds", 5, 2, 2},
		{"empty list", 0, 5, 0},
		{"empty list negative", 0, -1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newTestFocusConfig(nil)
			m := newTestFocusViewModel(cfg)
			m.cursor = tt.cursor

			dummyProject := newTestFocusProject(t, "Dummy")
			for i := 0; i < tt.taskCount; i++ {
				m.tasks = append(m.tasks, FocusTask{
					Task:    newTestFocusTask(t, "dummy", 3, false),
					Project: dummyProject,
				})
			}
			m.tabs.Active = focusTabByProject
			m.applyFilter()

			m.clampCursor()

			if m.cursor != tt.wantCursor {
				t.Errorf("clampCursor() cursor = %d, want %d", m.cursor, tt.wantCursor)
			}
		})
	}
}

func TestFocusViewHasPendingPrefix(t *testing.T) {
	m := &FocusViewModel{}

	if m.HasPendingPrefix() {
		t.Error("HasPendingPrefix() should return false initially")
	}

	m.pendingPrefix = ","
	if !m.HasPendingPrefix() {
		t.Error("HasPendingPrefix() should return true with pending prefix")
	}

	m.pendingPrefix = ""
	if m.HasPendingPrefix() {
		t.Error("HasPendingPrefix() should return false after clearing")
	}
}

func TestFocusViewSetSize(t *testing.T) {
	m := &FocusViewModel{}

	m.SetSize(100, 50)

	if m.width != 100 {
		t.Errorf("width = %d, want 100", m.width)
	}
	if m.height != 50 {
		t.Errorf("height = %d, want 50", m.height)
	}
}

func TestFocusViewFormatRelativeDue(t *testing.T) {
	tests := []struct {
		name       string
		daysOffset *int
		wantMatch  string
	}{
		{"nil due date", nil, ""},
		{"yesterday", intPtr(-1), "yesterday"},
		{"2 days ago", intPtr(-2), "days ago"},
		{"today", intPtr(0), "today"},
		{"tomorrow", intPtr(1), "tomorrow"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dueDate *time.Time
			if tt.daysOffset != nil {
				due := time.Now().AddDate(0, 0, *tt.daysOffset)
				dueDate = &due
			}

			got := FormatRelativeDue(dueDate, true)
			if tt.wantMatch == "" {
				if got != "" {
					t.Errorf("FormatRelativeDue() = %q, want empty", got)
				}
			} else if !containsSubstring(got, tt.wantMatch) {
				t.Errorf("FormatRelativeDue() = %q, want to contain %q", got, tt.wantMatch)
			}
		})
	}
}

func TestFocusViewRenderPriorityBadge(t *testing.T) {
	tests := []struct {
		priority  int
		wantBadge string
	}{
		{1, PriorityIcon1},
		{2, PriorityIcon2},
		{3, PriorityIcon3},
		{4, PriorityIcon4},
		{5, PriorityIcon5},
	}

	for _, tt := range tests {
		t.Run(tt.wantBadge, func(t *testing.T) {
			got := renderPriorityBadge(tt.priority)
			if !containsSubstring(got, tt.wantBadge) {
				t.Errorf("renderPriorityBadge(%d) = %q, want to contain %q", tt.priority, got, tt.wantBadge)
			}
		})
	}
}

func TestFocusViewName(t *testing.T) {
	m := &FocusViewModel{}
	if m.viewName() != "Backlog" {
		t.Errorf("expected 'Backlog', got %q", m.viewName())
	}
}

func TestDueBucket(t *testing.T) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	m := &FocusViewModel{}

	tests := []struct {
		name     string
		dueDate  time.Time
		expected string
	}{
		{"overdue by 3 days", today.AddDate(0, 0, -3), "Overdue"},
		{"overdue by 1 day", today.AddDate(0, 0, -1), "Overdue"},
		{"today", today, "Today"},
		{"tomorrow", today.AddDate(0, 0, 1), "Tomorrow"},
		{"in 3 days", today.AddDate(0, 0, 3), "This Week"},
		{"in 7 days", today.AddDate(0, 0, 7), "This Week"},
		{"in 8 days", today.AddDate(0, 0, 8), "Later"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := newTestFocusTask(t, "test", 3, false)
			task.DueDate = &tt.dueDate
			got := m.dueBucket(task, now, today)
			if got != tt.expected {
				t.Errorf("dueBucket() = %q, want %q", got, tt.expected)
			}
		})
	}

	t.Run("nil due date returns Later", func(t *testing.T) {
		task := newTestFocusTask(t, "no due", 3, false)
		got := m.dueBucket(task, now, today)
		if got != "Later" {
			t.Errorf("dueBucket(nil) = %q, want Later", got)
		}
	})
}
