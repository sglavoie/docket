package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// saveOutlinerToggleState captures the current outliner's section/fold state
// into the per-project cache so it can be restored after recreation.
func (m *AppModel) saveOutlinerToggleState() {
	if m.outliner != nil && m.outliner.project != nil {
		m.outlinerToggleStates[m.outliner.project.ID] = m.outliner.SaveToggleState()
	}
}

// restoreOutlinerToggleState applies any cached section/fold state to the
// current outliner for its project.
func (m *AppModel) restoreOutlinerToggleState() {
	if m.outliner != nil && m.outliner.project != nil {
		if state, ok := m.outlinerToggleStates[m.outliner.project.ID]; ok {
			m.outliner.RestoreToggleState(state)
		}
	}
}

// reloadAndRefreshProjectTree reloads project from storage and refreshes the tree.
// Preserves filter/sort state across the reload.
func (m *AppModel) reloadAndRefreshProjectTree() {
	if m.projectTree != nil && m.currentProject != nil {
		m.switchToProject(m.currentProject, true)
	}
}

// applyTagsToTask applies selected tags to a single task and saves the project.
func (m *AppModel) applyTagsToTask(task *models.Task, project *models.Project, selectedTags []string) tea.Cmd {
	// Re-lookup task to avoid stale pointer after concurrent reload
	if !project.MutateTask(task.ID, func(t *models.Task) {
		t.Tags = selectedTags
	}) {
		return errorCmd("task no longer exists in project")
	}

	// Create backup and save
	if err := saveProjectSync(project); err != nil {
		return errorCmd("save failed: %w", err)
	}

	// Refresh tree/outliner and set undo context
	if m.projectTree != nil {
		m.projectTree.setUndoContext("Set Tags", fmt.Sprintf("#%d %s", task.ShortID, task.Title))
		m.projectTree.buildTree()
	}
	if m.outliner != nil {
		m.outliner.setUndoContext("Set Tags", fmt.Sprintf("#%d %s", task.ShortID, task.Title))
		m.outliner.rebuildRows()
	}

	return nil
}

// applyTagsToTasks applies selected tags to multiple tasks and saves the project.
func (m *AppModel) applyTagsToTasks(tasks []*models.Task, project *models.Project, selectedTags []string) tea.Cmd {
	// Update all tasks with the selected tags
	for _, task := range tasks {
		project.MutateTask(task.ID, func(t *models.Task) {
			t.Tags = selectedTags
		})
	}

	// Create backup and save
	if err := saveProjectSync(project); err != nil {
		return errorCmd("save failed: %w", err)
	}

	// Refresh tree/outliner, clear selection, and set undo context
	if m.projectTree != nil {
		m.projectTree.clearSelection()
		m.projectTree.setUndoContext("Set Tags", fmt.Sprintf("%d tasks", len(tasks)))
		m.projectTree.buildTree()
	}
	if m.outliner != nil {
		m.outliner.clearMultiSelect()
		m.outliner.setUndoContext("Set Tags", fmt.Sprintf("%d tasks", len(tasks)))
		m.outliner.rebuildRows()
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
