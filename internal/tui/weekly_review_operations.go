package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// trackMutation increments the count for a mutation category.
func (m *WeeklyReviewModel) trackMutation(category string) {
	m.mutationCounts[category]++
}

// recordCommittedDeletion credits the completion summary's Deletions category
// for a committed Weekly Review-origin canonical deletion. count is the number
// of root tasks the user chose to delete (the confirmation-prompt count), so a
// delete increments the Deletions row by the same per-task unit the single-task
// status/priority/etc. counters use. Counting happens only here, after the shell
// commits, so cancelled and failed deletions stay uncounted.
func (m *WeeklyReviewModel) recordCommittedDeletion(count int) {
	if count <= 0 {
		return
	}
	m.mutationCounts["delete"] += count
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
