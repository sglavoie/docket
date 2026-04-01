package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/storage"
)

// navigateHistoryBack moves backward through navigation history, skipping stale entries.
// When the current view is untracked (editors, pickers, modals), the history cursor
// already points at the parent tracked view. In that case we navigate directly to the
// cursor entry instead of decrementing past it.
func (m *AppModel) navigateHistoryBack() tea.Cmd {
	// If we're on an untracked view, the cursor still points at the last tracked
	// view (our parent). Navigate there without decrementing.
	if cursorEntry, ok := m.navHistory.Current(); ok {
		if m.currentView() != cursorEntry.ViewType && m.isNavEntryValid(cursorEntry) {
			return m.navigateToEntry(cursorEntry)
		}
	}

	for m.navHistory.CanGoBack() {
		entry, ok := m.navHistory.GoBack()
		if !ok {
			break
		}
		if m.isNavEntryValid(entry) {
			return m.navigateToEntry(entry)
		}
	}
	return nil
}

// navigateHistoryForward moves forward through navigation history, skipping stale entries.
func (m *AppModel) navigateHistoryForward() tea.Cmd {
	for m.navHistory.CanGoForward() {
		entry, ok := m.navHistory.GoForward()
		if !ok {
			break
		}
		if m.isNavEntryValid(entry) {
			return m.navigateToEntry(entry)
		}
	}
	return nil
}

// isNavEntryValid checks whether a history entry still refers to existing entities.
func (m *AppModel) isNavEntryValid(entry NavEntry) bool {
	if entry.ProjectID != "" {
		if m.config.GetProject(entry.ProjectID) == nil {
			return false
		}
	}
	if entry.TaskID != "" && entry.ProjectID != "" {
		project := m.config.GetProject(entry.ProjectID)
		if project == nil {
			return false
		}
		if err := m.persister.Load(project); err != nil {
			logging.Warn("failed to load project %s: %v", project.Name, err)
		}
		if project.GetTaskByID(entry.TaskID) == nil {
			return false
		}
	}
	return true
}

// navigateToEntry reconstructs the view described by entry and navigates to it.
func (m *AppModel) navigateToEntry(entry NavEntry) tea.Cmd {
	m.navHistory.Lock()
	defer m.navHistory.Unlock()

	m.saveProjectTreeState()
	m.saveCurrentViewFilterState()

	// Reset view stack and project stack
	m.projectStack = nil
	m.viewStack = []ViewType{ViewDashboard}

	switch {
	case entry.ViewType == ViewDashboard:
		return m.reconstructDashboard()
	case isGlobalNavView(entry.ViewType):
		return m.reconstructGlobalView(entry)
	case entry.TaskID != "" && entry.ProjectID != "":
		return m.reconstructTaskView(entry)
	case entry.ProjectID != "":
		return m.reconstructProjectView(entry)
	default:
		return m.reconstructGlobalView(entry)
	}
}

func (m *AppModel) reconstructDashboard() tea.Cmd {
	if m.dashboard != nil {
		m.dashboard.Refresh(m.config)
	}
	return nil
}

func (m *AppModel) reconstructGlobalView(entry NavEntry) tea.Cmd {
	switch entry.ViewType {
	case ViewDailyReview:
		m.showView(ViewDailyReview, NewDailyReviewModel(m.config, m.configManager))
	case ViewWeeklyReview:
		m.showView(ViewWeeklyReview, NewWeeklyReviewModel(m.config, m.configManager))
	case ViewFocusView:
		m.showView(ViewFocusView, NewFocusViewModel(m.config, m.configManager, &m.focusQueue, m.focusQueueIDs, m.focusDismissedIDs))
	case ViewDueDateOverview:
		m.showView(ViewDueDateOverview, NewDueDateOverviewModel(m.config, m.configManager))
	case ViewRecentActivity:
		m.showView(ViewRecentActivity, NewRecentActivityModel(m.config))
	case ViewGlobalSearch:
		m.globalSearch = NewGlobalSearchModel(m.config)
		m.showView(ViewGlobalSearch, m.globalSearch)
		return m.globalSearch.Init()
	case ViewGlobalBlockers:
		m.showView(ViewGlobalBlockers, NewGlobalBlockersModel(m.config))
	case ViewGlobalNotes:
		dir := storage.GlobalNotesDir(m.config.DataDir)
		m.showView(ViewGlobalNotes, NewGlobalNotesViewModel(dir, m.config))
	case ViewGlobalInboxProcessor:
		processor := NewGlobalInboxProcessorModel(m.config, m.configManager)
		m.globalInboxProcessor = processor
		m.showView(ViewGlobalInboxProcessor, processor)
	case ViewGlobalTagManager:
		m.showView(ViewGlobalTagManager, NewGlobalTagManagerModel(m.config))
	case ViewSettings:
		m.showView(ViewSettings, NewSettingsModel(m.config, m.configManager))
	case ViewWorkflowRules:
		m.showView(ViewWorkflowRules, NewWorkflowRulesModel(m.config, m.configManager))
	case ViewTaskTemplates:
		m.showView(ViewTaskTemplates, NewTaskTemplatesModel(m.config, m.configManager))
	case ViewTutor:
		m.showView(ViewTutor, NewTutorModel())
	case ViewScoreProfileManager:
		projectID := ""
		if m.currentProject != nil {
			projectID = m.currentProject.ID
		}
		m.showView(ViewScoreProfileManager, NewScoreProfileManagerModel(m.config, m.configManager, projectID))
	case ViewMetricsDashboard:
		if m.metricsDashboard != nil {
			m.metricsDashboard.computeMetrics()
			m.showView(ViewMetricsDashboard, m.metricsDashboard)
		} else {
			m.metricsDashboard = NewMetricsDashboardModel(m.config)
			m.showView(ViewMetricsDashboard, m.metricsDashboard)
		}
	case ViewCrossProjectOutliner:
		m.crossOutliner = NewOutlinerModel(nil, m.config, m.configManager)
		m.showView(ViewCrossProjectOutliner, m.crossOutliner)
		return m.crossOutliner.Init()
	case ViewFocusBoard:
		m.showView(ViewFocusBoard, NewFocusBoardModel(m.config, m.configManager))
	case ViewAllTasks:
		atv := NewAllTasksViewModel(m.config, m.configManager)
		if m.allTasksFilterCache != nil {
			atv.filter = *copyCommonFilterState(&m.allTasksFilterCache.Filter)
			atv.groupBy = m.allTasksFilterCache.GroupBy
			atv.tabs.Active = m.allTasksFilterCache.TabActive
			atv.loadAllTasks()
		}
		m.allTasksView = atv
		m.showView(ViewAllTasks, atv)
	default:
	}
	return nil
}

func (m *AppModel) reconstructProjectView(entry NavEntry) tea.Cmd {
	project := m.config.GetProject(entry.ProjectID)
	if project == nil {
		return nil
	}

	m.switchToProject(project, true)
	m.pushView(ViewProjectTree)

	if entry.ViewType == ViewProjectTree {
		return nil
	}

	// Push the target view on top of the project tree
	switch entry.ViewType {
	case ViewPriorityView:
		pv := NewPriorityViewModel(project, m.config, m.configManager)
		if cached, ok := m.priorityViewFilterCache[project.ID]; ok {
			pv.filter = *copyCommonFilterState(&cached.Filter)
			pv.tabs.Active = cached.TabActive
			pv.loadActionableTasks()
		}
		m.showView(ViewPriorityView, pv)
	case ViewNotes:
		m.showView(ViewNotes, NewNotesViewModel(project, m.config))
	case ViewBlockers:
		m.showView(ViewBlockers, NewBlockersModelWithEditor(project, m.config.Editor))
	case ViewInboxProcessor:
		processor := NewInboxProcessorModel(project, m.config, m.configManager)
		m.inboxProcessor = processor
		m.showView(ViewInboxProcessor, processor)
	case ViewOutliner:
		m.saveOutlinerToggleState()
		m.saveOutlinerFilterState()
		m.outliner = NewOutlinerModel(project, m.config, m.configManager)
		m.restoreOutlinerToggleState()
		m.restoreOutlinerFilterState()
		m.showView(ViewOutliner, m.outliner)
	case ViewUndoHistory:
		m.showView(ViewUndoHistory, NewUndoHistoryModel(project))
	case ViewProjectFocusView:
		m.showView(ViewProjectFocusView, NewProjectFocusViewModel(project))
	case ViewMetricsDashboard:
		if m.metricsDashboard != nil {
			m.metricsDashboard.computeMetrics()
			m.showView(ViewMetricsDashboard, m.metricsDashboard)
		} else {
			m.metricsDashboard = NewMetricsDashboardModelForProject(m.config, project)
			m.showView(ViewMetricsDashboard, m.metricsDashboard)
		}
	case ViewTemporaryOutliner:
		fresh := NewOutlinerModel(project, m.config, m.configManager)
		m.showView(ViewTemporaryOutliner, fresh)
	case ViewProjectRecentActivity:
		m.showView(ViewProjectRecentActivity, NewProjectRecentActivityModel(m.config, project))
	default:
	}
	return nil
}

func (m *AppModel) reconstructTaskView(entry NavEntry) tea.Cmd {
	project := m.config.GetProject(entry.ProjectID)
	if project == nil {
		return nil
	}

	m.switchToProject(project, true)
	m.pushView(ViewProjectTree)

	task := project.GetTaskByID(entry.TaskID)
	if task == nil {
		return nil
	}

	switch entry.ViewType {
	case ViewTaskComments:
		m.lastNavigatedTaskID = task.ID
		m.showView(ViewTaskComments, NewTaskCommentsModelWithEditor(task, project, m.config.Editor))
	case ViewTaskChecklist:
		m.lastNavigatedTaskID = task.ID
		m.showView(ViewTaskChecklist, NewTaskChecklistModelWithEditor(task, project, m.config.Editor))
	case ViewTaskAttachments:
		m.lastNavigatedTaskID = task.ID
		m.showView(ViewTaskAttachments, NewTaskAttachmentsModel(task, project, m.config))
	case ViewTaskActivity:
		m.lastNavigatedTaskID = task.ID
		m.showView(ViewTaskActivity, NewTaskActivityModel(task, project))
	default:
	}
	return nil
}
