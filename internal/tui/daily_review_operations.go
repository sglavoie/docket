package tui

import (
	"fmt"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// changeTaskStatus changes the status of the task at cursor.
func (m *DailyReviewModel) changeTaskStatus(newStatus models.TaskStatus) (tea.Model, tea.Cmd) {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return m, nil
	}

	project := dt.Project

	// Re-lookup task to avoid stale pointer after concurrent reload
	task := project.GetTaskByID(dt.Task.ID)
	if task == nil {
		m.statusMessage = "Task no longer exists in project"
		return m, nil
	}

	// Skip completed tasks for most status changes
	if task.Status.IsComplete() && newStatus != models.StatusTodo {
		m.statusMessage = "Cannot change status of completed task"
		return m, nil
	}

	// Validate status transition
	switch newStatus {
	case models.StatusInProgress:
		if task.Status != models.StatusTodo && task.Status != models.StatusBlocked {
			m.statusMessage = "Task must be todo or blocked to start"
			return m, nil
		}
	case models.StatusDone:
		if !task.Status.IsActive() {
			m.statusMessage = "Task is not active"
			return m, nil
		}
		// Check for incomplete dependencies
		if project.HasIncompleteDependencies(task) {
			m.err = fmt.Errorf("cannot complete: has incomplete dependencies")
			return m, nil
		}
	case models.StatusBlocked:
		if !task.Status.IsActive() || task.Status == models.StatusBlocked {
			m.statusMessage = "Task must be active and not already blocked"
			return m, nil
		}
	case models.StatusCanceled:
		if !task.Status.IsActive() {
			m.statusMessage = "Task is not active"
			return m, nil
		}
	case models.StatusTodo:
		if task.Status == models.StatusTodo {
			m.statusMessage = "Task is already todo"
			return m, nil
		}
	}

	// Backup before change
	if err := storage.BackupProject(project); err != nil {
		m.err = fmt.Errorf("backup failed: %w", err)
		return m, nil
	}

	// Update task status
	project.MutateTask(dt.Task.ID, func(t *models.Task) {
		t.Status = newStatus
		if newStatus.IsComplete() {
			now := time.Now()
			t.CompletedAt = &now
		} else {
			t.CompletedAt = nil
		}
	})

	// Save project
	if err := storage.SaveProject(project); err != nil {
		m.err = fmt.Errorf("save failed: %w", err)
		return m, nil
	}

	// Reload project and refresh view
	_ = storage.LoadProject(project)
	m.loadTasks()
	m.clampCursor()

	m.statusMessage = fmt.Sprintf("Task #%d -> %s", task.ShortID, newStatus)

	return m, func() tea.Msg {
		return TaskStatusChangedMsg{Task: task}
	}
}

// changeTaskPriority changes the priority of the task at cursor.
func (m *DailyReviewModel) changeTaskPriority(key string) (tea.Model, tea.Cmd) {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return m, nil
	}

	priority, _ := strconv.Atoi(key)
	project := dt.Project

	// Re-lookup task to avoid stale pointer after concurrent reload
	task := project.GetTaskByID(dt.Task.ID)
	if task == nil {
		m.statusMessage = "Task no longer exists in project"
		return m, nil
	}

	if task.Priority == priority {
		return m, nil
	}

	// Backup before change
	if err := storage.BackupProject(project); err != nil {
		m.err = fmt.Errorf("backup failed: %w", err)
		return m, nil
	}

	// Update task priority
	project.MutateTask(dt.Task.ID, func(t *models.Task) {
		t.Priority = priority
	})

	// Save project
	if err := storage.SaveProject(project); err != nil {
		m.err = fmt.Errorf("save failed: %w", err)
		return m, nil
	}

	// Reload project and refresh view
	_ = storage.LoadProject(project)
	m.loadTasks()
	m.clampCursor()

	m.statusMessage = fmt.Sprintf("Priority set to %d", priority)

	return m, nil
}

// setQuickDueDate sets the due date to a relative number of days from today.
func (m *DailyReviewModel) setQuickDueDate(daysFromNow int) (tea.Model, tea.Cmd) {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return m, nil
	}

	project := dt.Project

	// Re-lookup task to avoid stale pointer after concurrent reload
	task := project.GetTaskByID(dt.Task.ID)
	if task == nil {
		m.statusMessage = "Task no longer exists in project"
		return m, nil
	}

	// Backup before change
	if err := storage.BackupProject(project); err != nil {
		m.err = fmt.Errorf("backup failed: %w", err)
		return m, nil
	}

	// Set due date
	project.MutateTask(dt.Task.ID, func(t *models.Task) {
		newDate := time.Now().AddDate(0, 0, daysFromNow)
		newDate = time.Date(newDate.Year(), newDate.Month(), newDate.Day(), 0, 0, 0, 0, newDate.Location())
		t.DueDate = &newDate
	})

	// Save project
	if err := storage.SaveProject(project); err != nil {
		m.err = fmt.Errorf("save failed: %w", err)
		return m, nil
	}

	// Reload project and refresh view
	_ = storage.LoadProject(project)
	m.loadTasks()
	m.clampCursor()

	label := "today"
	switch daysFromNow {
	case 1:
		label = "tomorrow"
	case 7:
		label = "next week"
	case 30:
		label = "next month"
	}
	m.statusMessage = fmt.Sprintf("Due date set to %s", label)

	return m, nil
}

// clearDueDate clears the due date from the task at cursor.
func (m *DailyReviewModel) clearDueDate() (tea.Model, tea.Cmd) {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return m, nil
	}

	project := dt.Project

	// Re-lookup task to avoid stale pointer after concurrent reload
	task := project.GetTaskByID(dt.Task.ID)
	if task == nil {
		m.statusMessage = "Task no longer exists in project"
		return m, nil
	}

	if task.DueDate == nil {
		m.statusMessage = "Task has no due date"
		return m, nil
	}

	// Backup before change
	if err := storage.BackupProject(project); err != nil {
		m.err = fmt.Errorf("backup failed: %w", err)
		return m, nil
	}

	// Clear due date
	project.MutateTask(dt.Task.ID, func(t *models.Task) {
		t.DueDate = nil
	})

	// Save project
	if err := storage.SaveProject(project); err != nil {
		m.err = fmt.Errorf("save failed: %w", err)
		return m, nil
	}

	// Reload project and refresh view
	_ = storage.LoadProject(project)
	m.loadTasks()
	m.clampCursor()

	m.statusMessage = "Due date cleared"

	return m, nil
}

// toggleActive toggles the Active flag on the task at cursor.
func (m *DailyReviewModel) toggleActive() (tea.Model, tea.Cmd) {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return m, nil
	}

	project := dt.Project

	// Re-lookup task to avoid stale pointer after concurrent reload
	task := project.GetTaskByID(dt.Task.ID)
	if task == nil {
		m.statusMessage = "Task no longer exists in project"
		return m, nil
	}

	// Backup before change
	if err := storage.BackupProject(project); err != nil {
		m.err = fmt.Errorf("backup failed: %w", err)
		return m, nil
	}

	// Toggle active flag
	project.MutateTask(dt.Task.ID, func(t *models.Task) {
		t.Active = !t.Active
	})

	// Save project
	if err := storage.SaveProject(project); err != nil {
		m.err = fmt.Errorf("save failed: %w", err)
		return m, nil
	}

	// Reload project and refresh view
	_ = storage.LoadProject(project)
	m.loadTasks()
	m.clampCursor()

	if task.Active {
		m.statusMessage = fmt.Sprintf("Task #%d marked active", task.ShortID)
	} else {
		m.statusMessage = fmt.Sprintf("Task #%d unmarked active", task.ShortID)
	}

	return m, func() tea.Msg {
		return TaskUpdatedMsg{Task: task}
	}
}
