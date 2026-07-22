package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
)

// recordTaskVisit records a task visit for the quick-switch list.
// It deduplicates by project-qualified identity (moving revisits to the front),
// trims to m.runtime.Settings().GetMaxRecentTaskVisits(), and persists the
// already-qualified config shape.
func (m *AppModel) recordTaskVisit(task *models.Task, project *models.Project) {
	if task == nil || project == nil {
		return
	}
	identity := taskIdentityFor(task, project)
	if !identity.Valid() {
		return
	}

	var wasPinned bool
	filtered := m.recentTaskVisits[:0:0]
	for _, entry := range m.recentTaskVisits {
		if entry.Identity() == identity {
			wasPinned = entry.Pinned
		} else {
			filtered = append(filtered, entry)
		}
	}

	m.recentTaskVisits = append([]RecentTaskVisitEntry{{Task: task, Project: project, Pinned: wasPinned}}, filtered...)

	if len(m.recentTaskVisits) > m.runtime.Settings().GetMaxRecentTaskVisits() {
		var pinned, unpinned []RecentTaskVisitEntry
		for _, entry := range m.recentTaskVisits {
			if entry.Pinned {
				pinned = append(pinned, entry)
			} else {
				unpinned = append(unpinned, entry)
			}
		}
		remaining := max(m.runtime.Settings().GetMaxRecentTaskVisits()-len(pinned), 0)
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
	if err := m.commitMachineMutation(func(machine *config.MachineState) {
		machine.SetRecentTaskVisits(visits)
	}); err != nil {
		logging.Warn("failed to save machine state: %v", err)
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
