package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
)

// recordTaskVisit records a task visit for the quick-switch list.
// It deduplicates by task ID (moving revisits to the front), trims to m.config.GetMaxRecentTaskVisits(),
// and persists the ID-only form to config.
func (m *AppModel) recordTaskVisit(task *models.Task, project *models.Project) {
	if task == nil || project == nil {
		return
	}

	var wasPinned bool
	filtered := m.recentTaskVisits[:0:0]
	for _, entry := range m.recentTaskVisits {
		if entry.Task.ID == task.ID {
			wasPinned = entry.Pinned
		} else {
			filtered = append(filtered, entry)
		}
	}

	m.recentTaskVisits = append([]RecentTaskVisitEntry{{Task: task, Project: project, Pinned: wasPinned}}, filtered...)

	if len(m.recentTaskVisits) > m.config.GetMaxRecentTaskVisits() {
		var pinned, unpinned []RecentTaskVisitEntry
		for _, entry := range m.recentTaskVisits {
			if entry.Pinned {
				pinned = append(pinned, entry)
			} else {
				unpinned = append(unpinned, entry)
			}
		}
		remaining := max(m.config.GetMaxRecentTaskVisits()-len(pinned), 0)
		if len(unpinned) > remaining {
			unpinned = unpinned[:remaining]
		}
		m.recentTaskVisits = append(pinned, unpinned...)
	}

	m.persistRecentTaskVisits()
}

// persistRecentTaskVisits saves the current recentTaskVisits to config.
func (m *AppModel) persistRecentTaskVisits() {
	visits := make([]config.RecentTaskVisit, len(m.recentTaskVisits))
	for i, entry := range m.recentTaskVisits {
		visits[i] = config.RecentTaskVisit{
			TaskID:    entry.Task.ID,
			ProjectID: entry.Project.ID,
			Pinned:    entry.Pinned,
		}
	}
	m.config.SetRecentTaskVisits(visits)
	if err := m.configManager.Save(m.config); err != nil {
		logging.Warn("failed to save config: %v", err)
	}
}

// refreshRecentTaskVisits re-hydrates each entry's Task pointer from current project data
// so that status changes (completions, etc.) are reflected when the popup opens.
func (m *AppModel) refreshRecentTaskVisits() {
	valid := m.recentTaskVisits[:0]
	for _, entry := range m.recentTaskVisits {
		_ = m.persister.Load(entry.Project)
		if fresh := entry.Project.GetTaskByID(entry.Task.ID); fresh != nil {
			entry.Task = fresh
			valid = append(valid, entry)
		}
	}
	m.recentTaskVisits = valid
}

// sortedQuickSwitchEntries returns entries sorted: pinned first (by recency), then unpinned (by recency).
func (m *AppModel) sortedQuickSwitchEntries() []RecentTaskVisitEntry {
	var pinned, unpinned []RecentTaskVisitEntry
	for _, entry := range m.recentTaskVisits {
		if entry.Pinned {
			pinned = append(pinned, entry)
		} else {
			unpinned = append(unpinned, entry)
		}
	}
	return append(pinned, unpinned...)
}
