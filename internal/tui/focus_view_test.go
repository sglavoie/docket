package tui

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/focus"
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
func newTestFocusProject(t *testing.T, name string) *models.Project {
	t.Helper()
	project, err := models.NewProject(uuid.New().String(), name, "")
	if err != nil {
		t.Fatal(err)
	}
	return project
}

// newTestFocusTask creates a task with optional active flag.
func newTestFocusTask(t *testing.T, title string, priority int, active bool) *models.Task {
	t.Helper()
	task, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, title)
	if err != nil {
		t.Fatal(err)
	}
	task.Priority = priority
	task.Active = active
	return task
}

// newTestFocusViewModel creates a FocusViewModel with session state for testing.
func newTestFocusViewModel(cfg *config.Config) *FocusViewModel {
	queue := []FocusTask{}
	queueIDs := make(map[string]bool)
	dismissedIDs := make(map[string]bool)
	return &FocusViewModel{
		ViewBundle:   ViewBundle{config: cfg},
		queue:        &queue,
		queueIDs:     queueIDs,
		dismissedIDs: dismissedIDs,
	}
}

func TestFocusViewLoadTasks_FilteringLogic(t *testing.T) {
	t.Run("includes all non-complete tasks", func(t *testing.T) {
		project := newTestFocusProject(t, "Test")
		active := newTestFocusTask(t, "Active task", 3, true)
		inactive := newTestFocusTask(t, "Inactive task", 3, false)
		project.Tasks = []*models.Task{active, inactive}

		cfg := newTestFocusConfig([]*models.Project{project})
		m := newTestFocusViewModel(cfg)

		// Manually populate tasks (bypassing storage.LoadProject)
		for _, task := range project.Tasks {
			if task.Status.IsComplete() {
				continue
			}
			m.tasks = append(m.tasks, FocusTask{Task: task, Project: project})
		}

		if len(m.tasks) != 2 {
			t.Errorf("expected 2 tasks (active and inactive), got %d", len(m.tasks))
		}
	})

	t.Run("skips completed tasks", func(t *testing.T) {
		project := newTestFocusProject(t, "Test")
		done := newTestFocusTask(t, "Done task", 3, true)
		done.Status = models.StatusDone

		canceled := newTestFocusTask(t, "Canceled task", 3, true)
		canceled.Status = models.StatusCanceled

		active := newTestFocusTask(t, "Active task", 3, true)
		project.Tasks = []*models.Task{done, canceled, active}

		cfg := newTestFocusConfig([]*models.Project{project})
		m := newTestFocusViewModel(cfg)

		for _, task := range project.Tasks {
			if task.Status.IsComplete() {
				continue
			}
			m.tasks = append(m.tasks, FocusTask{Task: task, Project: project})
		}

		if len(m.tasks) != 1 {
			t.Errorf("expected 1 task (non-completed), got %d", len(m.tasks))
		}
	})

	t.Run("includes blocked-status tasks (dependency check is separate)", func(t *testing.T) {
		project := newTestFocusProject(t, "Test")
		blocked := newTestFocusTask(t, "Blocked task", 3, true)
		blocked.Status = models.StatusBlocked
		project.Tasks = []*models.Task{blocked}

		cfg := newTestFocusConfig([]*models.Project{project})
		m := newTestFocusViewModel(cfg)

		for _, task := range project.Tasks {
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

func TestFocusViewLoadTasks_ExcludesSnoozedTasks(t *testing.T) {
	project := newTestFocusProject(t, "Test")
	normal := newTestFocusTask(t, "Normal task", 3, false)
	snoozed := newTestFocusTask(t, "Snoozed task", 3, false)
	future := time.Now().AddDate(0, 0, 7)
	snoozed.SnoozedUntil = &future
	project.Tasks = []*models.Task{normal, snoozed}

	cfg := newTestFocusConfig([]*models.Project{project})
	m := newTestFocusViewModel(cfg)

	for _, task := range project.Tasks {
		if task.Status.IsComplete() {
			continue
		}
		if task.IsSnoozed() {
			continue
		}
		m.tasks = append(m.tasks, FocusTask{Task: task, Project: project})
	}

	if len(m.tasks) != 1 {
		t.Fatalf("expected 1 task (snoozed excluded), got %d", len(m.tasks))
	}
	if m.tasks[0].Task.Title != "Normal task" {
		t.Errorf("expected 'Normal task', got %q", m.tasks[0].Task.Title)
	}
}

func TestFocusViewLoadTasks_ExcludesDependencyBlockedTasks(t *testing.T) {
	project := newTestFocusProject(t, "Test")
	blocker := newTestFocusTask(t, "Blocker", 1, false)
	dependent := newTestFocusTask(t, "Dependent", 2, false)
	dependent.DependsOn = []string{blocker.ID}
	project.Tasks = []*models.Task{blocker, dependent}

	cfg := newTestFocusConfig([]*models.Project{project})
	m := newTestFocusViewModel(cfg)

	for _, task := range project.Tasks {
		if task.Status.IsComplete() {
			continue
		}
		if task.IsSnoozed() {
			continue
		}
		if project.HasIncompleteDependencies(task) {
			continue
		}
		m.tasks = append(m.tasks, FocusTask{Task: task, Project: project})
	}

	if len(m.tasks) != 1 {
		t.Fatalf("expected 1 task (dependency-blocked excluded), got %d", len(m.tasks))
	}
	if m.tasks[0].Task.Title != "Blocker" {
		t.Errorf("expected 'Blocker', got %q", m.tasks[0].Task.Title)
	}
}

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
		// Newer should come first (most recent first)
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

			// Create dummy tasks with a dummy project
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

func TestFocusViewMultipleProjects(t *testing.T) {
	t.Run("aggregates tasks from multiple projects", func(t *testing.T) {
		project1 := newTestFocusProject(t, "Project 1")
		task1 := newTestFocusTask(t, "Task from P1", 1, true)
		project1.Tasks = []*models.Task{task1}

		project2 := newTestFocusProject(t, "Project 2")
		task2 := newTestFocusTask(t, "Task from P2", 2, true)
		project2.Tasks = []*models.Task{task2}

		cfg := newTestFocusConfig([]*models.Project{project1, project2})
		m := newTestFocusViewModel(cfg)

		// Simulate loadTasks logic
		for _, project := range []*models.Project{project1, project2} {
			for _, task := range project.Tasks {
				if !task.Status.IsComplete() {
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

func TestFocusViewQueue(t *testing.T) {
	t.Run("accept and remove from queue", func(t *testing.T) {
		project := newTestFocusProject(t, "Test")
		task1 := newTestFocusTask(t, "Task 1", 1, true)
		task2 := newTestFocusTask(t, "Task 2", 2, true)

		cfg := newTestFocusConfig([]*models.Project{project})
		m := newTestFocusViewModel(cfg)

		ft1 := FocusTask{Task: task1, Project: project}
		ft2 := FocusTask{Task: task2, Project: project}

		// Accept tasks
		m.acceptIntoQueue(ft1)
		m.acceptIntoQueue(ft2)
		if len(*m.queue) != 2 {
			t.Fatalf("expected 2 queued tasks, got %d", len(*m.queue))
		}

		// Duplicate add is no-op
		m.acceptIntoQueue(ft1)
		if len(*m.queue) != 2 {
			t.Fatalf("expected 2 queued tasks after duplicate, got %d", len(*m.queue))
		}

		// Check membership
		if !m.isInQueue(task1.ID) {
			t.Error("expected task1 to be in queue")
		}

		// Remove first
		m.removeFromQueue(0)
		if len(*m.queue) != 1 {
			t.Fatalf("expected 1 queued task after removal, got %d", len(*m.queue))
		}
		if m.isInQueue(task1.ID) {
			t.Error("expected task1 to be removed from queue")
		}
	})

	t.Run("reorder queue", func(t *testing.T) {
		project := newTestFocusProject(t, "Test")
		task1 := newTestFocusTask(t, "Task 1", 1, true)
		task2 := newTestFocusTask(t, "Task 2", 2, true)

		cfg := newTestFocusConfig([]*models.Project{project})
		m := newTestFocusViewModel(cfg)

		m.acceptIntoQueue(FocusTask{Task: task1, Project: project})
		m.acceptIntoQueue(FocusTask{Task: task2, Project: project})

		m.reorderQueue(0, 1)
		q := *m.queue
		if q[0].Task.Title != "Task 2" || q[1].Task.Title != "Task 1" {
			t.Errorf("expected swapped order, got %s, %s", q[0].Task.Title, q[1].Task.Title)
		}
	})
}

func TestFocusViewName(t *testing.T) {
	m := &FocusViewModel{}
	if m.viewName() != "What's Next" {
		t.Errorf("expected 'What's Next', got %q", m.viewName())
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

func TestStaleQueuePruning(t *testing.T) {
	project := newTestFocusProject(t, "Test")
	task1 := newTestFocusTask(t, "Active", 1, true)
	task2 := newTestFocusTask(t, "Done", 2, false)
	task2.Status = models.StatusDone
	task3 := newTestFocusTask(t, "Canceled", 3, false)
	task3.Status = models.StatusCanceled

	cfg := newTestFocusConfig([]*models.Project{project})
	m := newTestFocusViewModel(cfg)

	// Add all three to the queue
	*m.queue = []FocusTask{
		{Task: task1, Project: project},
		{Task: task2, Project: project},
		{Task: task3, Project: project},
	}
	m.queueIDs[task1.ID] = true
	m.queueIDs[task2.ID] = true
	m.queueIDs[task3.ID] = true

	// Switch to queue tab and apply filter (triggers pruning)
	m.tabs.Active = focusTabQueue
	m.applyFilter()

	if len(*m.queue) != 1 {
		t.Fatalf("expected 1 task in queue after pruning, got %d", len(*m.queue))
	}
	if (*m.queue)[0].Task.Title != "Active" {
		t.Errorf("expected Active task to remain, got %q", (*m.queue)[0].Task.Title)
	}
	if m.isInQueue(task2.ID) || m.isInQueue(task3.ID) {
		t.Error("completed/canceled tasks should be removed from queueIDs")
	}
	if !m.isInQueue(task1.ID) {
		t.Error("active task should remain in queueIDs")
	}
}

func TestQueuePointerReconciliation(t *testing.T) {
	t.Run("queue pointers updated after loadTasks", func(t *testing.T) {
		project := newTestFocusProject(t, "Test")
		task := newTestFocusTask(t, "My Task", 1, true)
		project.Tasks = []*models.Task{task}

		cfg := newTestFocusConfig([]*models.Project{project})
		m := newTestFocusViewModel(cfg)

		// Simulate the queue holding a stale copy of the task
		staleTask := *task // copy
		*m.queue = []FocusTask{{Task: &staleTask, Project: project}}
		m.queueIDs[task.ID] = true

		// Modify the original task (simulating a status change via project tree)
		task.Status = models.StatusInProgress

		// The queue's stale pointer should NOT reflect the change yet
		if (*m.queue)[0].Task.Status == models.StatusInProgress {
			t.Fatal("stale pointer should not reflect status change before reconciliation")
		}

		// Manually populate tasks (bypassing storage) and run reconciliation
		m.tasks = nil
		for _, tk := range project.Tasks {
			if !tk.Status.IsComplete() {
				m.tasks = append(m.tasks, FocusTask{Task: tk, Project: project})
			}
		}

		// Build lookup and reconcile (same logic as loadTasks)
		taskByID := make(map[string]FocusTask, len(m.tasks))
		for _, ft := range m.tasks {
			taskByID[ft.Task.ID] = ft
		}
		q := *m.queue
		for i := range q {
			if fresh, ok := taskByID[q[i].Task.ID]; ok {
				q[i] = fresh
			}
		}
		*m.queue = q

		// Now the queue should have the fresh pointer
		if (*m.queue)[0].Task.Status != models.StatusInProgress {
			t.Errorf("expected reconciled status InProgress, got %v", (*m.queue)[0].Task.Status)
		}
	})

	t.Run("queue entries for non-actionable tasks are removed", func(t *testing.T) {
		project := newTestFocusProject(t, "Test")
		task1 := newTestFocusTask(t, "Active", 1, true)
		task2 := newTestFocusTask(t, "Completed", 2, false)
		task2.Status = models.StatusDone
		project.Tasks = []*models.Task{task1, task2}

		cfg := newTestFocusConfig([]*models.Project{project})
		m := newTestFocusViewModel(cfg)

		// Queue both tasks (task2 was todo when queued, now done)
		*m.queue = []FocusTask{
			{Task: task1, Project: project},
			{Task: task2, Project: project},
		}
		m.queueIDs[task1.ID] = true
		m.queueIDs[task2.ID] = true

		// Simulate loadTasks gathering only actionable tasks
		m.tasks = nil
		for _, tk := range project.Tasks {
			if !tk.Status.IsComplete() {
				m.tasks = append(m.tasks, FocusTask{Task: tk, Project: project})
			}
		}

		// Run reconciliation logic
		taskByID := make(map[string]FocusTask, len(m.tasks))
		for _, ft := range m.tasks {
			taskByID[ft.Task.ID] = ft
		}
		q := *m.queue
		i := 0
		for i < len(q) {
			if fresh, ok := taskByID[q[i].Task.ID]; ok {
				q[i] = fresh
				i++
			} else {
				delete(m.queueIDs, q[i].Task.ID)
				q = append(q[:i], q[i+1:]...)
			}
		}
		*m.queue = q

		if len(*m.queue) != 1 {
			t.Fatalf("expected 1 task in queue after reconciliation, got %d", len(*m.queue))
		}
		if (*m.queue)[0].Task.Title != "Active" {
			t.Errorf("expected 'Active' to remain, got %q", (*m.queue)[0].Task.Title)
		}
		if m.isInQueue(task2.ID) {
			t.Error("completed task should be removed from queueIDs")
		}
	})
}

func TestAcceptOnAnyTab(t *testing.T) {
	project := newTestFocusProject(t, "Test")
	task1 := newTestFocusTask(t, "Task 1", 1, true)
	task2 := newTestFocusTask(t, "Task 2", 2, true)
	task3 := newTestFocusTask(t, "Task 3", 3, true)

	cfg := newTestFocusConfig([]*models.Project{project})
	m := newTestFocusViewModel(cfg)
	m.tasks = []FocusTask{
		{Task: task1, Project: project},
		{Task: task2, Project: project},
		{Task: task3, Project: project},
	}

	// Populate ranked suggestions
	m.ranked = focus.RankCrossProjectTasks(m.tasks, m.dismissedIDs, cfg)

	t.Run("accept on Queue tab adds top non-queued suggestion", func(t *testing.T) {
		m.tabs.Active = focusTabQueue
		m.handleAccept()

		if len(*m.queue) != 1 {
			t.Fatalf("expected 1 task in queue, got %d", len(*m.queue))
		}
		// Should be whatever ranked first
		firstID := (*m.queue)[0].Task.ID
		if firstID != m.ranked[0].Task.ID {
			t.Errorf("expected top ranked task in queue")
		}
	})

	t.Run("accept on Queue tab skips already-queued task", func(t *testing.T) {
		m.handleAccept()

		if len(*m.queue) != 2 {
			t.Fatalf("expected 2 tasks in queue, got %d", len(*m.queue))
		}
		// Second queued should be different from first
		if (*m.queue)[0].Task.ID == (*m.queue)[1].Task.ID {
			t.Error("second accept should add a different task")
		}
	})

	t.Run("accept on By Project tab also adds suggestion", func(t *testing.T) {
		// Reset
		*m.queue = nil
		m.queueIDs = make(map[string]bool)

		m.tabs.Active = focusTabByProject
		m.handleAccept()

		if len(*m.queue) != 1 {
			t.Fatalf("expected 1 task in queue, got %d", len(*m.queue))
		}
	})
}

func TestTopNonQueuedSuggestion(t *testing.T) {
	project := newTestFocusProject(t, "Test")
	task1 := newTestFocusTask(t, "Task 1", 1, true)
	task2 := newTestFocusTask(t, "Task 2", 2, true)

	cfg := newTestFocusConfig([]*models.Project{project})
	m := newTestFocusViewModel(cfg)
	m.tasks = []FocusTask{
		{Task: task1, Project: project},
		{Task: task2, Project: project},
	}
	m.ranked = focus.RankCrossProjectTasks(m.tasks, m.dismissedIDs, cfg)

	// Queue the top-ranked task
	m.acceptIntoQueue(m.ranked[0].FocusTask)

	sft := m.topNonQueuedSuggestion()
	if sft == nil {
		t.Fatal("expected a non-queued suggestion")
	}
	if sft.Task.ID == m.ranked[0].Task.ID {
		t.Error("topNonQueuedSuggestion should skip queued tasks")
	}

	// Queue the second one too
	m.acceptIntoQueue(sft.FocusTask)
	if m.topNonQueuedSuggestion() != nil {
		t.Error("expected nil when all suggestions are queued")
	}
}
