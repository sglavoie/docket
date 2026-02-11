package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// reviewOpsView implementation for DailyReviewModel.

func (m *DailyReviewModel) getReviewTaskRef() (*models.Task, *models.Project) {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return nil, nil
	}
	return dt.Task, dt.Project
}

func (m *DailyReviewModel) reloadReviewData()          { m.loadTasks() }
func (m *DailyReviewModel) setReviewErr(err error)     { m.err = err }
func (m *DailyReviewModel) setReviewStatus(msg string) { m.statusMessage = msg }

// Delegate all operations to the shared review functions.

func (m *DailyReviewModel) changeTaskStatus(newStatus models.TaskStatus) (tea.Model, tea.Cmd) {
	return reviewChangeStatus(m, newStatus)
}

func (m *DailyReviewModel) changeTaskPriority(key string) (tea.Model, tea.Cmd) {
	return reviewChangePriority(m, key)
}

func (m *DailyReviewModel) setQuickDueDate(daysFromNow int) (tea.Model, tea.Cmd) {
	return reviewSetQuickDueDate(m, daysFromNow)
}

func (m *DailyReviewModel) clearDueDate() (tea.Model, tea.Cmd) {
	return reviewClearDueDate(m)
}

func (m *DailyReviewModel) toggleActive() (tea.Model, tea.Cmd) {
	return reviewToggleActive(m)
}

func (m *DailyReviewModel) sectionMove(section string) (tea.Model, tea.Cmd) {
	return reviewSectionMove(m, section)
}

func (m *DailyReviewModel) deleteTask() (tea.Model, tea.Cmd) {
	return reviewDeleteTask(m)
}
