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
	m.saveOutlinerToggleState()
	m.saveOutlinerFilterState()
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
		dr := NewDailyReviewModel(m.config, m.configManager)
		dr.queueIDs = m.focusQueueIDs
		dr.dailyPlanIDs = m.dailyPlanIDs
		m.showView(ViewDailyReview, dr)
	case ViewWeeklyReview:
		wr := NewWeeklyReviewModel(m.config, m.configManager)
		wr.queueIDs = m.focusQueueIDs
		wr.dailyPlanIDs = m.dailyPlanIDs
		m.showView(ViewWeeklyReview, wr)
	case ViewFocusView:
		m.showView(ViewFocusView, NewFocusViewModel(m.config, m.configManager, &m.focusQueue, m.focusQueueIDs, m.dailyPlanIDs, m.focusDismissedIDs))
	case ViewDueDateOverview:
		ddo := NewDueDateOverviewModel(m.config, m.configManager)
		ddo.queueIDs = m.focusQueueIDs
		ddo.dailyPlanIDs = m.dailyPlanIDs
		m.showView(ViewDueDateOverview, ddo)
	case ViewRecentActivity:
		ra := NewRecentActivityModel(m.config, m.configManager)
		ra.queueIDs = m.focusQueueIDs
		ra.dailyPlanIDs = m.dailyPlanIDs
		m.showView(ViewRecentActivity, ra)
	case ViewGlobalSearch:
		m.globalSearch = NewGlobalSearchModel(m.config, m.configManager)
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
		// Recreate when cached model is project-scoped so the global view is
		// not contaminated by a previous project's metrics.
		if m.metricsDashboard == nil || !m.metricsDashboard.isGlobal {
			m.metricsDashboard = NewMetricsDashboardModel(m.config)
		} else {
			m.metricsDashboard.computeMetrics()
		}
		m.showView(ViewMetricsDashboard, m.metricsDashboard)
	case ViewRoadmap:
		if m.roadmap == nil || !m.roadmap.isGlobal {
			m.roadmap = NewRoadmapModel(m.config)
		} else {
			m.roadmap.computeData()
		}
		m.showView(ViewRoadmap, m.roadmap)
	case ViewCrossProjectOutliner:
		m.crossOutliner = NewOutlinerModel(nil, m.config, m.configManager)
		m.crossOutliner.queueIDs = m.focusQueueIDs
		m.crossOutliner.dailyPlanIDs = m.dailyPlanIDs
		m.showView(ViewCrossProjectOutliner, m.crossOutliner)
		return m.crossOutliner.Init()
	case ViewFocusBoard:
		fb := NewFocusBoardModel(m.config, m.configManager, &m.dailyPlan, m.focusQueueIDs, m.dailyPlanIDs, m.dailyPlanCarriedIDs)
		m.showView(ViewFocusBoard, fb)
	case ViewAllTasks:
		atv := NewAllTasksViewModel(m.config, m.configManager)
		atv.queueIDs = m.focusQueueIDs
		atv.dailyPlanIDs = m.dailyPlanIDs
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
		pv.queueIDs = m.focusQueueIDs
		pv.dailyPlanIDs = m.dailyPlanIDs
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
		m.outliner.queueIDs = m.focusQueueIDs
		m.outliner.dailyPlanIDs = m.dailyPlanIDs
		m.restoreOutlinerToggleState()
		m.restoreOutlinerFilterState()
		m.showView(ViewOutliner, m.outliner)
	case ViewUndoHistory:
		m.showView(ViewUndoHistory, NewUndoHistoryModel(project))
	case ViewProjectFocusView:
		m.showView(ViewProjectFocusView, NewProjectFocusViewModel(project, m.config))
	case ViewMetricsDashboard:
		// Recreate when cached model is for a different project (or global)
		// to avoid leaking the previous project's metrics into this view.
		if m.metricsDashboard == nil || m.metricsDashboard.isGlobal || m.metricsDashboard.project != project {
			m.metricsDashboard = NewMetricsDashboardModelForProject(m.config, project)
		} else {
			m.metricsDashboard.computeMetrics()
		}
		m.showView(ViewMetricsDashboard, m.metricsDashboard)
	case ViewRoadmap:
		if m.roadmap == nil || m.roadmap.isGlobal || m.roadmap.project != project {
			m.roadmap = NewRoadmapModelForProject(m.config, project)
		} else {
			m.roadmap.computeData()
		}
		m.showView(ViewRoadmap, m.roadmap)
	case ViewTemporaryOutliner:
		fresh := NewOutlinerModel(project, m.config, m.configManager)
		fresh.queueIDs = m.focusQueueIDs
		fresh.dailyPlanIDs = m.dailyPlanIDs
		m.showView(ViewTemporaryOutliner, fresh)
	case ViewProjectRecentActivity:
		pra := NewProjectRecentActivityModel(m.config, m.configManager, project)
		pra.queueIDs = m.focusQueueIDs
		pra.dailyPlanIDs = m.dailyPlanIDs
		m.showView(ViewProjectRecentActivity, pra)
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
	case ViewTaskDecisions:
		m.lastNavigatedTaskID = task.ID
		m.showView(ViewTaskDecisions, NewTaskDecisionsModelWithEditor(task, project, m.config.Editor))
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
