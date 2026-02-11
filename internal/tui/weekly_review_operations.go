package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// reviewOpsView implementation for WeeklyReviewModel.

func (m *WeeklyReviewModel) getReviewTaskRef() (*models.Task, *models.Project) {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return nil, nil
	}
	return dt.Task, dt.Project
}

func (m *WeeklyReviewModel) reloadReviewData()          { m.loadData() }
func (m *WeeklyReviewModel) setReviewErr(err error)     { m.err = err }
func (m *WeeklyReviewModel) setReviewStatus(msg string) { m.statusMessage = msg }

// Delegate all operations to the shared review functions.

func (m *WeeklyReviewModel) changeTaskStatus(newStatus models.TaskStatus) (tea.Model, tea.Cmd) {
	return reviewChangeStatus(m, newStatus)
}

func (m *WeeklyReviewModel) changeTaskPriority(key string) (tea.Model, tea.Cmd) {
	return reviewChangePriority(m, key)
}

func (m *WeeklyReviewModel) setQuickDueDate(daysFromNow int) (tea.Model, tea.Cmd) {
	return reviewSetQuickDueDate(m, daysFromNow)
}

func (m *WeeklyReviewModel) clearDueDate() (tea.Model, tea.Cmd) {
	return reviewClearDueDate(m)
}

func (m *WeeklyReviewModel) toggleActive() (tea.Model, tea.Cmd) {
	return reviewToggleActive(m)
}

func (m *WeeklyReviewModel) sectionMove(section string) (tea.Model, tea.Cmd) {
	return reviewSectionMove(m, section)
}

func (m *WeeklyReviewModel) deleteTask() (tea.Model, tea.Cmd) {
	return reviewDeleteTask(m)
}
