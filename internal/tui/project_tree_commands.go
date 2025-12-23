package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func (m *ProjectTreeModel) archiveProject() tea.Cmd {
	return func() tea.Msg {
		m.project.Archive()

		// Clear current project if it was the archived one
		if m.config.CurrentProject == m.project.ID {
			m.config.CurrentProject = ""
		}

		if err := m.configManager.Save(m.config); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
		}

		return ProjectArchivedMsg{Project: m.project}
	}
}

func (m *ProjectTreeModel) unarchiveProject() tea.Cmd {
	return func() tea.Msg {
		m.project.Unarchive()

		if err := m.configManager.Save(m.config); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
		}

		return ProjectArchivedMsg{Project: m.project}
	}
}

// performUndo restores the project from backup.
func (m *ProjectTreeModel) performUndo() tea.Cmd {
	return func() tea.Msg {
		if !storage.BackupExists(m.project) {
			return ErrorMsg{Err: fmt.Errorf("no backup available to undo")}
		}

		tasksBefore := len(m.project.Tasks)

		if err := storage.RestoreFromBackup(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to undo: %w", err)}
		}

		if err := storage.LoadProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to reload after undo: %w", err)}
		}

		m.buildTree()
		return UndoCompletedMsg{
			TasksBefore: tasksBefore,
			TasksAfter:  len(m.project.Tasks),
		}
	}
}

// saveAndRefresh saves the project and refreshes the tree view.
func (m *ProjectTreeModel) saveAndRefresh() tea.Cmd {
	return func() tea.Msg {
		// Check for auto-unblocking before save
		m.project.CheckAndAutoUnblock()

		// Reorder root tasks so uncompleted tasks appear before completed tasks
		m.project.ReorderRootTasksByCompletion()

		// Create backup before saving for undo support
		if err := storage.BackupProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}

		if err := storage.SaveProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}
		storage.LoadProject(m.project)
		return treeRefreshMsg{}
	}
}

// deleteTask removes a task and its children from the project.
func (m *ProjectTreeModel) deleteTask() tea.Cmd {
	return func() tea.Msg {
		if m.deleteTarget == nil {
			return nil
		}

		task := m.deleteTarget
		m.deleteTarget = nil

		// Create backup before deleting for undo support
		if err := storage.BackupProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}

		// Collect IDs of task and all descendants to delete
		idsToDelete := make(map[string]bool)
		var collectIDs func(t *models.Task)
		collectIDs = func(t *models.Task) {
			idsToDelete[t.ID] = true
			for _, child := range t.Children {
				collectIDs(child)
			}
		}
		collectIDs(task)

		// Exit focus mode if focused task is being deleted
		if m.focusedTask != nil && idsToDelete[m.focusedTask.ID] {
			m.focusedTask = nil
		}

		// Filter out deleted tasks
		var remaining []*models.Task
		for _, t := range m.project.Tasks {
			if !idsToDelete[t.ID] {
				// Clear parent reference if parent was deleted
				if t.ParentID != nil && idsToDelete[*t.ParentID] {
					t.ParentID = nil
				}
				remaining = append(remaining, t)
			}
		}
		m.project.Tasks = remaining

		if err := storage.SaveProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		storage.LoadProject(m.project)
		m.buildTree()
		return TaskDeletedMsg{Task: task}
	}
}

// bulkDeleteTasks removes all selected tasks and their children from the project.
func (m *ProjectTreeModel) bulkDeleteTasks() tea.Cmd {
	return func() tea.Msg {
		if len(m.selectedTasks) == 0 {
			return nil
		}

		// Create backup before deleting for undo support
		if err := storage.BackupProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}

		// Collect all IDs to delete (selected tasks and their descendants)
		idsToDelete := make(map[string]bool)
		var collectIDs func(t *models.Task)
		collectIDs = func(t *models.Task) {
			idsToDelete[t.ID] = true
			for _, child := range t.Children {
				collectIDs(child)
			}
		}

		for _, task := range m.project.Tasks {
			if m.selectedTasks[task.ID] {
				collectIDs(task)
			}
		}

		// Exit focus mode if focused task is being deleted
		if m.focusedTask != nil && idsToDelete[m.focusedTask.ID] {
			m.focusedTask = nil
		}

		// Filter out deleted tasks
		var remaining []*models.Task
		for _, t := range m.project.Tasks {
			if !idsToDelete[t.ID] {
				// Clear parent reference if parent was deleted
				if t.ParentID != nil && idsToDelete[*t.ParentID] {
					t.ParentID = nil
				}
				remaining = append(remaining, t)
			}
		}
		m.project.Tasks = remaining

		if err := storage.SaveProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		m.clearSelection()
		storage.LoadProject(m.project)
		m.buildTree()
		m.clampCursor()
		return treeRefreshMsg{}
	}
}

// applyBulkTag adds a tag to all selected tasks.
func (m *ProjectTreeModel) applyBulkTag(tag string) {
	for _, task := range m.getSelectedTasks() {
		task.AddTag(tag)
	}
}
