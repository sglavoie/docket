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
	addMode := func() bool { return m.addMode && !m.confirm.IsActive() }
	editMode := func() bool { return m.editMode && !m.confirm.IsActive() }
	expandedMode := func() bool { return m.expandedView && !m.confirm.IsActive() }
	saveLabel := "save"
	if m.err != nil {
		saveLabel = "retry"
	}
	return buildTaskItemPanelKeyMap(m.panelActions(),
		keymap.Bind("ctrl+s", "Save comment", "Actions", keymap.WithFooter(1, saveLabel, true), keymap.WithFooterKey("Ctrl+S"), keymap.WithGate(func() bool {
			return addMode() || editMode()
		})),
		keymap.Bind("ctrl+e", "Open in external editor", "Actions", keymap.WithFooter(3, "editor", false), keymap.WithFooterKey("Ctrl+E"), keymap.WithGate(func() bool {
			return addMode() || editMode()
		})),
		keymap.Bind("shift+enter", "Insert newline", "Actions", keymap.WithFooter(4, "newline", false), keymap.WithFooterKey("Shift+Enter"), keymap.WithGate(func() bool {
			return addMode() || editMode()
		})),
		keymap.Bind("esc", "Cancel comment input", "Navigation", keymap.WithFooter(2, "cancel", true), keymap.WithFooterKey("Esc"), keymap.WithGate(func() bool {
			return addMode() || editMode()
		})),
		keymap.Bind("enter", "Close expanded comment", "Navigation", keymap.WithFooter(1, "close", true), keymap.WithFooterKey("Enter"), keymap.WithGate(expandedMode)),
		keymap.Bind("esc", "Close expanded comment", "Navigation", keymap.WithFooter(2, "close", true), keymap.WithFooterKey("Esc"), keymap.WithGate(expandedMode)),
	)
}
