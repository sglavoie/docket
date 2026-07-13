package tui

import (
	"github.com/sglavoie/tt/internal/models"
)

// minimalOutliner returns a bare OutlinerModel suitable for unit tests that
// call cross-project row builders directly without touching disk-backed state.
func minimalOutliner() *OutlinerModel {
	return &OutlinerModel{
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
	}
}

// makeTask creates a minimal task with the given title and section. The section
// name is mapped onto the task's canonical WorkflowState via SetSection.
func makeTask(id, title, section string) *models.Task {
	task := &models.Task{
		ID:    id,
		Title: title,
	}
	task.SetSection(section)
	return task
}
