package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// handleArchiveCompletedTasks archives completed tasks from the current project.
func (m *AppModel) handleArchiveCompletedTasks() (bool, tea.Cmd) {
	if m.currentProject == nil {
		return true, nil
	}

	project := m.currentProject
	undoDepth := 0
	if m.config.UndoHistoryDepth != nil {
		undoDepth = *m.config.UndoHistoryDepth
	}
	completedCount := project.CountCompletedTasks()

	cmd := func() tea.Msg {
		if completedCount == 0 {
			return TasksArchivedMsg{Count: 0}
		}

		// Record undo snapshot before archiving
		if undoDepth > 0 {
			_ = m.persister.RecordUndoSnapshot(project, "Archive Tasks",
				fmt.Sprintf("%d tasks", completedCount), undoDepth)
		}

		// Create backup before archiving
		if err := m.persister.Backup(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}

		// Archive the tasks
		count, err := m.persister.ArchiveTasks(project)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to archive tasks: %w", err)}
		}

		// Reload project
		_ = m.persister.Load(project)

		return TasksArchivedMsg{Count: count}
	}

	return true, cmd
}

// handleArchiveAllCompletedTasks archives completed tasks across all active projects.
func (m *AppModel) handleArchiveAllCompletedTasks() (bool, tea.Cmd) {
	projects := m.config.GetActiveProjects()

	cmd := func() tea.Msg {
		totalArchived := 0
		projectsAffected := 0

		for _, project := range projects {
			if err := m.persister.Load(project); err != nil {
				continue
			}

			if project.CountCompletedTasks() == 0 {
				continue
			}

			if err := m.persister.Backup(project); err != nil {
				continue
			}

			count, err := m.persister.ArchiveTasks(project)
			if err != nil {
				continue
			}

			totalArchived += count
			projectsAffected++
		}

		// Reload current project if it was affected
		if m.currentProject != nil {
			_ = m.persister.Load(m.currentProject)
		}

		return TasksArchivedMsg{Count: totalArchived, Projects: projectsAffected}
	}

	return true, cmd
}
