package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// RecentTaskVisitEntry holds a hydrated task+project pair for the quick-switch list.
type RecentTaskVisitEntry struct {
	Task    *models.Task
	Project *models.Project
	Pinned  bool
}

const maxPinnedTaskVisits = 9

// openQuickSwitch resets quick-switch state and refreshes the task list.
func (m *AppModel) openQuickSwitch() {
	m.showQuickSwitch = true
	m.quickSwitchCursor = 0
	m.quickSwitchFilter = ""
	m.quickSwitchFiltering = false
	m.quickSwitchFiltered = nil
	m.quickSwitchPendingPrefix = ""
	m.quickSwitchShowCompleted = false
	m.refreshRecentTaskVisits()
	// Re-sort so pinned entries come first
	m.recentTaskVisits = m.sortedQuickSwitchEntries()
	m.rebuildQuickSwitchFilter()
}

// quickSwitchSelectFiltered selects the entry at the given filtered index.
func (m *AppModel) quickSwitchSelectFiltered(filteredIdx int) (*AppModel, tea.Cmd) {
	if filteredIdx < 0 || filteredIdx >= m.quickSwitchFilteredLen() {
		m.showQuickSwitch = false
		return m, nil
	}
	realIdx := m.quickSwitchFiltered[filteredIdx]
	return m.quickSwitchSelect(realIdx)
}

// quickSwitchSelect navigates to the entry at the given index and closes the popup.
func (m *AppModel) quickSwitchSelect(idx int) (*AppModel, tea.Cmd) {
	if idx < 0 || idx >= len(m.recentTaskVisits) {
		m.showQuickSwitch = false
		return m, nil
	}
	entry := m.recentTaskVisits[idx]
	m.showQuickSwitch = false
	return m, m.navigateToTaskInProject(entry.Task, entry.Project)
}
