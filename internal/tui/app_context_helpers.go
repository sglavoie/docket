package tui

import "github.com/sglavoie/tt/internal/models"

// contextualProject resolves the project the user is currently looking at.
// On the dashboard it returns the highlighted project; in project-specific views
// it reads the project from the view model; in cross-project views it resolves
// from the cursor task. Falls back to m.currentProject.
func (m *AppModel) contextualProject() *models.Project {
	cv := m.currentView()
	switch cv {
	case ViewDashboard:
		if m.dashboard != nil {
			if p := m.dashboard.SelectedProject(); p != nil {
				return p
			}
		}
	case ViewProjectTree:
		if m.projectTree != nil {
			return m.projectTree.project
		}
	case ViewOutliner:
		if m.outliner != nil && m.outliner.project != nil {
			return m.outliner.project
		}
	case ViewKanban:
		if v := m.views.Get(ViewKanban); v != nil {
			if kb, ok := v.(*KanbanModel); ok && kb.project != nil {
				return kb.project
			}
		}
	default:
		if v := m.views.Get(cv); v != nil {
			type taskRefProvider interface {
				getTaskRef() (*models.Task, *models.Project)
			}
			if trp, ok := v.(taskRefProvider); ok {
				if _, p := trp.getTaskRef(); p != nil {
					return p
				}
			}
		}
	}
	return m.currentProject
}

// saveCurrentViewFilterState snapshots filter state for the current view
// so it can be restored after history navigation reconstructs the view.
func (m *AppModel) saveCurrentViewFilterState() {
	switch m.currentView() {
	case ViewPriorityView:
		if pv, ok := m.views.Get(ViewPriorityView).(*PriorityViewModel); ok && m.currentProject != nil {
			m.priorityViewFilterCache[m.currentProject.ID] = &PriorityViewCache{
				Filter:    *copyCommonFilterState(&pv.filter),
				TabActive: pv.tabs.Active,
			}
			// Evict oldest entries when over cap
			for len(m.priorityViewFilterCache) > maxProjectLastViewEntries {
				for id := range m.priorityViewFilterCache {
					if id != m.currentProject.ID {
						delete(m.priorityViewFilterCache, id)
						break
					}
				}
			}
		}
	case ViewAllTasks:
		if m.allTasksView != nil {
			m.allTasksFilterCache = &AllTasksCache{
				Filter:    *copyCommonFilterState(&m.allTasksView.filter),
				GroupBy:   m.allTasksView.groupBy,
				TabActive: m.allTasksView.tabs.Active,
			}
		}
	default:
	}
}

// mapKeys returns the keys of a map[string]bool as a slice.
func mapKeys(m map[string]bool) []string {
	if m == nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
