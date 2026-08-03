package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/storage"
)

// handleUndoToggle reverses the most recent q/w planning toggle as one logical
// action (workflow state + Next order index / Daily Plan entry). It fires
// wherever a toggle was just recorded — the same task views where q/w work — so
// an immediate u undoes the toggle atomically instead of falling through to
// ordinary project-history undo (which would restore the state but leave the
// planning order/list stale). The localToggleUndoIsNewest guard still yields to
// a newer persistent mutation.
func (m *AppModel) handleUndoToggle() tea.Cmd {
	if m.lastToggleUndoKind == toggleUndoNone {
		return nil
	}
	if !m.currentViewOwnsPlanningToggle() {
		return nil
	}
	if !m.localToggleUndoIsNewest() {
		return nil
	}
	var cmd tea.Cmd
	switch m.lastToggleUndoKind {
	case toggleUndoNone:
	case toggleUndoToday:
		cmd = m.undoTodayToggle()
	case toggleUndoQueue:
		cmd = m.undoQueueToggle()
	}
	if cmd != nil {
		return cmd
	}
	return nil
}

// handleRedoToggle replays the most recently undone q/w planning toggle, mirror
// of handleUndoToggle. Reachable from every view where the toggle is reachable,
// so ctrl+r replays both sides of the logical action without navigating away.
func (m *AppModel) handleRedoToggle() tea.Cmd {
	if m.lastToggleRedoKind == toggleUndoNone {
		return nil
	}
	if !m.currentViewOwnsPlanningToggle() {
		return nil
	}
	if !m.localToggleRedoIsNewest() {
		return nil
	}
	var cmd tea.Cmd
	switch m.lastToggleRedoKind {
	case toggleUndoNone:
	case toggleUndoToday:
		cmd = m.redoTodayToggle()
	case toggleUndoQueue:
		cmd = m.redoQueueToggle()
	}
	if cmd != nil {
		return cmd
	}
	return nil
}

// currentViewOwnsPlanningToggle reports whether the current view is one where a
// q/w planning toggle is reachable (a QueueToggleable task view, including the
// planning surface). The bespoke atomic toggle undo/redo only fires from such a
// view so pressing u/ctrl+r elsewhere does not silently
// reverse a planning action taken in another view.
func (m *AppModel) currentViewOwnsPlanningToggle() bool {
	_, ok := m.currentViewAsModel().(QueueToggleable)
	return ok
}

func (m *AppModel) localToggleUndoIsNewest() bool {
	return m.localToggleIsNewest(m.toggleUndoTimestamp(m.lastToggleUndoKind), false)
}

func (m *AppModel) localToggleRedoIsNewest() bool {
	return m.localToggleIsNewest(m.toggleRedoTimestamp(m.lastToggleRedoKind), true)
}

func (m *AppModel) localToggleIsNewest(localAt time.Time, redo bool) bool {
	if localAt.IsZero() {
		return true
	}
	entry, ok, err := ledgerNewestEntry(workspaceLedgerFromConfig(m.runtime), storage.GlobalScope(), redo)
	if err != nil || !ok {
		return true
	}
	return !entry.Timestamp.After(localAt)
}
