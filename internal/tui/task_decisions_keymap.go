package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

func (m *TaskDecisionsModel) panelActions() taskItemPanelActions {
	return taskItemPanelActions{
		model:       m,
		itemName:    "decision",
		inInputMode: func() bool { return m.addMode || m.editMode || m.expandedView },
		moveCursorDown: func() {
			m.moveCursorDown()
		},
		moveCursorUp: func() {
			m.moveCursorUp()
		},
		moveCursorBottom: func() {
			m.moveCursorBottom()
		},
		jumpToTop: func() {
			m.cursor = 0
		},
		beginAdd:     m.beginAddMode,
		hasSelection: m.hasSelection,
		startEdit:    m.startEditDecision,
		editExternal: m.editInExternalEditor,
		showDeleteConfirm: func() {
			m.confirm.Show("delete", "Delete decision?")
		},
		showIndexedDeleteConfirm: func() {
			m.confirm.Show("delete", fmt.Sprintf("Delete decision [%d]?", m.cursor))
		},
		toggleResolved: m.toggleResolved,
		copyText:       m.copyDecisionText,
		expandSelection: func() {
			m.expandedView = true
		},
		close: func() tea.Msg { return CloseTaskDecisionsMsg{} },
	}
}

// buildKeyMap constructs the declarative KeyMap for TaskDecisionsModel.
func (m *TaskDecisionsModel) buildKeyMap() *keymap.KeyMap {
	return buildTaskItemPanelKeyMap("Decisions", m.panelActions(),
		keymap.Bind("tab", "Cycle field (text/rationale)", "Actions", func() (tea.Model, tea.Cmd) {
			if m.editMode {
				m.editActiveField = 1 - m.editActiveField
				return m, m.focusEditField()
			}
			if m.addMode {
				m.activeField = 1 - m.activeField
				return m, m.focusAddField()
			}
			return m, nil
		}),
		keymap.Bind("ctrl+y", "Cycle category", "Actions", func() (tea.Model, tea.Cmd) {
			if m.editMode {
				cats := allDecisionCategories()
				m.editCategoryIdx = (m.editCategoryIdx + 1) % len(cats)
			} else if m.addMode {
				cats := allDecisionCategories()
				m.categoryIndex = (m.categoryIndex + 1) % len(cats)
			}
			return m, nil
		}),
		keymap.Bind("ctrl+s", "Save (add/edit mode)", "Actions", func() (tea.Model, tea.Cmd) {
			if m.editMode {
				return m, m.saveEditedDecision()
			}
			if m.addMode {
				return m, m.saveDecision()
			}
			return m, nil
		}),
		keymap.Bind("ctrl+e", "Open in external editor (add/edit mode)", "Actions", func() (tea.Model, tea.Cmd) {
			if m.editMode {
				return m, m.editDecisionInExternalEditorFromEditMode()
			}
			if m.addMode {
				return m, m.addDecisionInExternalEditor()
			}
			return m, nil
		}),
	)
}
