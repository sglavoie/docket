package tui

import tea "charm.land/bubbletea/v2"

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
