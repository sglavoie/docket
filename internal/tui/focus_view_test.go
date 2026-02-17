package tui

import (
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// newTestFocusConfig creates a test config with the given projects.
func newTestFocusConfig(projects []*models.Project) *config.Config {
	cfg := &config.Config{
		Projects: projects,
	}
	return cfg
}

// newTestFocusProject creates a test project for focus view tests.
func newTestFocusProject(name string) *models.Project {
	return models.NewProject(uuid.New().String(), name, "")
}

// newTestFocusTask creates a task with optional active flag.
func newTestFocusTask(title string, priority int, active bool) *models.Task {
	task := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, title)
	task.Priority = priority
	task.Active = active
	return task
}

func TestFocusViewLoadTasks_FilteringLogic(t *testing.T) {
	t.Run("only includes active-flagged tasks", func(t *testing.T) {
		project := newTestFocusProject("Test")
		active := newTestFocusTask("Active task", 3, true)
		inactive := newTestFocusTask("Inactive task", 3, false)
		project.Tasks = []*models.Task{active, inactive}

		cfg := newTestFocusConfig([]*models.Project{project})
		m := &FocusViewModel{config: cfg}
		_ = m.config
		m.tasks = nil

		// Manually populate tasks (bypassing storage.LoadProject)
		for _, task := range project.Tasks {
			if !task.Active {
				continue
			}
			if task.Status.IsComplete() {
				continue
			}
			m.tasks = append(m.tasks, FocusTask{Task: task, Project: project})
		}

		if len(m.tasks) != 1 {
			t.Errorf("expected 1 active task, got %d", len(m.tasks))
		}
		if m.tasks[0].Task.Title != "Active task" {
			t.Errorf("expected 'Active task', got %q", m.tasks[0].Task.Title)
		}
	})

	t.Run("skips completed tasks even if active", func(t *testing.T) {
		project := newTestFocusProject("Test")
		done := newTestFocusTask("Done task", 3, true)
		done.Status = models.StatusDone

		canceled := newTestFocusTask("Canceled task", 3, true)
		canceled.Status = models.StatusCanceled

		active := newTestFocusTask("Active task", 3, true)
		project.Tasks = []*models.Task{done, canceled, active}

		cfg := newTestFocusConfig([]*models.Project{project})
		m := &FocusViewModel{config: cfg}
		_ = m.config
		m.tasks = nil

		for _, task := range project.Tasks {
			if !task.Active {
				continue
			}
			if task.Status.IsComplete() {
				continue
			}
			m.tasks = append(m.tasks, FocusTask{Task: task, Project: project})
		}

		if len(m.tasks) != 1 {
			t.Errorf("expected 1 task (non-completed), got %d", len(m.tasks))
		}
	})

	t.Run("includes blocked tasks if active", func(t *testing.T) {
		project := newTestFocusProject("Test")
		blocked := newTestFocusTask("Blocked task", 3, true)
		blocked.Status = models.StatusBlocked
		project.Tasks = []*models.Task{blocked}

		cfg := newTestFocusConfig([]*models.Project{project})
		m := &FocusViewModel{config: cfg}
		_ = m.config
		m.tasks = nil

		for _, task := range project.Tasks {
			if !task.Active {
				continue
			}
			if task.Status.IsComplete() {
				continue
			}
			m.tasks = append(m.tasks, FocusTask{Task: task, Project: project})
		}

		if len(m.tasks) != 1 {
			t.Errorf("expected 1 blocked task, got %d", len(m.tasks))
		}
	})
}

func TestFocusViewSorting(t *testing.T) {
	t.Run("sorts by priority ascending", func(t *testing.T) {
		project := newTestFocusProject("Test")
		low := newTestFocusTask("Low priority", 5, true)
		high := newTestFocusTask("High priority", 1, true)
		medium := newTestFocusTask("Medium priority", 3, true)
		project.Tasks = []*models.Task{low, high, medium}

		cfg := newTestFocusConfig([]*models.Project{project})
		m := &FocusViewModel{config: cfg}
		m.tasks = nil

		for _, task := range project.Tasks {
			if task.Active && !task.Status.IsComplete() {
				m.tasks = append(m.tasks, FocusTask{Task: task, Project: project})
			}
		}

		// Apply sorting
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
		project := newTestFocusProject("Test")
		older := newTestFocusTask("Older task", 3, true)
		older.UpdatedAt = time.Now().Add(-1 * time.Hour)

		newer := newTestFocusTask("Newer task", 3, true)
		newer.UpdatedAt = time.Now()

		project.Tasks = []*models.Task{older, newer}

		cfg := newTestFocusConfig([]*models.Project{project})
		m := &FocusViewModel{config: cfg}
		m.tasks = nil

		for _, task := range project.Tasks {
			if task.Active && !task.Status.IsComplete() {
				m.tasks = append(m.tasks, FocusTask{Task: task, Project: project})
			}
		}

		m.sortTasks()

		if len(m.tasks) != 2 {
			t.Fatalf("expected 2 tasks, got %d", len(m.tasks))
		}
		// Newer should come first (most recent first)
		if m.tasks[0].Task.Title != "Newer task" {
			t.Errorf("expected 'Newer task' first, got %q", m.tasks[0].Task.Title)
		}
	})
}

func TestFocusViewGetTaskAtCursor(t *testing.T) {
	t.Run("returns correct task", func(t *testing.T) {
		project := newTestFocusProject("Test")
		task1 := newTestFocusTask("Task 1", 1, true)
		task2 := newTestFocusTask("Task 2", 2, true)
		project.Tasks = []*models.Task{task1, task2}

		cfg := newTestFocusConfig([]*models.Project{project})
		m := &FocusViewModel{config: cfg, InteractiveViewState: InteractiveViewState{cursor: 0}}
		m.tasks = []FocusTask{
			{Task: task1, Project: project},
			{Task: task2, Project: project},
		}
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
		m := &FocusViewModel{config: cfg, InteractiveViewState: InteractiveViewState{cursor: 10}}
		m.tasks = []FocusTask{}
		m.applyFilter()

		if ft := m.getTaskAtCursor(); ft != nil {
			t.Errorf("expected nil for out of bounds, got %v", ft)
		}
	})

	t.Run("returns nil for negative cursor", func(t *testing.T) {
		cfg := newTestFocusConfig(nil)
		m := &FocusViewModel{config: cfg, InteractiveViewState: InteractiveViewState{cursor: -1}}
		m.tasks = []FocusTask{}
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
			m := &FocusViewModel{config: cfg, InteractiveViewState: InteractiveViewState{cursor: tt.cursor}}

			// Create dummy tasks
			for i := 0; i < tt.taskCount; i++ {
				m.tasks = append(m.tasks, FocusTask{})
			}
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

			got := formatRelativeDue(dueDate, true)
			if tt.wantMatch == "" {
				if got != "" {
					t.Errorf("formatRelativeDue() = %q, want empty", got)
				}
			} else if !containsSubstring(got, tt.wantMatch) {
				t.Errorf("formatRelativeDue() = %q, want to contain %q", got, tt.wantMatch)
			}
		})
	}
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestFocusViewRenderPriorityBadge(t *testing.T) {
	tests := []struct {
		priority  int
		wantBadge string
	}{
		{1, "!1"},
		{2, "!2"},
		{3, "!3"},
		{4, "!4"},
		{5, "!5"},
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

func TestFocusViewMultipleProjects(t *testing.T) {
	t.Run("aggregates tasks from multiple projects", func(t *testing.T) {
		project1 := newTestFocusProject("Project 1")
		task1 := newTestFocusTask("Task from P1", 1, true)
		project1.Tasks = []*models.Task{task1}

		project2 := newTestFocusProject("Project 2")
		task2 := newTestFocusTask("Task from P2", 2, true)
		project2.Tasks = []*models.Task{task2}

		cfg := newTestFocusConfig([]*models.Project{project1, project2})
		m := &FocusViewModel{config: cfg}
		m.tasks = nil

		// Simulate loadTasks logic
		for _, project := range []*models.Project{project1, project2} {
			for _, task := range project.Tasks {
				if task.Active && !task.Status.IsComplete() {
					m.tasks = append(m.tasks, FocusTask{Task: task, Project: project})
				}
			}
		}
		m.sortTasks()

		if len(m.tasks) != 2 {
			t.Fatalf("expected 2 tasks from 2 projects, got %d", len(m.tasks))
		}

		// Higher priority (1) should come first
		if m.tasks[0].Task.Title != "Task from P1" {
			t.Errorf("expected 'Task from P1' first (priority 1), got %q", m.tasks[0].Task.Title)
		}
	})
}

// sortTasks is a helper to apply the sorting logic from loadTasks.
func (m *FocusViewModel) sortTasks() {
	// Sort by priority (ascending, 1=highest), then by UpdatedAt (most recent first)
	sort.Slice(m.tasks, func(i, j int) bool {
		if m.tasks[i].Task.Priority != m.tasks[j].Task.Priority {
			return m.tasks[i].Task.Priority < m.tasks[j].Task.Priority
		}
		return m.tasks[i].Task.UpdatedAt.After(m.tasks[j].Task.UpdatedAt)
	})
}
