package tui

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
)

// Helper to create a task with a due date relative to today.
func newTestTaskWithDueDate(t *testing.T, title string, daysOffset int) *models.Task {
	t.Helper()
	task, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, title)
	if err != nil {
		t.Fatal(err)
	}
	due := time.Now().AddDate(0, 0, daysOffset)
	task.DueDate = &due
	return task
}

// Helper to create a test project for priority view tests.
func newTestPriorityProject(t *testing.T) *models.Project {
	t.Helper()
	project, err := models.NewProject(uuid.New().String(), "Priority Test", "")
	if err != nil {
		t.Fatal(err)
	}
	return project
}

func TestComputeUrgencyScore(t *testing.T) {
	project := newTestPriorityProject(t)
	m := NewPriorityViewModel(project, nil, nil)

	tests := []struct {
		name    string
		task    *models.Task
		wantMin int // minimum expected score
		wantMax int // maximum expected score
	}{
		{
			name: "priority 1 base score",
			task: func() *models.Task {
				task, err := models.NewTask("1", 1, models.TaskTypeTask, "High priority")
				if err != nil {
					t.Fatal(err)
				}
				task.Priority = 1
				return task
			}(),
			wantMin: 5000,
			wantMax: 5000,
		},
		{
			name: "priority 5 base score",
			task: func() *models.Task {
				task, err := models.NewTask("2", 2, models.TaskTypeTask, "Low priority")
				if err != nil {
					t.Fatal(err)
				}
				task.Priority = 5
				return task
			}(),
			wantMin: 1000,
			wantMax: 1000,
		},
		{
			name: "priority 3 base score",
			task: func() *models.Task {
				task, err := models.NewTask("3", 3, models.TaskTypeTask, "Medium priority")
				if err != nil {
					t.Fatal(err)
				}
				task.Priority = 3
				return task
			}(),
			wantMin: 3000,
			wantMax: 3000,
		},
		{
			name: "in progress bonus",
			task: func() *models.Task {
				task, err := models.NewTask("4", 4, models.TaskTypeTask, "In progress task")
				if err != nil {
					t.Fatal(err)
				}
				task.Priority = 3
				task.Status = models.StatusInProgress
				return task
			}(),
			wantMin: 3500, // 3000 + 500
			wantMax: 3500,
		},
		{
			name: "active flag bonus",
			task: func() *models.Task {
				task, err := models.NewTask("5", 5, models.TaskTypeTask, "Active task")
				if err != nil {
					t.Fatal(err)
				}
				task.Priority = 3
				task.Active = true
				return task
			}(),
			wantMin: 3200, // 3000 + 200
			wantMax: 3200,
		},
		{
			name: "combined bonuses",
			task: func() *models.Task {
				task, err := models.NewTask("6", 6, models.TaskTypeTask, "Active in progress")
				if err != nil {
					t.Fatal(err)
				}
				task.Priority = 1
				task.Status = models.StatusInProgress
				task.Active = true
				return task
			}(),
			wantMin: 5700, // 5000 + 500 + 200
			wantMax: 5700,
		},
		{
			name: "bug type bonus",
			task: func() *models.Task {
				task, err := models.NewTask("7", 7, models.TaskTypeBug, "Bug task")
				if err != nil {
					t.Fatal(err)
				}
				task.Priority = 3
				return task
			}(),
			wantMin: 3100, // 3000 + 100 (bug bonus)
			wantMax: 3100,
		},
		{
			name: "bug outranks task at same priority",
			task: func() *models.Task {
				task, err := models.NewTask("8", 8, models.TaskTypeBug, "Bug at priority 3")
				if err != nil {
					t.Fatal(err)
				}
				task.Priority = 3
				return task
			}(),
			wantMin: 3100, // Bug: 3000 + 100 > Task: 3000
			wantMax: 3100,
		},
		{
			name: "overdue adds points",
			task: func() *models.Task {
				task := newTestTaskWithDueDate(t, "Overdue task", -2)
				task.Priority = 3
				return task
			}(),
			wantMin: 3100, // 3000 + 100 (2 days * 50)
			wantMax: 3100,
		},
		{
			name: "future due subtracts points",
			task: func() *models.Task {
				task := newTestTaskWithDueDate(t, "Future task", 2)
				task.Priority = 3
				return task
			}(),
			wantMin: 2900, // 3000 - 100 (2 days * 50)
			wantMax: 2900,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.computeUrgencyScore(tt.task)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("computeUrgencyScore() = %d, want between %d and %d", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestCategorizeByDueDate(t *testing.T) {
	project := newTestPriorityProject(t)
	m := NewPriorityViewModel(project, nil, nil)

	tests := []struct {
		name       string
		daysOffset *int // nil means no due date
		want       PriorityGroup
	}{
		{"overdue yesterday", intPtr(-1), PriorityGroupOverdue},
		{"overdue 10 days ago", intPtr(-10), PriorityGroupOverdue},
		{"due today", intPtr(0), PriorityGroupToday},
		{"due tomorrow", intPtr(1), PriorityGroupThisWeek},
		{"due in 3 days", intPtr(3), PriorityGroupThisWeek},
		{"due in 6 days", intPtr(6), PriorityGroupThisWeek},
		{"due in 7 days", intPtr(7), PriorityGroupLater},
		{"due in 14 days", intPtr(14), PriorityGroupLater},
		{"no due date", nil, PriorityGroupBacklog},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
			if err != nil {
				t.Fatal(err)
			}
			if tt.daysOffset != nil {
				due := time.Now().AddDate(0, 0, *tt.daysOffset)
				task.DueDate = &due
			}
			got := m.categorizeByDueDate(task)
			if got != tt.want {
				t.Errorf("categorizeByDueDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func intPtr(i int) *int {
	return &i
}

func TestLoadActionableTasks(t *testing.T) { //nolint:gocyclo
	t.Run("filters out completed tasks", func(t *testing.T) {
		project := newTestPriorityProject(t)

		// Add completed tasks
		done, err := models.NewTask("1", 1, models.TaskTypeTask, "Done task")
		if err != nil {
			t.Fatal(err)
		}
		done.Status = models.StatusDone
		canceled, err := models.NewTask("2", 2, models.TaskTypeTask, "Canceled task")
		if err != nil {
			t.Fatal(err)
		}
		canceled.Status = models.StatusCanceled

		// Add active task
		active, err := models.NewTask("3", 3, models.TaskTypeTask, "Active task")
		if err != nil {
			t.Fatal(err)
		}
		active.Status = models.StatusTodo

		project.Tasks = []*models.Task{done, canceled, active}

		m := NewPriorityViewModel(project, nil, nil)

		if m.totalTasks != 1 {
			t.Errorf("expected 1 actionable task, got %d", m.totalTasks)
		}
	})

	t.Run("separates blocked tasks to waitingOn", func(t *testing.T) {
		project := newTestPriorityProject(t)

		// Add blocked task
		blocked, err := models.NewTask("1", 1, models.TaskTypeTask, "Blocked task")
		if err != nil {
			t.Fatal(err)
		}
		blocked.Status = models.StatusBlocked

		// Add active task
		active, err := models.NewTask("2", 2, models.TaskTypeTask, "Active task")
		if err != nil {
			t.Fatal(err)
		}
		active.Status = models.StatusTodo

		project.Tasks = []*models.Task{blocked, active}

		m := NewPriorityViewModel(project, nil, nil)

		if m.totalTasks != 1 {
			t.Errorf("expected 1 actionable task, got %d", m.totalTasks)
		}
		if len(m.waitingOn) != 1 {
			t.Errorf("expected 1 waiting task, got %d", len(m.waitingOn))
		}
	})

	t.Run("tasks with incomplete dependencies go to waitingOn", func(t *testing.T) {
		project := newTestPriorityProject(t)

		// Add blocker task
		blocker, err := models.NewTask("1", 1, models.TaskTypeTask, "Blocker")
		if err != nil {
			t.Fatal(err)
		}
		blocker.Status = models.StatusTodo

		// Add dependent task
		dependent, err := models.NewTask("2", 2, models.TaskTypeTask, "Dependent")
		if err != nil {
			t.Fatal(err)
		}
		dependent.Status = models.StatusTodo
		dependent.DependsOn = []string{blocker.ID}

		project.Tasks = []*models.Task{blocker, dependent}

		m := NewPriorityViewModel(project, nil, nil)

		// Blocker should be actionable, dependent should be waiting
		if m.totalTasks != 1 {
			t.Errorf("expected 1 actionable task, got %d", m.totalTasks)
		}
		if len(m.waitingOn) != 1 {
			t.Errorf("expected 1 waiting task, got %d", len(m.waitingOn))
		}
	})

	t.Run("sorts tasks by urgency within groups", func(t *testing.T) {
		project := newTestPriorityProject(t)

		// Add tasks with different priorities (no due dates -> backlog)
		low, err := models.NewTask("1", 1, models.TaskTypeTask, "Low priority")
		if err != nil {
			t.Fatal(err)
		}
		low.Priority = 5

		high, err := models.NewTask("2", 2, models.TaskTypeTask, "High priority")
		if err != nil {
			t.Fatal(err)
		}
		high.Priority = 1

		medium, err := models.NewTask("3", 3, models.TaskTypeTask, "Medium priority")
		if err != nil {
			t.Fatal(err)
		}
		medium.Priority = 3

		project.Tasks = []*models.Task{low, high, medium}

		m := NewPriorityViewModel(project, nil, nil)

		if len(m.backlog) != 3 {
			t.Fatalf("expected 3 backlog tasks, got %d", len(m.backlog))
		}

		// Should be sorted high to low urgency
		if m.backlog[0].Task.Priority != 1 {
			t.Errorf("expected first task to have priority 1, got %d", m.backlog[0].Task.Priority)
		}
		if m.backlog[1].Task.Priority != 3 {
			t.Errorf("expected second task to have priority 3, got %d", m.backlog[1].Task.Priority)
		}
		if m.backlog[2].Task.Priority != 5 {
			t.Errorf("expected third task to have priority 5, got %d", m.backlog[2].Task.Priority)
		}
	})

	t.Run("bugs rank higher than tasks at same priority", func(t *testing.T) {
		project := newTestPriorityProject(t)

		// Add a task and a bug with the same priority (no due dates -> backlog)
		task, err := models.NewTask("1", 1, models.TaskTypeTask, "Regular task")
		if err != nil {
			t.Fatal(err)
		}
		task.Priority = 3

		bug, err := models.NewTask("2", 2, models.TaskTypeBug, "Bug task")
		if err != nil {
			t.Fatal(err)
		}
		bug.Priority = 3

		// Add task first to ensure sorting changes the order
		project.Tasks = []*models.Task{task, bug}

		m := NewPriorityViewModel(project, nil, nil)

		if len(m.backlog) != 2 {
			t.Fatalf("expected 2 backlog tasks, got %d", len(m.backlog))
		}

		// Bug should appear first due to +100 bonus
		if m.backlog[0].Task.Type != models.TaskTypeBug {
			t.Errorf("expected bug to be first, got %s", m.backlog[0].Task.Type)
		}
		if m.backlog[1].Task.Type != models.TaskTypeTask {
			t.Errorf("expected task to be second, got %s", m.backlog[1].Task.Type)
		}
	})

	t.Run("higher priority task still outranks lower priority bug", func(t *testing.T) {
		project := newTestPriorityProject(t)

		// Priority 2 task should beat priority 3 bug
		task, err := models.NewTask("1", 1, models.TaskTypeTask, "Priority 2 task")
		if err != nil {
			t.Fatal(err)
		}
		task.Priority = 2

		bug, err := models.NewTask("2", 2, models.TaskTypeBug, "Priority 3 bug")
		if err != nil {
			t.Fatal(err)
		}
		bug.Priority = 3

		project.Tasks = []*models.Task{bug, task}

		m := NewPriorityViewModel(project, nil, nil)

		if len(m.backlog) != 2 {
			t.Fatalf("expected 2 backlog tasks, got %d", len(m.backlog))
		}

		// Priority 2 task (4000 points) should outrank priority 3 bug (3100 points)
		if m.backlog[0].Task.Priority != 2 {
			t.Errorf("expected priority 2 task to be first, got priority %d", m.backlog[0].Task.Priority)
		}
		if m.backlog[1].Task.Priority != 3 {
			t.Errorf("expected priority 3 bug to be second, got priority %d", m.backlog[1].Task.Priority)
		}
	})

	t.Run("totalTasks excludes waitingOn", func(t *testing.T) {
		project := newTestPriorityProject(t)

		// Add 2 active tasks
		active1, err := models.NewTask("1", 1, models.TaskTypeTask, "Active 1")
		if err != nil {
			t.Fatal(err)
		}
		active2, err := models.NewTask("2", 2, models.TaskTypeTask, "Active 2")
		if err != nil {
			t.Fatal(err)
		}

		// Add 1 blocked task
		blocked, err := models.NewTask("3", 3, models.TaskTypeTask, "Blocked")
		if err != nil {
			t.Fatal(err)
		}
		blocked.Status = models.StatusBlocked

		project.Tasks = []*models.Task{active1, active2, blocked}

		m := NewPriorityViewModel(project, nil, nil)

		if m.totalTasks != 2 {
			t.Errorf("expected totalTasks=2 (excluding waitingOn), got %d", m.totalTasks)
		}
	})
}

func TestGetTaskAtCursor(t *testing.T) {
	t.Run("returns correct task across groups", func(t *testing.T) {
		project := newTestPriorityProject(t)

		// Add tasks to different groups
		overdue := newTestTaskWithDueDate(t, "Overdue", -1)
		today := newTestTaskWithDueDate(t, "Today", 0)
		thisWeek := newTestTaskWithDueDate(t, "This week", 3)

		project.Tasks = []*models.Task{overdue, today, thisWeek}

		m := NewPriorityViewModel(project, nil, nil)

		// Cursor 0 -> overdue[0]
		m.cursor = 0
		task := m.getTaskAtCursor()
		if task == nil || task.Task.Title != "Overdue" {
			t.Errorf("cursor 0: expected 'Overdue', got %v", task)
		}

		// Cursor 1 -> today[0]
		m.cursor = 1
		task = m.getTaskAtCursor()
		if task == nil || task.Task.Title != "Today" {
			t.Errorf("cursor 1: expected 'Today', got %v", task)
		}

		// Cursor 2 -> thisWeek[0]
		m.cursor = 2
		task = m.getTaskAtCursor()
		if task == nil || task.Task.Title != "This week" {
			t.Errorf("cursor 2: expected 'This week', got %v", task)
		}
	})

	t.Run("returns nil for out of bounds cursor", func(t *testing.T) {
		project := newTestPriorityProject(t)
		task, err := models.NewTask("1", 1, models.TaskTypeTask, "Only task")
		if err != nil {
			t.Fatal(err)
		}
		project.Tasks = []*models.Task{task}

		m := NewPriorityViewModel(project, nil, nil)

		m.cursor = 10
		if got := m.getTaskAtCursor(); got != nil {
			t.Errorf("expected nil for out of bounds cursor, got %v", got)
		}
	})

	t.Run("returns nil for empty project", func(t *testing.T) {
		project := newTestPriorityProject(t)
		m := NewPriorityViewModel(project, nil, nil)

		if got := m.getTaskAtCursor(); got != nil {
			t.Errorf("expected nil for empty project, got %v", got)
		}
	})
}

func TestPriorityViewClampCursor(t *testing.T) {
	tests := []struct {
		name       string
		totalTasks int
		cursor     int
		wantCursor int
	}{
		{"cursor exceeds total", 5, 10, 4},
		{"cursor at total", 5, 5, 4},
		{"cursor negative", 5, -1, 0},
		{"cursor within bounds", 5, 2, 2},
		{"empty project", 0, 5, 0},
		{"empty project negative", 0, -1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := newTestPriorityProject(t)

			// Add tasks to match totalTasks
			for i := 0; i < tt.totalTasks; i++ {
				task, err := models.NewTask(uuid.New().String(), i+1, models.TaskTypeTask, "Task")
				if err != nil {
					t.Fatal(err)
				}
				project.Tasks = append(project.Tasks, task)
			}

			m := NewPriorityViewModel(project, nil, nil)
			m.cursor = tt.cursor
			m.clampCursor()

			if m.cursor != tt.wantCursor {
				t.Errorf("clampCursor() cursor = %d, want %d", m.cursor, tt.wantCursor)
			}
		})
	}
}

func TestFormatRelativeDue(t *testing.T) {
	tests := []struct {
		name       string
		daysOffset *int
		wantMatch  string // partial match expected
	}{
		{"nil due date", nil, ""},
		{"yesterday", intPtr(-1), "yesterday"},
		{"2 days ago", intPtr(-2), "days ago"},
		{"10 days ago", intPtr(-10), "days ago"},
		{"today", intPtr(0), "today"},
		{"tomorrow", intPtr(1), "tomorrow"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
			if err != nil {
				t.Fatal(err)
			}
			if tt.daysOffset != nil {
				due := time.Now().AddDate(0, 0, *tt.daysOffset)
				task.DueDate = &due
			}

			got := FormatRelativeDue(task.DueDate, false)
			if tt.wantMatch == "" {
				if got != "" {
					t.Errorf("FormatRelativeDue() = %q, want empty", got)
				}
			} else if got != tt.wantMatch && !contains(got, tt.wantMatch) {
				t.Errorf("FormatRelativeDue() = %q, want to contain %q", got, tt.wantMatch)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestGetParentTitle(t *testing.T) {
	t.Run("no parent returns empty", func(t *testing.T) {
		project := newTestPriorityProject(t)
		task, err := models.NewTask("1", 1, models.TaskTypeTask, "Orphan task")
		if err != nil {
			t.Fatal(err)
		}
		project.Tasks = []*models.Task{task}

		m := NewPriorityViewModel(project, nil, nil)
		got := m.getParentTitle(task)

		if got != "" {
			t.Errorf("getParentTitle() = %q, want empty", got)
		}
	})

	t.Run("valid parent returns title", func(t *testing.T) {
		project := newTestPriorityProject(t)
		parent, err := models.NewTask("1", 1, models.TaskTypeEpic, "Parent Epic")
		if err != nil {
			t.Fatal(err)
		}
		child, err := models.NewTask("2", 2, models.TaskTypeTask, "Child Task")
		if err != nil {
			t.Fatal(err)
		}
		child.ParentID = &parent.ID

		project.Tasks = []*models.Task{parent, child}
		project.BuildTaskTree()

		m := NewPriorityViewModel(project, nil, nil)
		got := m.getParentTitle(child)

		if got != "Parent Epic" {
			t.Errorf("getParentTitle() = %q, want 'Parent Epic'", got)
		}
	})

	t.Run("missing parent returns empty", func(t *testing.T) {
		project := newTestPriorityProject(t)
		child, err := models.NewTask("1", 1, models.TaskTypeTask, "Orphan with invalid parent")
		if err != nil {
			t.Fatal(err)
		}
		invalidID := "non-existent-id"
		child.ParentID = &invalidID

		project.Tasks = []*models.Task{child}

		m := NewPriorityViewModel(project, nil, nil)
		got := m.getParentTitle(child)

		if got != "" {
			t.Errorf("getParentTitle() = %q, want empty", got)
		}
	})
}

func TestGetBlockingReason(t *testing.T) {
	t.Run("explicitly blocked returns blocked", func(t *testing.T) {
		project := newTestPriorityProject(t)
		task, err := models.NewTask("1", 1, models.TaskTypeTask, "Blocked task")
		if err != nil {
			t.Fatal(err)
		}
		task.Status = models.StatusBlocked

		project.Tasks = []*models.Task{task}

		m := NewPriorityViewModel(project, nil, nil)
		got := m.getBlockingReason(task)

		if got != "(blocked)" {
			t.Errorf("getBlockingReason() = %q, want '(blocked)'", got)
		}
	})

	t.Run("incomplete dependency shows dependency title", func(t *testing.T) {
		project := newTestPriorityProject(t)
		blocker, err := models.NewTask("1", 1, models.TaskTypeTask, "Blocker Task")
		if err != nil {
			t.Fatal(err)
		}
		blocker.Status = models.StatusTodo

		dependent, err := models.NewTask("2", 2, models.TaskTypeTask, "Dependent")
		if err != nil {
			t.Fatal(err)
		}
		dependent.DependsOn = []string{blocker.ID}

		project.Tasks = []*models.Task{blocker, dependent}

		m := NewPriorityViewModel(project, nil, nil)
		got := m.getBlockingReason(dependent)

		if got != "← Blocker Task" {
			t.Errorf("getBlockingReason() = %q, want '← Blocker Task'", got)
		}
	})
}

func TestEdgeCases(t *testing.T) {
	t.Run("empty project", func(t *testing.T) {
		project := newTestPriorityProject(t)
		m := NewPriorityViewModel(project, nil, nil)

		if m.totalTasks != 0 {
			t.Errorf("expected 0 total tasks for empty project, got %d", m.totalTasks)
		}
		if len(m.overdue) != 0 || len(m.today) != 0 || len(m.thisWeek) != 0 ||
			len(m.later) != 0 || len(m.backlog) != 0 || len(m.waitingOn) != 0 {
			t.Error("expected all groups to be empty for empty project")
		}
	})

	t.Run("all tasks completed", func(t *testing.T) {
		project := newTestPriorityProject(t)

		done1, err := models.NewTask("1", 1, models.TaskTypeTask, "Done 1")
		if err != nil {
			t.Fatal(err)
		}
		done1.Status = models.StatusDone
		done2, err := models.NewTask("2", 2, models.TaskTypeTask, "Done 2")
		if err != nil {
			t.Fatal(err)
		}
		done2.Status = models.StatusCanceled

		project.Tasks = []*models.Task{done1, done2}

		m := NewPriorityViewModel(project, nil, nil)

		if m.totalTasks != 0 {
			t.Errorf("expected 0 total tasks when all completed, got %d", m.totalTasks)
		}
	})

	t.Run("all tasks blocked", func(t *testing.T) {
		project := newTestPriorityProject(t)

		blocked1, err := models.NewTask("1", 1, models.TaskTypeTask, "Blocked 1")
		if err != nil {
			t.Fatal(err)
		}
		blocked1.Status = models.StatusBlocked
		blocked2, err := models.NewTask("2", 2, models.TaskTypeTask, "Blocked 2")
		if err != nil {
			t.Fatal(err)
		}
		blocked2.Status = models.StatusBlocked

		project.Tasks = []*models.Task{blocked1, blocked2}

		m := NewPriorityViewModel(project, nil, nil)

		if m.totalTasks != 0 {
			t.Errorf("expected 0 actionable tasks when all blocked, got %d", m.totalTasks)
		}
		if len(m.waitingOn) != 2 {
			t.Errorf("expected 2 waiting tasks, got %d", len(m.waitingOn))
		}
	})
}

func TestHasActiveSearch(t *testing.T) {
	project := newTestPriorityProject(t)
	m := NewPriorityViewModel(project, nil, nil)

	if m.HasActiveSearch() {
		t.Error("HasActiveSearch() should return false for priority view")
	}
}

func TestWaitingOnCollapsed(t *testing.T) {
	project := newTestPriorityProject(t)
	m := NewPriorityViewModel(project, nil, nil)

	// Should start collapsed
	if !m.waitingOnCollapsed {
		t.Error("waitingOnCollapsed should be true by default")
	}

	// Toggle
	m.waitingOnCollapsed = false
	if m.waitingOnCollapsed {
		t.Error("waitingOnCollapsed should be false after toggle")
	}
}

func TestChangeTaskPriority(t *testing.T) {
	t.Run("changes priority of selected task", func(t *testing.T) {
		project := newTestPriorityProject(t)
		task, err := models.NewTask("1", 1, models.TaskTypeTask, "Test task")
		if err != nil {
			t.Fatal(err)
		}
		task.Priority = 3 // Default medium priority
		project.Tasks = []*models.Task{task}

		m := NewPriorityViewModel(project, nil, nil)
		m.cursor = 0

		// Change priority to 1 (highest)
		m.changeTaskPriority(1)

		if task.Priority != 1 {
			t.Errorf("expected priority 1, got %d", task.Priority)
		}
	})

	t.Run("does nothing when priority unchanged", func(t *testing.T) {
		project := newTestPriorityProject(t)
		task, err := models.NewTask("1", 1, models.TaskTypeTask, "Test task")
		if err != nil {
			t.Fatal(err)
		}
		task.Priority = 2
		originalUpdatedAt := task.UpdatedAt
		project.Tasks = []*models.Task{task}

		m := NewPriorityViewModel(project, nil, nil)
		m.cursor = 0

		// Try to set same priority
		m.changeTaskPriority(2)

		if task.Priority != 2 {
			t.Errorf("expected priority 2, got %d", task.Priority)
		}
		if !task.UpdatedAt.Equal(originalUpdatedAt) {
			t.Error("UpdatedAt should not change when priority is unchanged")
		}
	})

	t.Run("does nothing when no task at cursor", func(t *testing.T) {
		project := newTestPriorityProject(t)
		m := NewPriorityViewModel(project, nil, nil)

		// Should not panic with empty project
		result, _ := m.changeTaskPriority(1)
		if result == nil {
			t.Error("expected non-nil result")
		}
	})

	t.Run("updates UpdatedAt timestamp", func(t *testing.T) {
		project := newTestPriorityProject(t)
		task, err := models.NewTask("1", 1, models.TaskTypeTask, "Test task")
		if err != nil {
			t.Fatal(err)
		}
		task.Priority = 3
		originalUpdatedAt := task.UpdatedAt
		project.Tasks = []*models.Task{task}

		m := NewPriorityViewModel(project, nil, nil)
		m.cursor = 0

		// Small delay to ensure time difference
		time.Sleep(time.Millisecond)

		m.changeTaskPriority(1)

		if !task.UpdatedAt.After(originalUpdatedAt) {
			t.Error("UpdatedAt should be updated after priority change")
		}
	})
}
