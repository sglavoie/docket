package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle global key messages first
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "esc":
			if handled := m.handleEscKey(); handled {
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.handleWindowSize(msg)
	}

	// Try each handler group in sequence
	if handled, cmd := m.handleProjectMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleTaskMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleViewMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleTagMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleSectionMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleClipboardMessages(msg); handled {
		return m, cmd
	}

	// Delegate to current view
	cmds = append(cmds, m.delegateToCurrentView(msg))

	return m, tea.Batch(cmds...)
}

// handleEscKey handles the escape key for view navigation.
// Returns true if the key was handled (should not propagate to views).
func (m *AppModel) handleEscKey() bool {
	// Let views handle esc if they have an active search
	if m.currentView() == ViewProjectTree && m.projectTree != nil && m.projectTree.HasActiveSearch() {
		return false
	}
	if m.currentView() == ViewDashboard && m.dashboard != nil && m.dashboard.HasActiveSearch() {
		return false
	}
	if m.currentView() == ViewTaskDetail && m.taskDetail != nil && m.taskDetail.HasActiveSearch() {
		return false
	}
	if m.currentView() == ViewHelp && m.helpView != nil && m.helpView.HasActiveSearch() {
		return false
	}
	if m.currentView() == ViewTaskJumper && m.taskJumper != nil {
		return false
	}
	if m.currentView() != ViewDashboard {
		m.popView()
		return true
	}
	return false
}

// handleWindowSize updates dimensions for all views.
func (m *AppModel) handleWindowSize(msg tea.WindowSizeMsg) {
	m.width = msg.Width
	m.height = msg.Height

	if m.dashboard != nil {
		m.dashboard.SetSize(msg.Width, msg.Height-2)
	}
	if m.projectList != nil {
		m.projectList.SetSize(msg.Width, msg.Height-2)
	}
	if m.projectTree != nil {
		m.projectTree.SetSize(msg.Width, msg.Height-2)
	}
	if m.taskDetail != nil {
		m.taskDetail.SetSize(msg.Width, msg.Height-2)
	}
	if m.settings != nil {
		m.settings.SetSize(msg.Width, msg.Height-2)
	}
}

// delegateToCurrentView forwards messages to the active view model.
func (m *AppModel) delegateToCurrentView(msg tea.Msg) tea.Cmd {
	switch m.currentView() {
	case ViewDashboard:
		if m.dashboard != nil {
			newModel, cmd := m.dashboard.Update(msg)
			m.dashboard = newModel.(*DashboardModel)
			return cmd
		}

	case ViewProjectList:
		if m.projectList != nil {
			newModel, cmd := m.projectList.Update(msg)
			m.projectList = newModel.(*ProjectListModel)
			return cmd
		}

	case ViewProjectTree:
		if m.projectTree != nil {
			newModel, cmd := m.projectTree.Update(msg)
			m.projectTree = newModel.(*ProjectTreeModel)
			return cmd
		}

	case ViewTaskDetail:
		if m.taskDetail != nil {
			newModel, cmd := m.taskDetail.Update(msg)
			m.taskDetail = newModel.(*TaskDetailModel)
			return cmd
		}

	case ViewProjectCreator:
		if m.projectCreator != nil {
			newModel, cmd := m.projectCreator.Update(msg)
			m.projectCreator = newModel.(*ProjectCreatorModel)
			return cmd
		}

	case ViewProjectEditor:
		if m.projectEditor != nil {
			newModel, cmd := m.projectEditor.Update(msg)
			m.projectEditor = newModel.(*ProjectEditorModel)
			return cmd
		}

	case ViewTaskCreator:
		if m.taskCreator != nil {
			newModel, cmd := m.taskCreator.Update(msg)
			m.taskCreator = newModel.(*TaskCreatorModel)
			return cmd
		}

	case ViewTaskEditor:
		if m.taskEditor != nil {
			newModel, cmd := m.taskEditor.Update(msg)
			m.taskEditor = newModel.(*TaskEditorModel)
			return cmd
		}

	case ViewSettings:
		if m.settings != nil {
			newModel, cmd := m.settings.Update(msg)
			m.settings = newModel.(*SettingsModel)
			return cmd
		}

	case ViewHelp:
		if m.helpView != nil {
			newModel, cmd := m.helpView.Update(msg)
			m.helpView = newModel.(*HelpViewModel)
			return cmd
		}

	case ViewDateInput:
		if m.dateInput != nil {
			newModel, cmd := m.dateInput.Update(msg)
			m.dateInput = newModel.(*DateInputModel)
			return cmd
		}

	case ViewDependencySelector:
		if m.dependencySelector != nil {
			newModel, cmd := m.dependencySelector.Update(msg)
			m.dependencySelector = newModel.(*DependencySelectorModel)
			return cmd
		}

	case ViewFilter:
		if m.filterView != nil {
			newModel, cmd := m.filterView.Update(msg)
			m.filterView = newModel.(*FilterViewModel)
			return cmd
		}

	case ViewTaskJumper:
		if m.taskJumper != nil {
			newModel, cmd := m.taskJumper.Update(msg)
			m.taskJumper = newModel.(*TaskJumperModel)
			return cmd
		}

	case ViewProjectPreview:
		if m.projectPreview != nil {
			newModel, cmd := m.projectPreview.Update(msg)
			m.projectPreview = newModel.(*ProjectPreviewModel)
			return cmd
		}

	case ViewDueDateOverview:
		if m.dueDateOverview != nil {
			newModel, cmd := m.dueDateOverview.Update(msg)
			m.dueDateOverview = newModel.(*DueDateOverviewModel)
			return cmd
		}

	case ViewTagManager:
		if m.tagManager != nil {
			newModel, cmd := m.tagManager.Update(msg)
			m.tagManager = newModel.(*TagManagerModel)
			return cmd
		}

	case ViewGlobalTagManager:
		if m.globalTagManager != nil {
			newModel, cmd := m.globalTagManager.Update(msg)
			m.globalTagManager = newModel.(*GlobalTagManagerModel)
			return cmd
		}

	case ViewTagFilterSelector:
		if m.tagFilterSelector != nil {
			newModel, cmd := m.tagFilterSelector.Update(msg)
			m.tagFilterSelector = newModel.(*TagFilterSelectorModel)
			return cmd
		}

	case ViewSectionFilterSelector:
		if m.sectionFilterSelector != nil {
			newModel, cmd := m.sectionFilterSelector.Update(msg)
			m.sectionFilterSelector = newModel.(*SectionFilterSelectorModel)
			return cmd
		}

	case ViewSectionPicker:
		if m.sectionPicker != nil {
			newModel, cmd := m.sectionPicker.Update(msg)
			m.sectionPicker = newModel.(*SectionPickerModel)
			return cmd
		}

	case ViewSectionManager:
		if m.sectionManager != nil {
			newModel, cmd := m.sectionManager.Update(msg)
			m.sectionManager = newModel.(*SectionManagerModel)
			return cmd
		}

	case ViewContextPicker:
		if m.contextPicker != nil {
			newModel, cmd := m.contextPicker.Update(msg)
			m.contextPicker = newModel.(*ContextPickerModel)
			return cmd
		}
	}

	return nil
}
