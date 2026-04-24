package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleViewActionMessages handles task action views such as priority, capture, blockers, and attachments.
func (m *AppModel) handleViewActionMessages(msg tea.Msg) (bool, tea.Cmd) {
	if handled, cmd := m.handleGlobalPickerMessages(msg); handled {
		return handled, cmd
	}
	return m.handleCaptureAndBulkMessages(msg)
}

// handleGlobalPickerMessages handles global picker, status, priority, due date, section, and linked task messages.
func (m *AppModel) handleGlobalPickerMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowPriorityViewMsg:
		if msg.Project != nil {
			pv := NewPriorityViewModel(msg.Project, m.config, m.configManager)
			pv.queueIDs = m.focusQueueIDs
			pv.dailyPlanIDs = m.dailyPlanIDs
			m.showView(ViewPriorityView, pv)
		}
		return true, nil

	case ClosePriorityViewMsg:
		m.popView()
		return true, nil

	case PriorityViewSelectMsg:
		m.popView()
		if msg.Task != nil && m.currentProject != nil {
			return true, m.navigateToTaskInProject(msg.Task, m.currentProject)
		}
		return true, nil

	case ShowGlobalProjectPickerMsg:
		activeProjects := m.config.GetActiveProjects()
		m.globalProjectPicker = newGlobalProjectPicker(activeProjects, m.config.Projects, msg.Purpose, msg.ExcludeIDs, msg.BackMsg)
		return true, m.globalProjectPicker.initCmd()

	case GlobalProjectPickerResultMsg:
		return m.routeGlobalProjectPickerResult(msg)

	case ShowGlobalTaskPickerMsg:
		var backMsg tea.Msg = ShowGlobalProjectPickerMsg{Purpose: PickerPurposeStatusUpdate}
		if msg.BackMsg != nil {
			backMsg = msg.BackMsg
		}
		// Load project if not yet populated (may happen if project was never visited this session)
		if len(msg.Project.Tasks) == 0 {
			_ = m.persister.Load(msg.Project)
		}
		m.globalTaskPicker = newGlobalTaskPicker(msg.Project, msg.Purpose, backMsg)
		return true, m.globalTaskPicker.initCmd()

	case GlobalTaskPickerResultMsg:
		return m.routeGlobalTaskPickerResult(msg)

	case GlobalStatusAppliedMsg:
		return m.handleGlobalStatusApplied(msg)

	case GlobalPriorityAppliedMsg:
		return m.handleGlobalPriorityApplied(msg)

	case GlobalDueDateAppliedMsg:
		return m.handleGlobalDueDateApplied(msg)

	case GlobalSectionChangedMsg:
		return m.handleGlobalSectionChanged(msg)

	case GlobalMoveSectionSelectedMsg:
		return m.handleGlobalMoveSectionSelected(msg)

	case ShowGlobalLinkedTaskTypePickerMsg:
		backMsg := ShowGlobalTaskPickerMsg{
			Project: msg.Project,
			Purpose: TaskPickerPurposeLinkedTask,
			BackMsg: ShowGlobalProjectPickerMsg{Purpose: PickerPurposeLinkedTask},
		}
		m.linkTypeQuickPicker = newLinkTypeQuickPicker(msg.Task, msg.Project, backMsg)
		return true, nil

	case GlobalLinkedTaskTypeSelectedMsg:
		m.globalLinkedTaskState.linkType = msg.LinkType
		backMsg := ShowGlobalLinkedTaskTypePickerMsg{
			Task:    msg.Task,
			Project: msg.Project,
		}
		m.linkModeQuickPicker = newLinkModeQuickPicker(msg.Task, msg.Project, msg.LinkType, backMsg)
		return true, nil

	case GlobalLinkedTaskModeCreateMsg:
		m.globalLinkedTaskState.sourceTask = nil
		m.globalLinkedTaskState.sourceProject = nil
		task := msg.Task
		project := msg.Project
		linkType := msg.LinkType
		return true, func() tea.Msg {
			return CreateLinkedTaskMsg{
				SourceTask: task,
				Project:    project,
				LinkType:   linkType,
			}
		}

	case GlobalLinkedTaskModeLinkMsg:
		m.globalLinkedTaskState.sourceTask = nil
		m.globalLinkedTaskState.sourceProject = nil
		task := msg.Task
		project := msg.Project
		return true, func() tea.Msg {
			return ShowCrossProjectLinkerMsg{
				SourceTask:    task,
				SourceProject: project,
			}
		}
	}

	return false, nil
}

// handleCaptureAndBulkMessages handles quick capture, bulk capture, bulk edit, and palette bulk messages.
func (m *AppModel) handleCaptureAndBulkMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowQuickCaptureMsg:
		if msg.Project != nil {
			m.showView(ViewQuickCapture, NewQuickCaptureModel(msg.Project, msg.AllProjects, msg.FromGlobalCapture))
		}
		return true, nil

	case CloseQuickCaptureMsg:
		m.popView()
		return true, nil

	case ShowBulkCaptureMsg:
		if msg.Project != nil {
			m.showView(ViewBulkCapture, NewBulkCaptureModel(msg.Project))
		}
		return true, nil

	case CloseBulkCaptureMsg:
		m.popView()
		return true, nil

	case ShowBulkEditMsg:
		if len(msg.Tasks) > 0 && msg.Project != nil {
			m.showView(ViewBulkEdit, NewBulkEditModel(msg.Tasks, msg.Project))
		}
		return true, nil

	case BulkEditDoneMsg:
		m.popView()
		if msg.Applied && m.projectTree != nil {
			return true, m.projectTree.applyBulkChanges(msg.Changes, msg.Tasks)
		}
		return true, nil

	case PaletteBulkPriorityMsg:
		if len(msg.Tasks) > 0 {
			m.pendingBulkTasks = msg.Tasks
			m.showView(ViewPriorityPicker, NewPriorityPickerModel(msg.Tasks[0].Task, msg.Tasks[0].Project))
		}
		return true, nil

	case PaletteBulkTagMsg:
		if len(msg.Tasks) > 0 {
			firstProject := msg.Tasks[0].Project
			var tasks []*models.Task
			for _, item := range msg.Tasks {
				if item.Project.ID == firstProject.ID {
					tasks = append(tasks, item.Task)
				}
			}
			if len(tasks) > 0 {
				m.showView(ViewBulkTagSelector, NewBulkTagSelectorModel(tasks, firstProject))
			}
		}
		return true, nil

	case PaletteBulkDueDateMsg:
		if len(msg.Tasks) > 0 {
			m.pendingBulkTasks = msg.Tasks
			m.showView(ViewDateInput, NewDateInputModel(msg.Tasks[0].Task, msg.Tasks[0].Project))
		}
		return true, nil
	}

	return false, nil
}

// routeGlobalProjectPickerResult dispatches the result of the global project picker
// to the appropriate action based on the picker's purpose.
func (m *AppModel) routeGlobalProjectPickerResult(msg GlobalProjectPickerResultMsg) (bool, tea.Cmd) {
	if msg.Project == nil {
		return true, nil
	}
	switch msg.Purpose {
	case PickerPurposeQuickCapture:
		return true, func() tea.Msg {
			return ShowQuickCaptureMsg{
				Project:           msg.Project,
				AllProjects:       msg.AllProjects,
				FromGlobalCapture: true,
			}
		}
	case PickerPurposeBulkCapture:
		return true, func() tea.Msg {
			return ShowBulkCaptureMsg{Project: msg.Project}
		}
	case PickerPurposeNoteCreate:
		return true, func() tea.Msg {
			return ShowNoteCreatorMsg{Project: msg.Project}
		}
	case PickerPurposeStatusUpdate:
		project := msg.Project
		return true, func() tea.Msg {
			return ShowGlobalTaskPickerMsg{
				Project: project,
				Purpose: TaskPickerPurposeStatusUpdate,
				BackMsg: ShowGlobalProjectPickerMsg{Purpose: PickerPurposeStatusUpdate},
			}
		}
	case PickerPurposeTaskMoveSource:
		m.globalMoveState.sourceProject = msg.Project
		project := msg.Project
		return true, func() tea.Msg {
			return ShowGlobalTaskPickerMsg{
				Project: project,
				Purpose: TaskPickerPurposeTaskMove,
				BackMsg: ShowGlobalProjectPickerMsg{Purpose: PickerPurposeTaskMoveSource},
			}
		}
	case PickerPurposeLinkedTask:
		project := msg.Project
		return true, func() tea.Msg {
			return ShowGlobalTaskPickerMsg{
				Project: project,
				Purpose: TaskPickerPurposeLinkedTask,
				BackMsg: ShowGlobalProjectPickerMsg{Purpose: PickerPurposeLinkedTask},
			}
		}
	case PickerPurposeTemplatePick:
		project := msg.Project
		if err := m.persister.Load(project); err != nil {
			return true, errorCmd("failed to load project: %w", err)
		}
		m.currentProject = project
		return true, func() tea.Msg {
			return ShowTemplatePickerMsg{Project: project}
		}
	case PickerPurposePriorityUpdate:
		project := msg.Project
		return true, func() tea.Msg {
			return ShowGlobalTaskPickerMsg{
				Project: project,
				Purpose: TaskPickerPurposePriorityUpdate,
				BackMsg: ShowGlobalProjectPickerMsg{Purpose: PickerPurposePriorityUpdate},
			}
		}
	case PickerPurposeDueDate:
		project := msg.Project
		return true, func() tea.Msg {
			return ShowGlobalTaskPickerMsg{
				Project: project,
				Purpose: TaskPickerPurposeDueDate,
				BackMsg: ShowGlobalProjectPickerMsg{Purpose: PickerPurposeDueDate},
			}
		}
	case PickerPurposeSectionChange:
		project := msg.Project
		return true, func() tea.Msg {
			return ShowGlobalTaskPickerMsg{
				Project: project,
				Purpose: TaskPickerPurposeSectionChange,
				BackMsg: ShowGlobalProjectPickerMsg{Purpose: PickerPurposeSectionChange},
			}
		}
	case PickerPurposeTaskMoveDest:
		destProject := msg.Project
		if err := m.persister.Load(destProject); err != nil {
			return true, errorCmd("failed to load destination project: %w", err)
		}
		m.globalMoveState.destProject = destProject
		sourceProject := m.globalMoveState.sourceProject
		sourceID := ""
		if sourceProject != nil {
			sourceID = sourceProject.ID
		}
		// Back from section picker → re-open dest project picker, with task picker as its back step.
		sectionPickerBackMsg := ShowGlobalProjectPickerMsg{
			Purpose:    PickerPurposeTaskMoveDest,
			ExcludeIDs: []string{sourceID},
			BackMsg: ShowGlobalTaskPickerMsg{
				Project: sourceProject,
				Purpose: TaskPickerPurposeTaskMove,
				BackMsg: ShowGlobalProjectPickerMsg{Purpose: PickerPurposeTaskMoveSource},
			},
		}
		m.sectionQuickPicker = newSectionQuickPicker(destProject, m.globalMoveState.sourceTask, sectionPickerBackMsg)
		return true, nil
	}
	return true, nil
}

// routeGlobalTaskPickerResult dispatches the result of the global task picker
// to the appropriate action based on the picker's purpose.
func (m *AppModel) routeGlobalTaskPickerResult(msg GlobalTaskPickerResultMsg) (bool, tea.Cmd) {
	if msg.Task == nil || msg.Project == nil {
		return true, nil
	}
	switch msg.Purpose {
	case TaskPickerPurposeStatusUpdate:
		// backMsg re-opens task picker for the same project
		backMsg := ShowGlobalTaskPickerMsg{
			Project: msg.Project,
			Purpose: TaskPickerPurposeStatusUpdate,
			BackMsg: ShowGlobalProjectPickerMsg{Purpose: PickerPurposeStatusUpdate},
		}
		m.statusQuickPicker = newStatusQuickPicker(msg.Task, msg.Project, backMsg)
		return true, nil
	case TaskPickerPurposeTaskMove:
		m.globalMoveState.sourceTask = msg.Task
		sourceProject := m.globalMoveState.sourceProject
		sourceID := ""
		if sourceProject != nil {
			sourceID = sourceProject.ID
		}
		taskPickerBackMsg := ShowGlobalProjectPickerMsg{Purpose: PickerPurposeTaskMoveSource}
		return true, func() tea.Msg {
			return ShowGlobalProjectPickerMsg{
				Purpose:    PickerPurposeTaskMoveDest,
				ExcludeIDs: []string{sourceID},
				BackMsg: ShowGlobalTaskPickerMsg{
					Project: sourceProject,
					Purpose: TaskPickerPurposeTaskMove,
					BackMsg: taskPickerBackMsg,
				},
			}
		}
	case TaskPickerPurposeLinkedTask:
		m.globalLinkedTaskState.sourceTask = msg.Task
		m.globalLinkedTaskState.sourceProject = msg.Project
		backMsg := ShowGlobalTaskPickerMsg{
			Project: msg.Project,
			Purpose: TaskPickerPurposeLinkedTask,
			BackMsg: ShowGlobalProjectPickerMsg{Purpose: PickerPurposeLinkedTask},
		}
		m.linkTypeQuickPicker = newLinkTypeQuickPicker(msg.Task, msg.Project, backMsg)
		return true, nil
	case TaskPickerPurposePriorityUpdate:
		backMsg := ShowGlobalTaskPickerMsg{
			Project: msg.Project,
			Purpose: TaskPickerPurposePriorityUpdate,
			BackMsg: ShowGlobalProjectPickerMsg{Purpose: PickerPurposePriorityUpdate},
		}
		m.priorityQuickPicker = newPriorityQuickPicker(msg.Task, msg.Project, backMsg)
		return true, nil
	case TaskPickerPurposeDueDate:
		backMsg := ShowGlobalTaskPickerMsg{
			Project: msg.Project,
			Purpose: TaskPickerPurposeDueDate,
			BackMsg: ShowGlobalProjectPickerMsg{Purpose: PickerPurposeDueDate},
		}
		m.dueDateQuickPicker = newDueDateQuickPicker(msg.Task, msg.Project, backMsg)
		return true, nil
	case TaskPickerPurposeSectionChange:
		backMsg := ShowGlobalTaskPickerMsg{
			Project: msg.Project,
			Purpose: TaskPickerPurposeSectionChange,
			BackMsg: ShowGlobalProjectPickerMsg{Purpose: PickerPurposeSectionChange},
		}
		if len(msg.Project.Tasks) == 0 {
			_ = m.persister.Load(msg.Project)
		}
		m.sectionChangeQuickPicker = newSectionChangeQuickPicker(msg.Project, msg.Task, backMsg)
		return true, nil
	}
	return true, nil
}

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

// handleViewSupportMessages handles blockers and attachment views.
func (m *AppModel) handleViewSupportMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowBlockersMsg:
		if msg.Project != nil {
			editor := m.config.Editor
			m.showView(ViewBlockers, NewBlockersModelWithEditor(msg.Project, editor))
		}
		return true, nil

	case CloseBlockersMsg:
		m.popView()
		return true, nil

	case ShowGlobalBlockersMsg:
		m.showView(ViewGlobalBlockers, NewGlobalBlockersModel(m.config))
		return true, nil

	case CloseGlobalBlockersMsg:
		m.popToAnchorView()
		return true, nil

	case BlockerGoToProjectMsg:
		m.popView()
		if msg.Project != nil {
			m.switchToProject(msg.Project, true)
			m.pushView(ViewProjectTree)
		}
		return true, nil

	case BlockerAddedMsg, BlockerEditedMsg, BlockerDeletedMsg:
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case ShowTaskAttachmentsMsg:
		if msg.Task != nil && msg.Project != nil {
			m.lastNavigatedTaskID = msg.Task.ID
			m.recordTaskVisit(msg.Task, msg.Project)
			m.showView(ViewTaskAttachments, NewTaskAttachmentsModel(msg.Task, msg.Project, m.config))
		}
		return true, nil

	case CloseTaskAttachmentsMsg:
		m.popView()
		return true, nil

	case AttachmentAddedMsg, AttachmentDeletedMsg:
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case AttachmentReorderedMsg:
		return true, nil
	}

	return false, nil
}
