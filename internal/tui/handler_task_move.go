package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
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

	// When the move flow picks a destination section, that section changes the moved
	// root's workflow state, so compute the state it will hold AFTER the move to
	// reconcile Today/Next against the true final state rather than the pre-move
	// state. A section-preserving move (no section chosen) changes no state and skips
	// the exclusivity reconciliation, leaving the reassignment behavior untouched.
	var finalState *models.WorkflowState
	if msg.DestSection != nil {
		probe := *sourceTask
		probe.SetSection(*msg.DestSection)
		st := probe.State()
		finalState = &st
	}

	// Fold the durable planning refs (Focus Queue, Daily Plan) reassigned to the
	// destination into the move transaction as a config artifact, so a partial move
	// can never leave planning refs pointing at the old project. When a destination
	// section was chosen, the same artifact reconciles Today/Next exclusivity for the
	// moved root against its destination final state, so a move that lands the root in
	// Next drops any stale Today entry and a move out of Next drops any stale order
	// entry. Recent visits are excluded here and repaired separately after the commit.
	artifacts, err := m.moveConfigArtifacts(movedSet, sourceProjectID, destProject.ID, sourceTask.ID, finalState)
	if err != nil {
		return true, errorCmd("failed to prepare move: %w", err)
	}

	var res app.MoveResult
	locator := app.TaskLocator{ProjectID: sourceProjectID, TaskID: sourceTask.ID}
	if msg.DestSection != nil {
		res, err = m.appService.MoveToProjectInSectionByLocator(locator, destProject.ID, *msg.DestSection, artifacts...)
	} else {
		res, err = m.appService.MoveToProjectByLocator(locator, destProject.ID, artifacts...)
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
	if err := m.commitMachineMutation(func(machine *config.MachineState) {
		machine.ReassignRecentVisitsProjectFrom(movedSet, sourceProjectID, destProject.ID)
	}); err != nil {
		logging.Warn("failed to update recent visits after move: %v", err)
	}

	// Update in-memory TUI focus queue, daily plan, and recent-visit references so
	// open planning views point at the destination project immediately.
	for i := range m.focusQueue {
		if m.focusQueue[i].Project != nil && m.focusQueue[i].Project.ID == sourceProjectID && movedSet[m.focusQueue[i].Task.ID] {
			m.focusQueue[i].Project = destProject
		}
	}
	for i := range m.dailyPlan {
		if m.dailyPlan[i].Project != nil && m.dailyPlan[i].Project.ID == sourceProjectID && movedSet[m.dailyPlan[i].Task.ID] {
			m.dailyPlan[i].Project = destProject
		}
	}
	for i := range m.recentTaskVisits {
		if m.recentTaskVisits[i].Project != nil && m.recentTaskVisits[i].Project.ID == sourceProjectID && movedSet[m.recentTaskVisits[i].Task.ID] {
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
	if finalState != nil && reconcilePlanningExclusivity(planning, []app.PlanningReconcileTarget{{
		TaskID:    rootID,
		ProjectID: destProjectID,
		State:     *finalState,
	}}) {
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
	if !msg.FromGlobal {
		m.popView() // Close project selector (not needed for global flow)
	}
	m.refreshAllViews()
	// The cross-project move folded a checked config artifact (durable planning-ref
	// reassignment) into its transaction; advance the shared domain snapshot now
	// that it is committed so views reading it stay consistent.
	m.refreshRuntime()
	if msg.SourceProject != nil && m.runtime.IsSystemInboxProject(msg.SourceProject.ID) {
		m.advanceOnboarding(func(progress *config.OnboardingProgress) { progress.InboxTriaged = true })
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
