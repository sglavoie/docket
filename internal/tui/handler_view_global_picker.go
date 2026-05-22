package tui

import tea "charm.land/bubbletea/v2"

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
