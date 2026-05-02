package tui

import "charm.land/bubbletea/v2"

// finalizeGlobalOp converts a TaskOpResult into the (bool, tea.Cmd) pair
// used by handleGlobal*Applied methods: show success status, forward any
// workflow command, and append the default status timeout.
func (m *AppModel) finalizeGlobalOp(result TaskOpResult) (bool, tea.Cmd) {
	if result.Err != nil {
		return true, func() tea.Msg { return ErrorMsg{Err: result.Err} }
	}
	if result.StatusMessage != "" {
		m.setSuccessStatus(result.StatusMessage)
	}
	var cmds []tea.Cmd
	if result.Cmd != nil {
		cmds = append(cmds, result.Cmd)
	}
	cmds = append(cmds, defaultStatusTimeout())
	return true, tea.Batch(cmds...)
}

// handleGlobalStatusApplied applies a status change from the global status update flow.
func (m *AppModel) handleGlobalStatusApplied(msg GlobalStatusAppliedMsg) (bool, tea.Cmd) {
	m.statusQuickPicker = nil
	result := applyStatusChange(simplePersistWithUndo(m.persister, m.config, StatusVerb(msg.NewStatus)+" Task"), msg.Project, msg.Task.ID, msg.NewStatus)
	return m.finalizeGlobalOp(result)
}

// handleGlobalPriorityApplied applies a priority change from the global priority update flow.
func (m *AppModel) handleGlobalPriorityApplied(msg GlobalPriorityAppliedMsg) (bool, tea.Cmd) {
	m.priorityQuickPicker = nil
	result := applyPriorityChange(simplePersistWithUndo(m.persister, m.config, "Set Priority"), msg.Project, msg.Task.ID, msg.NewPriority)
	return m.finalizeGlobalOp(result)
}

// handleGlobalDueDateApplied applies a due date change from the global ;td due date flow.
func (m *AppModel) handleGlobalDueDateApplied(msg GlobalDueDateAppliedMsg) (bool, tea.Cmd) {
	m.dueDateQuickPicker = nil
	var result TaskOpResult
	if msg.Clear {
		result = applyClearDueDate(simplePersistWithUndo(m.persister, m.config, "Clear Due Date"), msg.Project, msg.Task.ID)
	} else {
		result = applyQuickDueDate(simplePersistWithUndo(m.persister, m.config, "Set Due Date"), msg.Project, msg.Task.ID, msg.DaysFromNow)
	}
	return m.finalizeGlobalOp(result)
}

// handleGlobalSectionChanged applies a section change from the global ;ts section change flow.
func (m *AppModel) handleGlobalSectionChanged(msg GlobalSectionChangedMsg) (bool, tea.Cmd) {
	m.sectionChangeQuickPicker = nil
	result := applySectionChange(simplePersistWithUndo(m.persister, m.config, "Move to Section"), msg.Project, msg.Task.ID, msg.Section)
	return m.finalizeGlobalOp(result)
}

// handleGlobalMoveSectionSelected completes the ;tm global task-move workflow.
// It sets the chosen section on the task, then emits MoveTaskToProjectMsg.
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

	task.SetSection(msg.Section)
	destProjectID := destProject.ID
	return true, func() tea.Msg {
		return MoveTaskToProjectMsg{
			Task:          task,
			SourceProject: sourceProject,
			DestProjectID: destProjectID,
			FromGlobal:    true,
		}
	}
}
