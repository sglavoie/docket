package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// handleConfirmDialog processes key input while a confirmation dialog is active.
//
// Task deletion is shell-owned, so this handler only owns Outliner-local
// confirmations with no shell equivalent (project archive and archive-completed).
func (m *OutlinerModel) handleConfirmDialog(key string) (tea.Model, tea.Cmd) {
	// Capture action before HandleKey clears it on confirm/cancel.
	action := m.confirm.Action()

	// Standard y/n handling
	if confirmed, handled := m.HandleConfirmKey(key); handled {
		if confirmed {
			switch action {
			case projectArchiveConfirmAction:
				return m, func() tea.Msg { return projectArchiveConfirmedMsg{} }
			case "archive-tasks":
				return m.archiveCompletedTasks()
			}
		}
		m.archiveTargetProject = nil
		return m, nil
	}
	return m, nil
}

// archiveCompletedTasks moves all completed tasks to the archive file.
// The target project is m.archiveTargetProject when set (used by cross-project
// mode to capture the cursor's owning project at confirmation time); otherwise
// it falls back to m.project for single-project mode.
func (m *OutlinerModel) archiveCompletedTasks() (tea.Model, tea.Cmd) {
	proj := m.archiveTargetProject
	m.archiveTargetProject = nil
	if proj == nil {
		proj = m.project
	}
	if proj == nil {
		return m, nil
	}

	undoDepth := 0
	if m.runtime.Settings().UndoHistoryDepth != nil {
		undoDepth = *m.runtime.Settings().UndoHistoryDepth
	}
	completedTasks := proj.GetCompletedTasks()
	completedCount := len(completedTasks)

	if completedCount == 0 {
		return m, nil
	}

	svc := m.projectService
	cmd := func() tea.Msg {
		if undoDepth > 0 {
			_ = svc.RecordUndoSnapshot(proj, "Archive Tasks",
				fmt.Sprintf("%d tasks", completedCount), undoDepth, nil)
		}
		if err := svc.Backup(proj); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}
		count, err := svc.ArchiveTasks(proj)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to archive tasks: %w", err)}
		}
		_ = svc.Load(proj)
		// Note: rebuildRows() is called by TasksArchivedMsg handler
		return TasksArchivedMsg{Count: count, ArchivedTaskLocators: taskLocatorsForProjectTasks(proj.ID, completedTasks)}
	}
	m.clearStatus()
	return m, cmd
}
