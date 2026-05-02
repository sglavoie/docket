package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// handleTaskUndoMessages handles task undo/redo messages.
// Returns true if the message was handled.
func (m *AppModel) handleTaskUndoMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowUndoHistoryMsg:
		return m.handleShowUndoHistory(msg)

	case UndoPerformedMsg:
		return m.handleUndoPerformed(msg)

	case RedoPerformedMsg:
		return m.handleRedoPerformed(msg)
	}
	return false, nil
}

// handleShowUndoHistory handles the ShowUndoHistoryMsg.
func (m *AppModel) handleShowUndoHistory(msg ShowUndoHistoryMsg) (bool, tea.Cmd) {
	if msg.Project != nil {
		m.showView(ViewUndoHistory, NewUndoHistoryModel(msg.Project))
	}
	return true, nil
}

// handleUndoPerformed handles the UndoPerformedMsg.
func (m *AppModel) handleUndoPerformed(msg UndoPerformedMsg) (bool, tea.Cmd) {
	fromViewer := m.currentView() == ViewUndoHistory
	undoMsg := fmt.Sprintf("Undid: %s", msg.Action)
	cmd := m.handleWithRefresh(false, false, undoMsg)
	if fromViewer {
		if v := m.views.Get(ViewUndoHistory); v != nil {
			v.(*UndoHistoryModel).reloadEntries()
		}
	}
	return true, cmd
}

// handleRedoPerformed handles the RedoPerformedMsg.
func (m *AppModel) handleRedoPerformed(msg RedoPerformedMsg) (bool, tea.Cmd) {
	fromViewer := m.currentView() == ViewUndoHistory
	redoMsg := fmt.Sprintf("Redo: %s", msg.Action)
	cmd := m.handleWithRefresh(false, false, redoMsg)
	if fromViewer {
		if v := m.views.Get(ViewUndoHistory); v != nil {
			v.(*UndoHistoryModel).reloadEntries()
		}
	}
	return true, cmd
}
