package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/storage"
)

// handleMoveTaskToProject moves a task and its subtree to another project as ONE
// atomic application-layer transaction. The source removal, destination
// insertion, each moved task's attachment directory, and the durable planning
// refs (Focus Queue, Daily Plan) reassigned to the destination all commit
// together through app.Service.MoveToProject — replacing the retired
// saveBothProjectsAsync orchestration that persisted the two projects and their
// per-project .undo snapshots as separate raw saves. Machine-local recent visits
// are non-authoritative UI state repaired separately after the commit succeeds.
func (m *AppModel) handleMoveTaskToProject(msg MoveTaskToProjectMsg) (bool, tea.Cmd) {
	if msg.Task == nil || msg.SourceProject == nil || msg.DestProjectID == "" {
		return true, nil
	}
	sourceTask := msg.SourceProject.GetTaskByID(msg.Task.ID)
	if sourceTask == nil {
		return true, errorCmd("failed to move task: task %q is not in source project", msg.Task.ID)
	}
	destProject := m.config.GetProject(msg.DestProjectID)
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

	// Fold the durable planning refs (Focus Queue, Daily Plan) reassigned to the
	// destination into the move transaction as a config artifact, so a partial move
	// can never leave planning refs pointing at the old project. Recent visits are
	// excluded here and repaired separately after the commit.
	artifacts, err := m.moveConfigArtifacts(movedSet, destProject.ID)
	if err != nil {
		return true, errorCmd("failed to prepare move: %w", err)
	}

	var res app.MoveResult
	if msg.DestSection != nil {
		res, err = m.appService.MoveToProjectInSection(sourceTask.ID, destProject.ID, *msg.DestSection, artifacts...)
	} else {
		res, err = m.appService.MoveToProject(sourceTask.ID, destProject.ID, artifacts...)
	}
	if err != nil {
		return true, errorCmd("failed to move task: %w", err)
	}

	// Reload both projects from disk so the AppModel's in-memory copies reflect the
	// committed transaction before the shared post-save refresh runs.
	m.reloadProjectByID(destProject.ID)
	m.reloadProjectByID(msg.SourceProject.ID)

	// Repair the machine-local recent visits separately — non-authoritative UI state
	// updated only after the durable commit succeeded. A failure here is a warning,
	// not an error: the move is already committed.
	if m.config.ReassignRecentVisitsProject(movedSet, destProject.ID) > 0 && m.configManager != nil {
		if saveErr := m.configManager.Save(m.config); saveErr != nil {
			logging.Warn("failed to update recent visits after move: %v", saveErr)
		}
	}

	// Update in-memory TUI focus queue, daily plan, and recent-visit references so
	// open planning views point at the destination project immediately.
	for i := range m.focusQueue {
		if movedSet[m.focusQueue[i].Task.ID] {
			m.focusQueue[i].Project = destProject
		}
	}
	for i := range m.dailyPlan {
		if movedSet[m.dailyPlan[i].Task.ID] {
			m.dailyPlan[i].Project = destProject
		}
	}
	for i := range m.recentTaskVisits {
		if movedSet[m.recentTaskVisits[i].Task.ID] {
			m.recentTaskVisits[i].Project = destProject
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
// Plan) for the moved subtree to the destination project and encodes the updated
// config as a single ArtifactChange folded into the move transaction, mirroring
// the CLI cross-project move. Returns nil when nothing changed.
func (m *AppModel) moveConfigArtifacts(movedSet map[string]bool, destProjectID string) ([]storage.ArtifactChange, error) {
	if m.config == nil || m.configManager == nil {
		return nil, nil
	}
	if m.config.ReassignDurablePlanningProject(movedSet, destProjectID) == 0 {
		return nil, nil
	}
	return configDomainArtifacts(m.configManager, m.config)
}

// handleTaskMovedToProject handles the result of a cross-project task move.
func (m *AppModel) handleTaskMovedToProject(msg TaskMovedToProjectMsg) (bool, tea.Cmd) {
	m.isLoading = false
	m.loadingMessage = ""
	if !msg.FromGlobal {
		m.popView() // Close project selector (not needed for global flow)
	}
	m.refreshAllViews()
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
