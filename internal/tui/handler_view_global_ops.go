package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
)

// finalizeGlobalOp converts an application-layer mutation outcome into the
// (bool, tea.Cmd) pair used by handleGlobal*Applied methods: on error it
// surfaces an error toast; on success it reloads the committed project, shows a
// success status, and appends the default status timeout. The single-task
// coordinator seam these handlers now use commits one ledger-backed transaction,
// so the event-rule cascade and completion gamification the legacy simplePersist
// path fired are intentionally dropped (matching the primary-view migration).
func (m *AppModel) finalizeGlobalOp(res app.TaskResult, err error, successMsg string) (bool, tea.Cmd) {
	if err != nil {
		return true, func() tea.Msg { return ErrorMsg{Err: err} }
	}
	m.reloadProjectByID(res.ProjectID)
	if successMsg != "" {
		m.setSuccessStatus(successMsg)
	}
	return true, defaultStatusTimeout()
}

// handleGlobalStatusApplied applies a workflow-state change from the global
// state update flow.
func (m *AppModel) handleGlobalStatusApplied(msg GlobalStatusAppliedMsg) (bool, tea.Cmd) {
	m.statusQuickPicker = nil
	res, err := m.appService.Transition(app.StateRequest{Locator: taskIdentityFor(msg.Task, msg.Project).Locator(), Target: msg.NewState})
	return m.finalizeGlobalOp(res, err, fmt.Sprintf("Task #%d -> %s", res.ShortID, msg.NewState))
}

// handleGlobalPriorityApplied applies a priority change from the global priority update flow.
func (m *AppModel) handleGlobalPriorityApplied(msg GlobalPriorityAppliedMsg) (bool, tea.Cmd) {
	m.priorityQuickPicker = nil
	res, err := m.appService.SetPriorityByLocator(taskIdentityFor(msg.Task, msg.Project).Locator(), msg.NewPriority)
	return m.finalizeGlobalOp(res, err, fmt.Sprintf("Priority set to %d", msg.NewPriority))
}

// handleGlobalDueDateApplied applies a due date change from the global picker flow.
func (m *AppModel) handleGlobalDueDateApplied(msg GlobalDueDateAppliedMsg) (bool, tea.Cmd) {
	m.dueDateQuickPicker = nil
	if msg.Clear {
		res, err := m.appService.SetDueDateByLocator(taskIdentityFor(msg.Task, msg.Project).Locator(), nil, false)
		return m.finalizeGlobalOp(res, err, "Due date cleared")
	}
	due, label := dueDateFromDays(msg.DaysFromNow, false)
	res, err := m.appService.SetDueDateByLocator(taskIdentityFor(msg.Task, msg.Project).Locator(), due, false)
	return m.finalizeGlobalOp(res, err, fmt.Sprintf("Due date set to %s", label))
}

// handleGlobalSectionChanged applies a section change from the global picker flow.
func (m *AppModel) handleGlobalSectionChanged(msg GlobalSectionChangedMsg) (bool, tea.Cmd) {
	m.sectionChangeQuickPicker = nil
	res, err := m.appService.SetSectionByLocator(taskIdentityFor(msg.Task, msg.Project).Locator(), msg.Section)
	return m.finalizeGlobalOp(res, err, "Moved to section")
}

// handleGlobalMoveSectionSelected completes the global task-move workflow.
// It forwards the chosen destination section with the move request so the
// move handler can apply it to the canonical source task.
func (m *AppModel) handleGlobalMoveSectionSelected(msg GlobalMoveSectionSelectedMsg) (bool, tea.Cmd) {
	m.sectionQuickPicker = nil
	task := m.globalMoveState.sourceTask
	sourceProject := m.globalMoveState.sourceProject
	destProject := m.globalMoveState.destProject
	m.globalMoveState.sourceProject = nil
	m.globalMoveState.sourceTask = nil
	m.globalMoveState.destProject = nil

	if task == nil || sourceProject == nil || destProject == nil {
		return true, nil
	}

	destProjectID := destProject.ID
	destSection := msg.Section
	return true, func() tea.Msg {
		return MoveTaskToProjectMsg{
			Task:          task,
			SourceProject: sourceProject,
			DestProjectID: destProjectID,
			DestSection:   &destSection,
			FromGlobal:    true,
		}
	}
}
