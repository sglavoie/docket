package tui

import (
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
)

func TestGetTaskAtCursor(t *testing.T) {
	t.Run("returns correct task across groups", func(t *testing.T) {
		project := newTestPriorityProject(t)

		overdue := newTestTaskWithDueDate(t, "Overdue", -1)
		today := newTestTaskWithDueDate(t, "Today", 0)
		thisWeek := newTestTaskWithDueDate(t, "This week", 3)

		project.Tasks = []*models.Task{overdue, today, thisWeek}

		m := NewPriorityViewModel(project, nil, nil)

		m.cursor = 0
		task := m.getTaskAtCursor()
		if task == nil || task.Task.Title != "Overdue" {
			t.Errorf("cursor 0: expected 'Overdue', got %v", task)
		}

		m.cursor = 1
		task = m.getTaskAtCursor()
		if task == nil || task.Task.Title != "Today" {
			t.Errorf("cursor 1: expected 'Today', got %v", task)
		}

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

func TestJumpToTask_UsesFilteredGroups(t *testing.T) {
	project := newTestPriorityProject(t)

	// Create tasks in different urgency groups
	overdue := newTestTaskWithDueDate(t, "Overdue", -2)
	overdue.Priority = 1
	today := newTestTaskWithDueDate(t, "Today", 0)
	today.Priority = 2
	thisWeek := newTestTaskWithDueDate(t, "This week", 3)
	thisWeek.Priority = 3

	project.Tasks = []*models.Task{overdue, today, thisWeek}

	m := NewPriorityViewModel(project, nil, nil)

	// Without filters, jumpToTask should find the correct index
	m.jumpToTask(thisWeek)
	pt := m.getTaskAtCursor()
	if pt == nil || pt.Task.ID != thisWeek.ID {
		t.Errorf("jumpToTask without filters: expected 'This week', got cursor=%d", m.cursor)
	}

	// Activate search that excludes "Overdue" and "Today"
	m.searchQuery = "This week"
	m.recalculateNavigation()
	m.clampCursor()

	// Now jumpToTask should find "This week" at index 0 (only visible task)
	m.jumpToTask(thisWeek)
	pt = m.getTaskAtCursor()
	if pt == nil || pt.Task.ID != thisWeek.ID {
		t.Errorf("jumpToTask with search filter: expected 'This week' at cursor 0, got cursor=%d", m.cursor)
	}
	if m.cursor != 0 {
		t.Errorf("jumpToTask with search filter: expected cursor=0 (only matching task), got cursor=%d", m.cursor)
	}
}

func TestJumpToTask_BlockedTab(t *testing.T) {
	project := newTestPriorityProject(t)

	blockedTask := newTestTaskWithDueDate(t, "Blocked task", 1)
	blockedTask.WorkflowState = models.StateWaiting

	normalTask := newTestTaskWithDueDate(t, "Normal task", 0)

	project.Tasks = []*models.Task{blockedTask, normalTask}

	m := NewPriorityViewModel(project, nil, nil)

	// Switch to Blocked tab
	m.tabs.Active = priorityTabBlocked
	m.recalculateNavigation()
	m.clampCursor()

	// jumpToTask should find the blocked task in the waiting list
	m.jumpToTask(blockedTask)
	pt := m.getTaskAtCursor()
	if pt == nil || pt.Task.ID != blockedTask.ID {
		t.Errorf("jumpToTask on Blocked tab: expected 'Blocked task', got cursor=%d", m.cursor)
	}
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

	if !m.waitingOnCollapsed {
		t.Error("waitingOnCollapsed should be true by default")
	}

	m.waitingOnCollapsed = false
	if m.waitingOnCollapsed {
		t.Error("waitingOnCollapsed should be false after toggle")
	}
}
