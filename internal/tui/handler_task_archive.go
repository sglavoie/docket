package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// handleArchiveCompletedTasks archives completed tasks from msg.Project if set,
// otherwise from the current project.
func (m *AppModel) handleArchiveCompletedTasks(msg ArchiveCompletedTasksMsg) (bool, tea.Cmd) {
	project := msg.Project
	if project == nil {
		project = m.currentProject
	}
	if project == nil {
		return true, nil
	}
	persister := m.persister
	undoDepth := 0
	if m.config.UndoHistoryDepth != nil {
		undoDepth = *m.config.UndoHistoryDepth
	}

	cmd := func() tea.Msg {
		completedCount := project.CountCompletedTasks()
		if completedCount == 0 {
			return TasksArchivedMsg{Count: 0}
		}

		// Record undo snapshot before archiving
		if undoDepth > 0 {
			_ = persister.RecordUndoSnapshot(project, "Archive Tasks",
				fmt.Sprintf("%d tasks", completedCount), undoDepth, nil)
		}

		// Create backup before archiving
		if err := persister.Backup(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}

		// Archive the tasks
		count, err := persister.ArchiveTasks(project)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to archive tasks: %w", err)}
		}

		// Reload project
		_ = persister.Load(project)

		return TasksArchivedMsg{Count: count}
	}

	return true, cmd
}

// handleArchiveAllCompletedTasks archives completed tasks across all active projects.
func (m *AppModel) handleArchiveAllCompletedTasks() (bool, tea.Cmd) {
	projects := m.config.GetActiveProjects()
	persister := m.persister
	currentProject := m.currentProject

	cmd := func() tea.Msg {
		totalArchived := 0
		projectsAffected := 0

		for _, project := range projects {
			if err := persister.Load(project); err != nil {
				continue
			}

			if project.CountCompletedTasks() == 0 {
				continue
			}

			if err := persister.Backup(project); err != nil {
				continue
			}

			count, err := persister.ArchiveTasks(project)
			if err != nil {
				continue
			}

			totalArchived += count
			projectsAffected++
		}

		// Reload current project if it was affected
		if currentProject != nil {
			_ = persister.Load(currentProject)
		}

		return TasksArchivedMsg{Count: totalArchived, Projects: projectsAffected}
	}

	return true, cmd
}
