package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

func (m *TaskCommentsModel) panelActions() taskItemPanelActions {
	return taskItemPanelActions{
		model:       m,
		itemName:    "comment",
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
		startEdit:    m.startEditComment,
		editExternal: m.editInExternalEditor,
		showDeleteConfirm: func() {
			m.confirm.Show("delete", "Delete comment?")
		},
		showIndexedDeleteConfirm: func() {
			m.confirm.Show("delete", fmt.Sprintf("Delete comment [%d]?", m.cursor))
		},
		toggleResolved: m.toggleResolved,
		copyText:       m.copyCommentText,
		expandSelection: func() {
			m.expandedView = true
		},
		close: func() tea.Msg { return CloseTaskCommentsMsg{} },
	}
}

// buildKeyMap constructs the declarative KeyMap for TaskCommentsModel.
func (m *TaskCommentsModel) buildKeyMap() *keymap.KeyMap {
	return buildTaskItemPanelKeyMap("Comments", m.panelActions(),
		keymap.Bind("ctrl+s", "Save (add/edit mode)", "Actions", func() (tea.Model, tea.Cmd) {
			if m.editMode {
				return m, m.saveEditedComment()
			}
			if m.addMode {
				return m, m.saveComment()
			}
			return m, nil
		}),
		keymap.Bind("ctrl+e", "Open in external editor (add/edit mode)", "Actions", func() (tea.Model, tea.Cmd) {
			if m.editMode {
				return m, m.editCommentInExternalEditorFromEditMode()
			}
			if m.addMode {
				return m, m.addCommentInExternalEditor()
			}
			return m, nil
		}),
	)
}
