package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
)

// RecentTaskVisitEntry is a hydrated machine-local task visit used by Finder.
type RecentTaskVisitEntry struct {
	Task    *models.Task
	Project *models.Project
	Pinned  bool
}

func (entry RecentTaskVisitEntry) Identity() TaskIdentity {
	return taskIdentityFor(entry.Task, entry.Project)
}

// recordTaskVisit records a task visit for Finder recents.
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
