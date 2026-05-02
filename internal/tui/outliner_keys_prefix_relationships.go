package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleBlockerPrefixOutliner handles the b prefix key for blocker operations.
func (m *OutlinerModel) handleBlockerPrefixOutliner(key string, cursorTask *models.Task) (tea.Model, tea.Cmd) {
	if m.project == nil {
		row, proj, valid := m.crossGetMutationTarget()
		if !valid {
			return m, nil
		}
		if handled, cmd := handleBlockerPrefixKey(key, row.task, proj); handled {
			return m, cmd
		}
		return m, nil
	}
	if cursorTask == nil {
		return m, nil
	}
	if handled, cmd := handleBlockerPrefixKey(key, cursorTask, m.project); handled {
		return m, cmd
	}
	return m, nil
}

// handleLinkedTaskPrefixOutliner handles the c prefix key for linked task operations.
func (m *OutlinerModel) handleLinkedTaskPrefixOutliner(key string, cursorTask *models.Task) (tea.Model, tea.Cmd) {
	if m.project == nil {
		row, proj, valid := m.crossGetMutationTarget()
		if !valid {
			return m, nil
		}
		if handled, cmd := handleLinkedTaskPrefixKey(key, row.task, proj, true, true); handled {
			return m, cmd
		}
		return m, nil
	}
	if cursorTask == nil {
		return m, nil
	}
	if handled, cmd := handleLinkedTaskPrefixKey(key, cursorTask, m.project, true, true); handled {
		return m, cmd
	}
	return m, nil
}
