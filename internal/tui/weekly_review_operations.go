package tui

import (
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// changeTaskStatus changes the status of the task at cursor.
func (m *WeeklyReviewModel) changeTaskStatus(newStatus models.TaskStatus) (tea.Model, tea.Cmd) {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return m, nil
	}

	result := applyStatusChange(defaultPersister, dt.Project, dt.Task.ID, newStatus)
	if result.Err != nil {
		m.err = result.Err
	} else if result.StatusMessage != "" {
		m.statusMessage = result.StatusMessage
	}
	m.loadData()
	m.clampCursor()
	return m, result.Cmd
}

// changeTaskPriority changes the priority of the task at cursor.
func (m *WeeklyReviewModel) changeTaskPriority(key string) (tea.Model, tea.Cmd) {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return m, nil
	}

	priority, _ := strconv.Atoi(key)
	result := applyPriorityChange(defaultPersister, dt.Project, dt.Task.ID, priority)
	if result.Err != nil {
		m.err = result.Err
	} else if result.StatusMessage != "" {
		m.statusMessage = result.StatusMessage
	}
	m.loadData()
	m.clampCursor()
	return m, result.Cmd
}

// setQuickDueDate sets the due date to a relative number of days from today.
func (m *WeeklyReviewModel) setQuickDueDate(daysFromNow int) (tea.Model, tea.Cmd) {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return m, nil
	}

	result := applyQuickDueDate(defaultPersister, dt.Project, dt.Task.ID, daysFromNow)
	if result.Err != nil {
		m.err = result.Err
	} else if result.StatusMessage != "" {
		m.statusMessage = result.StatusMessage
	}
	m.loadData()
	m.clampCursor()
	return m, result.Cmd
}

// clearDueDate clears the due date from the task at cursor.
func (m *WeeklyReviewModel) clearDueDate() (tea.Model, tea.Cmd) {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return m, nil
	}

	result := applyClearDueDate(defaultPersister, dt.Project, dt.Task.ID)
	if result.Err != nil {
		m.err = result.Err
	} else if result.StatusMessage != "" {
		m.statusMessage = result.StatusMessage
	}
	m.loadData()
	m.clampCursor()
	return m, result.Cmd
}

// toggleActive toggles the Active flag on the task at cursor.
func (m *WeeklyReviewModel) toggleActive() (tea.Model, tea.Cmd) {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return m, nil
	}

	result := applyToggleActive(defaultPersister, dt.Project, dt.Task.ID)
	if result.Err != nil {
		m.err = result.Err
	} else if result.StatusMessage != "" {
		m.statusMessage = result.StatusMessage
	}
	m.loadData()
	m.clampCursor()
	return m, result.Cmd
}

// sectionMove moves the task at cursor to the specified GTD section.
func (m *WeeklyReviewModel) sectionMove(section string) (tea.Model, tea.Cmd) {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return m, nil
	}

	result := applySectionChange(defaultPersister, dt.Project, dt.Task.ID, section)
	if result.Err != nil {
		m.err = result.Err
	} else if result.StatusMessage != "" {
		m.statusMessage = result.StatusMessage
	}
	m.loadData()
	m.clampCursor()
	return m, result.Cmd
}

// deleteTask deletes the task at cursor after confirmation.
func (m *WeeklyReviewModel) deleteTask() (tea.Model, tea.Cmd) {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return m, nil
	}

	result := applyDeleteTask(defaultPersister, dt.Project, dt.Task.ID)
	if result.Err != nil {
		m.err = result.Err
	} else if result.StatusMessage != "" {
		m.statusMessage = result.StatusMessage
	}
	m.loadData()
	m.clampCursor()
	return m, result.Cmd
}
