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
		if m.projectByID(entry.ProjectID) == nil {
			return false
		}
	}
	if entry.TaskID != "" && entry.ProjectID != "" {
		project := m.projectByID(entry.ProjectID)
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

	m.saveProjectViewState()
	m.saveOutlinerToggleState()
	m.saveOutlinerFilterState()
	m.saveCurrentViewFilterState()

	// Reset view stack and project stack
	m.exitAskView()
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
		m.dashboard.Refresh()
	}
	return nil
}

func (m *AppModel) reconstructGlobalView(entry NavEntry) tea.Cmd {
	switch entry.ViewType {
	case ViewDailyReview:
		dr := NewDailyReviewModel(m.runtime, m.configManager)
		dr.queueIDs = m.focusQueueIDs
		dr.dailyPlanIDs = m.dailyPlanIDs
		m.showView(ViewDailyReview, dr)
	case ViewWeeklyReview:
		wr := NewWeeklyReviewModel(m.runtime, m.configManager)
		wr.queueIDs = m.focusQueueIDs
		wr.dailyPlanIDs = m.dailyPlanIDs
		m.showView(ViewWeeklyReview, wr)
	case ViewRecentActivity:
		ra := NewRecentActivityModel(m.runtime, m.configManager)
		ra.queueIDs = m.focusQueueIDs
		ra.dailyPlanIDs = m.dailyPlanIDs
		m.showView(ViewRecentActivity, ra)
	case ViewGlobalSearch:
		m.globalSearch = NewGlobalSearchModel(m.runtime)
		m.showView(ViewGlobalSearch, m.globalSearch)
		return m.globalSearch.Init()
	case ViewGlobalBlockers:
		m.showView(ViewGlobalBlockers, NewGlobalBlockersModel(m.runtime, m.secondaryFeatures.Blocker))
	case ViewGlobalNotes:
		dir := storage.GlobalNotesDir(m.runtime.DataDir())
		m.showView(ViewGlobalNotes, NewGlobalNotesViewModelWithService(dir, m.appService, m.deps.historyLedger(), m.runtime.Machine()))
	case ViewGlobalInboxProcessor:
		processor := NewGlobalInboxProcessorModel(m.configManager, m.runtime)
		m.globalInboxProcessor = processor
		m.showView(ViewGlobalInboxProcessor, processor)
	case ViewGlobalTagManager:
		m.showView(ViewGlobalTagManager, NewGlobalTagManagerModel(m.runtime.ActiveProjects(), m.runtime, m.secondaryFeatures.Tag))
	case ViewSettings:
		m.showView(ViewSettings, NewSettingsModel(m.deps))
	case ViewWorkflowRules:
		m.showView(ViewWorkflowRules, NewWorkflowRulesModel(m.runtime))
	case ViewTaskTemplates:
		m.showView(ViewTaskTemplates, NewTaskTemplatesModel(m.runtime))
	case ViewTutor:
		m.showView(ViewTutor, NewTutorModel())
	case ViewScoreProfileManager:
		projectID := ""
		if m.currentProject != nil {
			projectID = m.currentProject.ID
		}
		m.showView(ViewScoreProfileManager, NewScoreProfileManagerModel(m.runtime, projectID))
	case ViewMetricsDashboard:
		// Recreate when cached model is project-scoped so the global view is
		// not contaminated by a previous project's metrics.
		if m.metricsDashboard == nil || !m.metricsDashboard.isGlobal {
			m.metricsDashboard = NewMetricsDashboardModel(m.configManager)
		} else {
			m.metricsDashboard.computeMetrics()
		}
		m.showView(ViewMetricsDashboard, m.metricsDashboard)
	case ViewRoadmap:
		if m.roadmap == nil || !m.roadmap.isGlobal {
			m.roadmap = NewRoadmapModel(m.runtime)
		} else {
			m.roadmap.computeData()
		}
		m.showView(ViewRoadmap, m.roadmap)
	case ViewCrossProjectOutliner:
		m.crossOutliner = NewOutlinerModel(nil, m.configManager, m.runtime)
		m.crossOutliner.queueIDs = m.focusQueueIDs
		m.crossOutliner.dailyPlanIDs = m.dailyPlanIDs
		m.showView(ViewCrossProjectOutliner, m.crossOutliner)
		return m.crossOutliner.Init()
	case ViewFocusBoard:
		fb := NewFocusBoardModel(m.runtime, m.configManager, &m.dailyPlan, &m.focusQueue, m.focusQueueIDs, m.dailyPlanIDs, m.dailyPlanCarriedIDs)
		m.showView(ViewFocusBoard, fb)
	case ViewAllTasks:
		atv := NewAllTasksViewModel(m.runtime, m.configManager)
		atv.bulk = m.secondaryFeatures.Bulk
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
	project := m.projectByID(entry.ProjectID)
	if project == nil {
		return nil
	}

	m.switchToProject(project, true)
	m.pushView(ViewOutliner)

	// switchToProject + pushView above already establish the Outliner as the
	// base project view (created, state-restored, registered, and pushed). When
	// the history target IS the Outliner, there is nothing further to push — an
	// extra showView here would duplicate the stack entry and make the first
	// Escape appear ineffective (FR-010-003). Only stack a distinct target view
	// on top of the base Outliner.
	if entry.ViewType == ViewOutliner {
		return nil
	}

	// Push the target view on top of the project view
	switch entry.ViewType {
	case ViewPriorityView:
		pv := NewPriorityViewModel(project, m.runtime, m.configManager)
		pv.queueIDs = m.focusQueueIDs
		pv.dailyPlanIDs = m.dailyPlanIDs
		if cached, ok := m.priorityViewFilterCache[project.ID]; ok {
			pv.filter = *copyCommonFilterState(&cached.Filter)
			pv.tabs.Active = cached.TabActive
			pv.loadActionableTasks()
		}
		m.showView(ViewPriorityView, pv)
	case ViewNotes:
		m.showView(ViewNotes, NewNotesViewModelWithService(project, m.appService, m.deps.historyLedger(), m.runtime.Machine()))
	case ViewBlockers:
		m.showView(ViewBlockers, NewBlockersModelWithEditor(project, m.runtime.Machine().GetEditor(), m.secondaryFeatures.Blocker))
	case ViewInboxProcessor:
		processor := NewInboxProcessorModel(project, m.configManager, m.runtime)
		m.inboxProcessor = processor
		m.showView(ViewInboxProcessor, processor)
	case ViewProjectFocusView:
		m.showView(ViewProjectFocusView, NewProjectFocusViewModel(project, m.runtime))
	case ViewMetricsDashboard:
		// Recreate when cached model is for a different project (or global)
		// to avoid leaking the previous project's metrics into this view.
		if m.metricsDashboard == nil || m.metricsDashboard.isGlobal || m.metricsDashboard.project != project {
			m.metricsDashboard = NewMetricsDashboardModelForProject(m.configManager, project)
		} else {
			m.metricsDashboard.computeMetrics()
		}
		m.showView(ViewMetricsDashboard, m.metricsDashboard)
	case ViewRoadmap:
		if m.roadmap == nil || m.roadmap.isGlobal || m.roadmap.project != project {
			m.roadmap = NewRoadmapModelForProject(m.runtime, project)
		} else {
			m.roadmap.computeData()
		}
		m.showView(ViewRoadmap, m.roadmap)
	case ViewTemporaryOutliner:
		fresh := NewOutlinerModel(project, m.configManager, m.runtime)
		fresh.queueIDs = m.focusQueueIDs
		fresh.dailyPlanIDs = m.dailyPlanIDs
		m.showView(ViewTemporaryOutliner, fresh)
	case ViewProjectRecentActivity:
		pra := NewProjectRecentActivityModel(m.runtime, m.configManager, project)
		pra.queueIDs = m.focusQueueIDs
		pra.dailyPlanIDs = m.dailyPlanIDs
		m.showView(ViewProjectRecentActivity, pra)
	default:
	}
	return nil
}

func (m *AppModel) reconstructTaskView(entry NavEntry) tea.Cmd {
	project := m.projectByID(entry.ProjectID)
	if project == nil {
		return nil
	}

	m.switchToProject(project, true)
	m.pushView(ViewOutliner)

	task := project.GetTaskByID(entry.TaskID)
	if task == nil {
		return nil
	}

	switch entry.ViewType {
	case ViewTaskComments:
		m.lastNavigatedTaskID = task.ID
		m.showView(ViewTaskComments, NewTaskCommentsModelWithEditor(task, project, m.runtime.Machine().GetEditor(), m.secondaryFeatures.Subview))
	case ViewTaskDecisions:
		m.lastNavigatedTaskID = task.ID
		m.showView(ViewTaskDecisions, NewTaskDecisionsModelWithEditor(task, project, m.runtime.Machine().GetEditor(), m.secondaryFeatures.Subview))
	case ViewTaskChecklist:
		m.lastNavigatedTaskID = task.ID
		m.showView(ViewTaskChecklist, NewTaskChecklistModelWithEditor(task, project, m.runtime.Machine().GetEditor(), m.secondaryFeatures.Subview))
	case ViewTaskAttachments:
		m.lastNavigatedTaskID = task.ID
		m.showView(ViewTaskAttachments, NewTaskAttachmentsModel(task, project, m.runtime, m.secondaryFeatures.Attachment))
	case ViewTaskActivity:
		m.lastNavigatedTaskID = task.ID
		m.showView(ViewTaskActivity, NewTaskActivityModel(task, project))
	default:
	}
	return nil
}
