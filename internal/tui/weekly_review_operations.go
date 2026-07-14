package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// trackMutation increments the count for a mutation category.
func (m *WeeklyReviewModel) trackMutation(category string) {
	m.mutationCounts[category]++
}

// taskOpsView implementation for WeeklyReviewModel.

func (m *WeeklyReviewModel) getTaskRef() (*models.Task, *models.Project) {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return nil, nil
	}
	return dt.Task, dt.Project
}

func (m *WeeklyReviewModel) getRuntime() *config.RuntimeState  { return m.runtime }
func (m *WeeklyReviewModel) getConfigManager() *config.Manager { return m.configManager }
func (m *WeeklyReviewModel) reloadData()                       { m.loadData() }
func (m *WeeklyReviewModel) setStatus(msg string)              { m.InteractiveViewState.setStatus(msg) }
