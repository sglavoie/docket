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
		m.showQuickSwitch = false
		return nil
	}
	m.openQuickSwitch()
	if len(m.recentTaskVisits) == 0 {
		m.showQuickSwitch = false
		m.setInfoStatus("No recent tasks for quick-switch")
		return defaultStatusTimeout()
	}
	return nil
}

func (m *AppModel) handlePinnedPopupToggle() tea.Cmd {
	m.pinnedPopupItems = m.collectPinnedTasks()
	if len(m.pinnedPopupItems) == 0 {
		m.setInfoStatus("No pinned tasks — press tp on a task to pin it")
		return defaultStatusTimeout()
	}
	m.showPinnedPopup = !m.showPinnedPopup
	m.pinnedPopupCursor = 0
	m.pinnedPopupFilter = ""
	m.pinnedPopupFiltering = false
	m.rebuildPinnedPopupFilter()
	return nil
}

func (m *AppModel) handleUndoToggle() tea.Cmd {
	switch m.currentView() { //nolint:exhaustive
	case ViewFocusView, ViewFocusBoard:
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
			return m.batchWithFocusRefresh(cmd)
		}
	}
	return nil
}

func (m *AppModel) handleRedoToggle() tea.Cmd {
	switch m.currentView() { //nolint:exhaustive
	case ViewFocusView, ViewFocusBoard:
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
			return m.batchWithFocusRefresh(cmd)
		}
	}
	return nil
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
