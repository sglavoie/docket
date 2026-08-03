package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

func (m *TaskCommentsModel) panelActions() taskItemPanelActions {
	return taskItemPanelActions{
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
		showIndexedDeleteConfirm: func() {
			m.confirm.Show("delete", fmt.Sprintf("Delete comment [%d]?", m.cursor))
		},
		copyText:    m.copyCommentText,
		copyAllText: m.copyAllCommentsText,
		expandSelection: func() {
			m.expandedView = true
		},
		close: func() tea.Msg { return CloseTaskCommentsMsg{} },
	}
}

// buildKeyMap constructs the declarative KeyMap for TaskCommentsModel.
func (m *TaskCommentsModel) buildKeyMap() *keymap.KeyMap {
	return buildTaskItemPanelKeyMap(m.panelActions(),
		keymap.Bind("ctrl+s", "Save (add/edit mode)", "Actions"),

		keymap.Bind("ctrl+e", "Open in external editor (add/edit mode)", "Actions"),
	)
}
