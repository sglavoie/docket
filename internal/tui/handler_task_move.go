package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
)

// handleMoveTaskToProject moves a task and its subtree to another project as ONE
// atomic application-layer transaction. The source removal, destination
// insertion, each moved task's attachment directory, and the durable planning
// refs (Focus Queue, Daily Plan) reassigned to the destination all commit
// together through app.Service.MoveToProject.
func (m *AppModel) handleMoveTaskToProject(msg MoveTaskToProjectMsg) (bool, tea.Cmd) {
	if msg.Task == nil || msg.SourceProject == nil || msg.DestProjectID == "" {
		return true, nil
	}
	sourceTask := msg.SourceProject.GetTaskByID(msg.Task.ID)
	if sourceTask == nil {
		return true, errorCmd("failed to move task: task %q is not in source project", msg.Task.ID)
	}
	destProject := m.projectByID(msg.DestProjectID)
	if destProject == nil {
		return true, errorCmd("destination project not found")
	}
	oldShortID := sourceTask.ShortID
	m.appService.WithPlanningArtifacts(shellStructuralPlanningArtifactBuilder(m.runtime, m.configManager))

	// Build the moved set (root + descendants) from the loaded source project so the
	// durable planning reassignment can be computed before the application layer
	// performs the move. Task UUIDs are stable across the move, so this set keys the
	// config reassignment correctly regardless of the fresh snapshot the use case loads.
	msg.SourceProject.BuildTaskTree()
	movedSet := collectDescendantIDs(msg.SourceProject.Tasks, map[string]bool{sourceTask.ID: true})
	sourceProjectID := msg.SourceProject.ID

	locator := app.TaskLocator{ProjectID: sourceProjectID, TaskID: sourceTask.ID}
	res, err := m.appService.MoveToProjectByLocator(locator, destProject.ID)
	if err != nil {
		return true, errorCmd("failed to move task: %w", err)
	}

	// Reload both projects from disk so the AppModel's in-memory copies reflect the
	// committed transaction before the shared post-save refresh runs.
	m.reloadProjectByID(destProject.ID)
	m.reloadProjectByID(msg.SourceProject.ID)

	// Update in-memory planning references so open views point at the destination
	// project immediately.
	for i := range m.nextOrder {
		if m.nextOrder[i].Project != nil && m.nextOrder[i].Project.ID == sourceProjectID && movedSet[m.nextOrder[i].Task.ID] {
			m.nextOrder[i].Project = destProject
		}
	}
	for i := range m.today {
		if m.today[i].Project != nil && m.today[i].Project.ID == sourceProjectID && movedSet[m.today[i].Task.ID] {
			m.today[i].Project = destProject
		}
	}
	movedTask := destProject.GetTaskByID(sourceTask.ID)
	return true, func() tea.Msg {
		return TaskMovedToProjectMsg{
			Task:        movedTask,
			DestProject: destProject,
			MovedCount:  res.MovedCount,
			OldShortID:  oldShortID,
			FromGlobal:  msg.FromGlobal,
		}
	}
}

// handleTaskMovedToProject handles the result of a cross-project task move.
func (m *AppModel) handleTaskMovedToProject(msg TaskMovedToProjectMsg) (bool, tea.Cmd) {
	m.isLoading = false
	m.loadingMessage = ""
	var popCmd tea.Cmd
	if !msg.FromGlobal {
		popCmd = m.popView() // Close project selector (not needed for global flow)
	}
	m.refreshAllViews()
	// The cross-project move folded a checked config artifact (durable planning-ref
	// reassignment) into its transaction; advance the shared domain snapshot now
	// that it is committed so views reading it stay consistent.
	m.refreshRuntime()
	var statusMsg string
	if msg.MovedCount > 1 {
		statusMsg = fmt.Sprintf("Moved #%d and %d descendants to %s", msg.OldShortID, msg.MovedCount-1, msg.DestProject.Name)
	} else {
		statusMsg = fmt.Sprintf("Moved #%d to %s", msg.OldShortID, msg.DestProject.Name)
	}
	m.setSuccessStatus(statusMsg)
	reloadCmd := m.requestCrossProjectOutlinerReload()
	m.preserveMovedOutlinerTarget(msg)
	return true, tea.Batch(popCmd, reloadCmd, statusTimeout(3*time.Second))
}

func (m *AppModel) preserveMovedOutlinerTarget(msg TaskMovedToProjectMsg) {
	if !msg.FromGlobal || m.crossOutliner == nil || msg.Task == nil || msg.DestProject == nil {
		return
	}
	// reloadCrossProjects captured the old project-qualified cursor before
	// clearing rows. Replace it with the moved task's new qualified identity so
	// the task-menu operation returns to the same logical Outliner target.
	m.crossOutliner.crossReloadProjectID = msg.DestProject.ID
	m.crossOutliner.crossReloadTaskID = msg.Task.ID
}
