package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// taskOpsView implementation for DailyReviewModel.

func (m *DailyReviewModel) getTaskRef() (*models.Task, *models.Project) {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return nil, nil
	}
	return dt.Task, dt.Project
}

func (m *DailyReviewModel) getPersister() Persister { return defaultPersister }
func (m *DailyReviewModel) reloadData()             { m.loadTasks() }
func (m *DailyReviewModel) setErr(err error)        { m.err = err }
func (m *DailyReviewModel) setStatus(msg string)    { m.statusMessage = msg }

// Delegate all operations to the shared taskOps functions.

func (m *DailyReviewModel) changeTaskStatus(newStatus models.TaskStatus) (tea.Model, tea.Cmd) {
	return taskOpsChangeStatus(m, newStatus)
}

func (m *DailyReviewModel) changeTaskPriority(key string) (tea.Model, tea.Cmd) {
	return taskOpsChangePriority(m, key)
}

func (m *DailyReviewModel) setQuickDueDate(daysFromNow int) (tea.Model, tea.Cmd) {
	return taskOpsSetQuickDueDate(m, daysFromNow)
}

func (m *DailyReviewModel) clearDueDate() (tea.Model, tea.Cmd) {
	return taskOpsClearDueDate(m)
}

func (m *DailyReviewModel) toggleActive() (tea.Model, tea.Cmd) {
	return taskOpsToggleActive(m)
}

func (m *DailyReviewModel) sectionMove(section string) (tea.Model, tea.Cmd) {
	return taskOpsSectionMove(m, section)
}

func (m *DailyReviewModel) deleteTask() (tea.Model, tea.Cmd) {
	return taskOpsDeleteTask(m)
}
