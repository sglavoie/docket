package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleMoveTaskToProject handles moving a task to another project.
func (m *AppModel) handleMoveTaskToProject(msg MoveTaskToProjectMsg) (bool, tea.Cmd) {
	if msg.Task == nil || msg.SourceProject == nil || msg.DestProjectID == "" {
		return true, nil
	}

	// Get destination project
	destProject := m.config.GetProject(msg.DestProjectID)
	if destProject == nil {
		return true, errorCmd("destination project not found")
	}

	// Load destination project
	if err := m.persister.Load(destProject); err != nil {
		return true, errorCmd("failed to load destination project: %w", err)
	}

	// Store old short ID before move
	oldShortID := msg.Task.ShortID

	// Perform the move
	result, err := msg.SourceProject.MoveTaskToProject(msg.Task, destProject)
	if err != nil {
		return true, errorCmd("failed to move task: %w", err)
	}
	if result == nil {
		return true, nil
	}

	// Update persisted FocusQueue, DailyPlan, and RecentTaskVisits to reference the new project
	movedTaskIDs := make(map[string]bool, len(result.MovedTasks))
	for _, t := range result.MovedTasks {
		movedTaskIDs[t.ID] = true
	}
	if m.config.ReassignTaskProject(movedTaskIDs, destProject.ID) > 0 {
		if m.configManager != nil {
			_ = m.configManager.Save(m.config)
		}
	}

	// Update in-memory TUI focus queue and daily plan references
	for i := range m.focusQueue {
		if movedTaskIDs[m.focusQueue[i].Task.ID] {
			m.focusQueue[i].Project = destProject
		}
	}
	for i := range m.dailyPlan {
		if movedTaskIDs[m.dailyPlan[i].Task.ID] {
			m.dailyPlan[i].Project = destProject
		}
	}
	for i := range m.recentTaskVisits {
		if movedTaskIDs[m.recentTaskVisits[i].Task.ID] {
			m.recentTaskVisits[i].Project = destProject
		}
	}

	// Save both projects
	return true, tea.Batch(
		startLoading("Moving task..."),
		m.saveBothProjectsAsync(msg.SourceProject, destProject, result, oldShortID),
	)
}

// saveBothProjectsAsync saves both source and destination projects after a cross-project move.
func (m *AppModel) saveBothProjectsAsync(source, dest *models.Project, result *models.MoveTaskToProjectResult, oldShortID int) tea.Cmd {
	persister := m.persister
	return func() tea.Msg {
		// Backup and save source project
		if err := persister.Backup(source); err != nil {
			return SaveErrorMsg{Err: fmt.Errorf("failed to backup source project: %w", err)}
		}
		if err := persister.Save(source); err != nil {
			return SaveErrorMsg{Err: fmt.Errorf("failed to save source project: %w", err)}
		}

		// Backup and save destination project
		if err := persister.Backup(dest); err != nil {
			return SaveErrorMsg{Err: fmt.Errorf("failed to backup destination project: %w", err)}
		}
		if err := persister.Save(dest); err != nil {
			return SaveErrorMsg{Err: fmt.Errorf("failed to save destination project: %w", err)}
		}

		// Return success message with move details
		var movedTask *models.Task
		if len(result.MovedTasks) > 0 {
			movedTask = result.MovedTasks[0]
		}

		return TaskMovedToProjectMsg{
			Task:          movedTask,
			SourceProject: source,
			DestProject:   dest,
			MovedCount:    len(result.MovedTasks),
			ClearedDeps:   len(result.ClearedDeps),
			OldShortID:    oldShortID,
		}
	}
}

// handleTaskMovedToProject handles the result of a cross-project task move.
func (m *AppModel) handleTaskMovedToProject(msg TaskMovedToProjectMsg) (bool, tea.Cmd) {
	m.isLoading = false
	m.loadingMessage = ""
	m.popView() // Close project selector
	m.refreshProjectTree()
	if m.outliner != nil {
		m.outliner.rebuildRows()
	}
	// Also refresh the focus board so the moved task disappears from its old position
	if v := m.views.Get(ViewFocusBoard); v != nil {
		v.Update(FocusBoardRefreshMsg{})
	}
	var statusMsg string
	if msg.MovedCount > 1 {
		statusMsg = fmt.Sprintf("Moved #%d and %d descendants to %s", msg.OldShortID, msg.MovedCount-1, msg.DestProject.Name)
	} else {
		statusMsg = fmt.Sprintf("Moved #%d to %s", msg.OldShortID, msg.DestProject.Name)
	}
	if msg.ClearedDeps > 0 {
		statusMsg += fmt.Sprintf(" (%d deps cleared)", msg.ClearedDeps)
	}
	m.setSuccessStatus(statusMsg)
	return true, statusTimeout(3 * time.Second)
}
