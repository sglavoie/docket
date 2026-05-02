package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleSectionMovePrefixOutliner handles the m prefix key for section move operations.
func (m *OutlinerModel) handleSectionMovePrefixOutliner(key string, cursorTask *models.Task) (tea.Model, tea.Cmd) {
	if m.project == nil {
		return m.crossHandleSectionMove(key, cursorTask)
	}
	if model, cmd, handled := handleSectionMovePrefix(key, SectionMoveCallbacks{
		MoveToSection: func(section string) (tea.Model, tea.Cmd) {
			return m.sendTaskToSection(cursorTask, section)
		},
		BulkMoveToSection: func(section string) (tea.Model, tea.Cmd) {
			return m.sendTasksToSectionBulk(section)
		},
		MoveToTop: func() (tea.Model, tea.Cmd) {
			if cursorTask == nil {
				return m, nil
			}
			return m.moveTaskToPosition(cursorTask, true)
		},
		MoveToBottom: func() (tea.Model, tea.Cmd) {
			if cursorTask == nil {
				return m, nil
			}
			return m.moveTaskToPosition(cursorTask, false)
		},
		IsMultiSelect: func() bool { return m.multiSelectMode && len(m.selectedTasks) > 0 },
	}); handled {
		return model, cmd
	}

	return m, nil
}
