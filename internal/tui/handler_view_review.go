package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
)

// handleViewReviewMessages handles overview, review, focus, and activity views.
func (m *AppModel) handleViewReviewMessages(msg tea.Msg) (bool, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case ShowReviewHubMsg:
		return true, m.activateReviewHub(msg.Module, msg.Project)

	case ReviewHubModuleSelectedMsg:
		return true, m.activateReviewHub(msg.Module, msg.Project)

	case ShowMetricsDashboardMsg:
		return true, m.activateReviewHub(ReviewMetrics, msg.Project)

	case MetricsDashboardSelectMsg:
		return m.popAndNavigateToTask(msg.Task, msg.Project)

	case ShowDailyReviewMsg:
		return true, m.activateReviewHub(ReviewDaily, nil)

	case MarkDailyReviewDoneMsg:
		if err := m.commitPlanningMutation(func(p *config.PlanningState) {
			p.SetLastDailyReviewAt(time.Now())
		}); err != nil {
			logging.Warn("failed to save daily review completion: %v", err)
		}
		m.reevaluateAlerts()
		m.setSuccessStatus("Review complete! Next: open Planner Today")
		return true, nil

	case MarkWeeklyReviewDoneMsg:
		if err := m.commitPlanningMutation(func(p *config.PlanningState) {
			p.SetLastWeeklyReviewAt(time.Now())
		}); err != nil {
			logging.Warn("failed to save weekly review completion: %v", err)
		}
		m.reevaluateAlerts()
		m.setSuccessStatus("Weekly review completed")
		return true, nil

	case ResetDailyReviewMsg:
		if err := m.commitPlanningMutation(func(p *config.PlanningState) {
			p.ClearLastDailyReviewAt()
		}); err != nil {
			logging.Warn("failed to reset daily review completion: %v", err)
		}
		m.reevaluateAlerts()
		m.setSuccessStatus("Daily review reset")
		return true, nil

	case ResetWeeklyReviewMsg:
		if err := m.commitPlanningMutation(func(p *config.PlanningState) {
			p.ClearLastWeeklyReviewAt()
		}); err != nil {
			logging.Warn("failed to reset weekly review completion: %v", err)
		}
		m.reevaluateAlerts()
		m.setSuccessStatus("Weekly review reset")
		return true, nil

	case ShowWeeklyReviewMsg:
		return true, m.activateReviewHub(ReviewWeekly, nil)

	case WeeklyReviewSelectProjectMsg:
		m.popView()
		if msg.Project != nil {
			return true, m.navigateToTaskInProject(nil, msg.Project)
		}
		return true, nil

	case WeeklyReviewSelectTaskMsg:
		return m.popAndNavigateToTask(msg.Task, msg.Project)

	case DailyReviewSelectMsg:
		return m.popAndNavigateToTask(msg.Task, msg.Project)

	case ShowAllTasksMsg:
		if m.typedNavigation && m.taskWorkspaceView != nil {
			if msg.SetGroup {
				_ = m.taskWorkspace.State.SetGrouping(workspaceGroupingFromAllTasks(msg.Group))
			}
			if msg.SetSort {
				_ = m.taskWorkspace.State.SetSorting(SortState{Field: msg.Sort, Ascending: true})
			}
			var opts []AllTasksOption
			if msg.SetGroup {
				opts = append(opts, WithInitialGrouping(msg.Group))
			}
			if msg.SetSort {
				opts = append(opts, WithInitialSort(msg.Sort, true))
			}
			m.allTasksView = NewAllTasksViewModel(m.runtime, m.configManager, opts...)
			m.allTasksView.bulk = m.secondaryFeatures.Bulk
			m.allTasksView.queueIDs = m.focusQueueIDs
			m.allTasksView.dailyPlanIDs = m.dailyPlanIDs
			m.allTasksView.SetSize(m.width, m.height-2)
			m.wireSecondaryCoordinators(m.allTasksView)
			m.views.Register(ViewAllTasks, m.allTasksView)
			route, err := NewRoute(TaskWorkspace, TaskWorkspaceParams{
				Scope: AppEntityScope(), Layout: WorkspaceList, Focus: WorkspaceActive,
			})
			if err != nil {
				return true, func() tea.Msg { return ErrorMsg{Err: err} }
			}
			return true, m.activateTaskWorkspaceRoute(route)
		}
		var opts []AllTasksOption
		if msg.SetGroup {
			opts = append(opts, WithInitialGrouping(msg.Group))
		}
		if msg.SetSort {
			opts = append(opts, WithInitialSort(msg.Sort, true))
		}
		atv := NewAllTasksViewModel(m.runtime, m.configManager, opts...)
		atv.bulk = m.secondaryFeatures.Bulk
		atv.queueIDs = m.focusQueueIDs
		atv.dailyPlanIDs = m.dailyPlanIDs
		m.showView(ViewAllTasks, atv)
		return true, nil

	case AllTasksSelectMsg:
		return m.popAndNavigateToTask(msg.Task, msg.Project)

	case ShowFocusBoardMsg:
		m.reevaluateAlerts()
		fb := NewFocusBoardModel(m.runtime, m.configManager, &m.dailyPlan, &m.focusQueue, m.focusQueueIDs, m.dailyPlanIDs, m.dailyPlanCarriedIDs)
		if msg.SetTab {
			if _, valid := plannerTabFromBoard(msg.Tab); !valid {
				msg.Tab = boardTabToday
			}
			fb.tabs.Active = msg.Tab
			fb.initialTabSet = true
			fb.applyFilter()
			fb.clampCursor()
		}
		if m.dailyPlanCarried > 0 && !m.dailyPlanCarriedShown {
			fb.setStatus(fmt.Sprintf("%d item(s) carried over from yesterday", m.dailyPlanCarried))
			m.dailyPlanCarriedShown = true
		}
		m.showView(ViewFocusBoard, fb)
		if m.typedNavigation && m.navigation.Primary.Current().ID() == Planner {
			tab, _ := plannerTabFromBoard(fb.tabs.Active)
			route, err := NewRoute(Planner, PlannerParams{Tab: tab})
			if err == nil {
				route = withPresentation(route, ViewFocusBoard, "", "")
				_ = m.navigation.Primary.ReplaceCurrentMode(route)
				m.syncPresentationStack()
			}
		}
		return true, nil

	case FocusBoardSelectMsg:
		return m.popAndNavigateToTask(msg.Task, msg.Project)

	case FocusBoardRefreshMsg:
		if v := m.views.Get(ViewFocusBoard); v != nil {
			v.Update(msg)
		}
		return true, nil

	case ShowRecentActivityMsg:
		return true, m.activateReviewHub(ReviewActivity, nil)

	case ShowProjectRecentActivityMsg:
		if msg.Project != nil {
			return true, m.activateReviewHub(ReviewActivity, msg.Project)
		}
		return true, nil

	case ShowRecentActivityPreviewMsg:
		preview := NewRecentActivityPreviewModel(msg.Task, msg.ProjectScope)
		m.showView(ViewRecentActivityPreview, preview)
		return true, nil

	case RecentActivitySelectMsg:
		return m.popAndNavigateToTask(msg.Task, msg.Project)

	case ShowProjectFocusViewMsg:
		if msg.Project != nil {
			if m.typedNavigation && m.taskWorkspaceView != nil {
				m.currentProject = msg.Project
				_ = m.taskWorkspace.State.SetGrouping(WorkspaceGroupNone)
				_ = m.taskWorkspace.State.SetSorting(NewSortState())
				route, err := NewRoute(TaskWorkspace, TaskWorkspaceParams{
					Scope: ProjectEntityScope(msg.Project.ID), Layout: WorkspaceList, Focus: WorkspaceNext,
				})
				if err != nil {
					return true, func() tea.Msg { return ErrorMsg{Err: err} }
				}
				return true, m.activateTaskWorkspaceRoute(route)
			}
			m.showView(ViewProjectFocusView, NewProjectFocusViewModel(msg.Project, m.runtime))
		}
		return true, nil

	case ShowRoadmapMsg:
		return true, m.activateReviewHub(ReviewRoadmap, msg.Project)

	case RoadmapSelectMsg:
		return m.popAndNavigateToTask(msg.Task, msg.Project)

	case ShowKanbanMsg:
		if msg.Project != nil {
			if m.typedNavigation && m.taskWorkspaceView != nil {
				m.currentProject = msg.Project
				_ = m.taskWorkspace.State.SetBoardAxis(WorkspaceBoardByStatus)
				route, err := NewRoute(TaskWorkspace, TaskWorkspaceParams{
					Scope: ProjectEntityScope(msg.Project.ID), Layout: WorkspaceBoard, Focus: WorkspaceActive,
				})
				if err != nil {
					return true, func() tea.Msg { return ErrorMsg{Err: err} }
				}
				return true, m.activateTaskWorkspaceRoute(route)
			}
			kanban := NewKanbanModel(msg.Project, m.configManager, m.runtime, NewProjectService())
			m.restoreKanbanState(kanban)
			m.showView(ViewKanban, kanban)
		}
		return true, nil

	case ShowCrossProjectKanbanMsg:
		if m.typedNavigation && m.taskWorkspaceView != nil {
			_ = m.taskWorkspace.State.SetBoardAxis(WorkspaceBoardByStatus)
			route, err := NewRoute(TaskWorkspace, TaskWorkspaceParams{
				Scope: AppEntityScope(), Layout: WorkspaceBoard, Focus: WorkspaceActive,
			})
			if err != nil {
				return true, func() tea.Msg { return ErrorMsg{Err: err} }
			}
			return true, m.activateTaskWorkspaceRoute(route)
		}
		kanban := NewCrossProjectKanbanModel(m.configManager, m.runtime, NewProjectService())
		m.restoreKanbanState(kanban)
		m.showView(ViewCrossProjectKanban, kanban)
		return true, nil

	case KanbanSelectMsg:
		return m.popAndNavigateToTask(msg.Task, msg.Project)
	}

	return false, nil
}

func workspaceGroupingFromAllTasks(group AllTasksGroupBy) TaskWorkspaceGroupBy {
	switch group {
	case ATGroupByProject:
		return WorkspaceGroupProject
	case ATGroupByStatus:
		return WorkspaceGroupStatus
	case ATGroupByPriority:
		return WorkspaceGroupPriority
	case ATGroupBySection:
		return WorkspaceGroupSection
	case ATGroupByDueDate:
		return WorkspaceGroupDueDate
	default:
		return WorkspaceGroupNone
	}
}
