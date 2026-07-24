package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func (m *AppModel) handleQuickSwitchToggle() tea.Cmd {
	return m.toggleQuickSwitch()
}

func (m *AppModel) toggleQuickSwitch() tea.Cmd {
	if m.showQuickSwitch {
		m.closeQuickSwitchOverlay()
		return nil
	}
	m.openQuickSwitch()
	if len(m.recentTaskVisits) == 0 {
		m.closeQuickSwitchOverlay()
		m.setInfoStatus("No recent tasks for quick-switch")
		return defaultStatusTimeout()
	}
	m.pushTypedOverlay(OverlayRecent, "recent-tasks")
	return nil
}

func (m *AppModel) handlePinnedPopupToggle() tea.Cmd {
	m.pinnedPopupItems = m.collectPinnedTasks()
	if len(m.pinnedPopupItems) == 0 {
		m.setInfoStatus("No pinned tasks — press tp on a task to pin it")
		return defaultStatusTimeout()
	}
	m.showPinnedPopup = !m.showPinnedPopup
	if m.showPinnedPopup {
		m.pushTypedOverlay(OverlayPinned, "pinned-tasks")
	} else {
		m.closePinnedTasksOverlay()
	}
	m.pinnedPopupCursor = 0
	m.pinnedPopupFilter = ""
	m.pinnedPopupFiltering = false
	m.rebuildPinnedPopupFilter()
	return nil
}

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
	case toggleUndoDailyPlan:
		cmd = m.undoDailyPlanToggle()
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
	case toggleUndoDailyPlan:
		cmd = m.redoDailyPlanToggle()
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
// Planner cockpit). The bespoke atomic toggle undo/redo only fires from such a
// view so pressing u/ctrl+r elsewhere (e.g. the dashboard) does not silently
// reverse a planning action taken in another view.
func (m *AppModel) currentViewOwnsPlanningToggle() bool {
	_, ok := m.currentViewAsModel().(QueueToggleable)
	return ok
}

func (m *AppModel) localToggleUndoIsNewest() bool {
	return m.localToggleIsNewest(m.toggleUndoTimestamp(m.lastToggleUndoKind))
}

func (m *AppModel) localToggleRedoIsNewest() bool {
	return m.localToggleIsNewest(m.toggleRedoTimestamp(m.lastToggleRedoKind))
}

func (m *AppModel) localToggleIsNewest(localAt time.Time) bool {
	if localAt.IsZero() {
		return true
	}
	_, project := m.getCurrentCursorTaskAndProject()
	if project == nil {
		return true
	}
	persistentAt := latestPersistentTaskHistoryTimestamp(project)
	return persistentAt.IsZero() || !persistentAt.After(localAt)
}

func latestPersistentTaskHistoryTimestamp(project *models.Project) time.Time {
	var latest time.Time
	if entries, err := storage.LoadUndoHistory(project); err == nil {
		latest = latestUndoEntryTimestamp(latest, entries)
	}
	if entries, err := storage.LoadRedoHistory(project); err == nil {
		latest = latestUndoEntryTimestamp(latest, entries)
	}
	return latest
}

func latestUndoEntryTimestamp(latest time.Time, entries []storage.UndoEntry) time.Time {
	for _, entry := range entries {
		if entry.Timestamp.After(latest) {
			latest = entry.Timestamp
		}
	}
	return latest
}
