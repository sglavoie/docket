package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
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

	// Build the moved set (root + descendants) from the loaded source project so the
	// durable planning reassignment can be computed before the application layer
	// performs the move. Task UUIDs are stable across the move, so this set keys the
	// config reassignment correctly regardless of the fresh snapshot the use case loads.
	msg.SourceProject.BuildTaskTree()
	movedSet := collectDescendantIDs(msg.SourceProject.Tasks, map[string]bool{sourceTask.ID: true})
	sourceProjectID := msg.SourceProject.ID

	// Fold the durable planning refs (Focus Queue, Daily Plan) reassigned to the
	// destination into the move transaction as a config artifact, so a partial move
	// can never leave planning refs pointing at the old project. When a destination
	// section was chosen, the same artifact reconciles Today/Next exclusivity for the
	// moved root against its destination final state, so a move that lands the root in
	// Next drops any stale Today entry and a move out of Next drops any stale order
	// entry.
	artifacts, err := m.moveConfigArtifacts(movedSet, sourceProjectID, destProject.ID, sourceTask.ID, nil)
	if err != nil {
		return true, errorCmd("failed to prepare move: %w", err)
	}

	locator := app.TaskLocator{ProjectID: sourceProjectID, TaskID: sourceTask.ID}
	res, err := m.appService.MoveToProjectByLocator(locator, destProject.ID, artifacts...)
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
			Task:          movedTask,
			SourceProject: msg.SourceProject,
			DestProject:   destProject,
			MovedCount:    res.MovedCount,
			ClearedDeps:   len(res.ClearedDeps),
			OldShortID:    oldShortID,
			FromGlobal:    msg.FromGlobal,
		}
	}
}

// moveConfigArtifacts reassigns the durable planning refs (Focus Queue, Daily
// Plan) for the moved subtree to the destination project AND reconciles Today/Next
// exclusivity for the moved root against its destination final state, encoding the
// updated config as a single ArtifactChange folded into the move transaction,
// mirroring the CLI cross-project move. Reconciling in the same artifact as the
// reassignment keeps both planning deltas atomic with the task write. Returns nil
// when neither the reassignment nor the reconciliation changed anything.
func (m *AppModel) moveConfigArtifacts(movedSet map[string]bool, sourceProjectID, destProjectID, rootID string, finalState *models.WorkflowState) ([]storage.ArtifactChange, error) {
	if m.runtime == nil || m.configManager == nil {
		return nil, nil
	}
	planning := clonePlanningState(m.runtime.Planning())
	changed := planning.ReassignDurablePlanningProjectFrom(movedSet, sourceProjectID, destProjectID) > 0
	if finalState != nil && app.ReconcilePlanningState(planning, app.PlanningReconcileRequest{
		Targets: []app.PlanningReconcileTarget{{
			TaskID: rootID, ProjectID: destProjectID, State: *finalState,
		}},
	}) {
		changed = true
	}
	if !changed {
		return nil, nil
	}
	return planningDomainArtifact(m.configManager, planning)
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
	if msg.ClearedDeps > 0 {
		statusMsg += fmt.Sprintf(" (%d deps cleared)", msg.ClearedDeps)
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
