package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// taskOpsView implementation for WeeklyReviewModel.

func (m *WeeklyReviewModel) getTaskRef() (*models.Task, *models.Project) {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return nil, nil
	}
	return dt.Task, dt.Project
}

func (m *WeeklyReviewModel) getPersister() Persister { return defaultPersister }
func (m *WeeklyReviewModel) reloadData()             { m.loadData() }
func (m *WeeklyReviewModel) setErr(err error)        { m.err = err }
func (m *WeeklyReviewModel) setStatus(msg string)    { m.statusMessage = msg }

// Delegate all operations to the shared taskOps functions.

func (m *WeeklyReviewModel) changeTaskStatus(newStatus models.TaskStatus) (tea.Model, tea.Cmd) {
	return taskOpsChangeStatus(m, newStatus)
}

func (m *WeeklyReviewModel) changeTaskPriority(key string) (tea.Model, tea.Cmd) {
	return taskOpsChangePriority(m, key)
}

func (m *WeeklyReviewModel) setQuickDueDate(daysFromNow int) (tea.Model, tea.Cmd) {
	return taskOpsSetQuickDueDate(m, daysFromNow)
}

func (m *WeeklyReviewModel) clearDueDate() (tea.Model, tea.Cmd) {
	return taskOpsClearDueDate(m)
}

func (m *WeeklyReviewModel) toggleActive() (tea.Model, tea.Cmd) {
	return taskOpsToggleActive(m)
}

func (m *WeeklyReviewModel) sectionMove(section string) (tea.Model, tea.Cmd) {
	return taskOpsSectionMove(m, section)
}

func (m *WeeklyReviewModel) deleteTask() (tea.Model, tea.Cmd) {
	return taskOpsDeleteTask(m)
}
