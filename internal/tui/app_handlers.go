package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// reloadAndRefreshProjectTree reloads project from storage and refreshes the tree.
// Preserves filter/sort state across the reload.
func (m *AppModel) reloadAndRefreshProjectTree() {
	if m.projectTree != nil && m.currentProject != nil {
		m.switchToProject(m.currentProject, true)
	}
}

// applyTagsToTask applies selected tags to a single task and saves the project.
func (m *AppModel) applyTagsToTask(task *models.Task, project *models.Project, selectedTags []string) tea.Cmd {
	// Update task tags
	task.Tags = selectedTags
	task.UpdatedAt = time.Now()

	// Create backup and save
	if err := saveProjectSync(project); err != nil {
		return errorCmd("save failed: %w", err)
	}

	// Refresh tree and set undo context
	if m.projectTree != nil {
		m.projectTree.setUndoContext("Set Tags", fmt.Sprintf("#%d %s", task.ShortID, task.Title))
		m.projectTree.buildTree()
	}

	return nil
}

// applyTagsToTasks applies selected tags to multiple tasks and saves the project.
func (m *AppModel) applyTagsToTasks(tasks []*models.Task, project *models.Project, selectedTags []string) tea.Cmd {
	// Update all tasks with the selected tags
	for _, task := range tasks {
		task.Tags = selectedTags
		task.UpdatedAt = time.Now()
	}

	// Create backup and save
	if err := saveProjectSync(project); err != nil {
		return errorCmd("save failed: %w", err)
	}

	// Refresh tree, clear selection, and set undo context
	if m.projectTree != nil {
		m.projectTree.clearSelection()
		m.projectTree.setUndoContext("Set Tags", fmt.Sprintf("%d tasks", len(tasks)))
		m.projectTree.buildTree()
	}

	return nil
}

// processMessage routes a message through the app's handler chain.
// Used by CommandPaletteExecuteMsg to process the inner message synchronously.
func (m *AppModel) processMessage(msg tea.Msg) (bool, tea.Cmd) {
	// Handle QuitMsg directly since it's not in the handler chains
	if _, ok := msg.(QuitMsg); ok {
		m.saveProjectTreeState()
		m.quitting = true
		return true, tea.Quit
	}
	if handled, cmd := m.handleProjectMessages(msg); handled {
		return true, cmd
	}
	if handled, cmd := m.handleTaskMessages(msg); handled {
		return true, cmd
	}
	if handled, cmd := m.handleViewMessages(msg); handled {
		return true, cmd
	}
	if handled, cmd := m.handleTagMessages(msg); handled {
		return true, cmd
	}
	if handled, cmd := m.handleSectionMessages(msg); handled {
		return true, cmd
	}
	if handled, cmd := m.handleInboxMessages(msg); handled {
		return true, cmd
	}
	if handled, cmd := m.handleClipboardMessages(msg); handled {
		return true, cmd
	}
	if handled, cmd := m.handleProjectCycleMessages(msg); handled {
		return true, cmd
	}
	return false, nil
}
