package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *AppModel) viewCommands() []Command {
	return []Command{
		{
			Name:        "open project outliner",
			Description: "Open highlighted project's outliner",
			Shortcut:    "ctrl+p",
			Category:    CategoryViews,
			Views:       []PresentationID{ViewCrossProjectOutliner},
			Aliases:     []string{"project outliner", "drill into project", "open project view"},
			Handler: func() tea.Msg {
				project := m.paletteCursorProject()
				if project == nil {
					return StatusMsg(statusNoProjectUnderCursor)
				}
				return ShowOutlinerMsg{Project: project}
			},
		},
		{
			Name:        "all tasks",
			Description: "View all tasks across every project",
			Category:    CategoryViews,
			Aliases:     []string{"cross-project tasks", "every task"},
			Handler: func() tea.Msg {
				return ShowGlobalOutlinerMsg{Options: OutlinerOpenOptions{
					Focus: OutlinerAll, SetFocus: true,
					Lens: OutlinerLensTree, SetLens: true,
				}}
			},
		},
		{
			Name:        "outliner",
			Description: "Open the cross-project outliner",
			Category:    CategoryViews,
			Aliases:     []string{"global outliner", "cross-project outliner", "all tasks outliner"},
			Handler: func() tea.Msg {
				return ShowGlobalOutlinerMsg{}
			},
		},
		{
			Name:        "all tasks by due date",
			Description: "View all tasks ordered by due date",
			Category:    CategoryViews,
			Aliases:     []string{"due dates", "by due date", "tasks by due date", "due date overview"},
			Handler: func() tea.Msg {
				return ShowGlobalOutlinerMsg{Options: OutlinerOpenOptions{
					Focus: OutlinerAll, SetFocus: true, Sort: SortByDueDate, SetSort: true,
					Lens: OutlinerLensDueDate, SetLens: true,
				}}
			},
		},
		{
			Name:        "daily review",
			Description: "Daily planning and review view",
			Category:    CategoryViews,
			Aliases:     []string{"planning", "morning review", "DailyReview", "review daily", "ReviewHub/Daily"},
			Handler: func() tea.Msg {
				return ShowDailyReviewMsg{}
			},
		},
		{
			Name:        "weekly review",
			Description: "Strategic project health review",
			Category:    CategoryViews,
			Aliases:     []string{"weekly", "project health", "strategic review", "WeeklyReview", "review weekly", "ReviewHub/Weekly"},
			Handler: func() tea.Msg {
				return ShowWeeklyReviewMsg{}
			},
		},
		{
			Name:        "latest activity",
			Description: "View recently modified tasks across all projects",
			Category:    CategoryViews,
			Aliases:     []string{"recent", "activity", "latest", "recent tasks", "recent activity", "RecentActivity", "project recent activity", "ProjectRecentActivity", "review activity", "completed tasks", "ReviewHub/Activity"},
			Handler: func() tea.Msg {
				return ShowRecentActivityMsg{}
			},
		},
		{
			Name:        "planner next",
			Description: "Planner — Next (on-deck shortlist, StateNext ordered)",
			Category:    CategoryViews,
			Aliases:     []string{"next", "planner on-deck", "queue", "on deck", "shortlist", "what's next"},
			Handler: func() tea.Msg {
				return ShowFocusBoardMsg{Tab: boardTabNext, SetTab: true}
			},
		},
		{
			Name:        "today",
			Description: "Planner — Today cockpit for daily execution",
			Category:    CategoryViews,
			Aliases:     []string{"planner today", "daily", "cockpit", "today cockpit", "command center", "external items"},
			Handler: func() tea.Msg {
				return ShowFocusBoardMsg{Tab: boardTabToday, SetTab: true}
			},
		},
		{
			Name:        "planner backlog",
			Description: "Planner — Backlog (ranked actionable work behind Today/Next)",
			Category:    CategoryViews,
			Aliases:     []string{"backlog ranked", "ranked backlog", "suggestions", "what to work on"},
			Handler: func() tea.Msg {
				return ShowFocusBoardMsg{Tab: boardTabBacklog, SetTab: true}
			},
		},
		{
			Name:        "priority view",
			Description: "Open the High Priority smart focus in Task Workspace",
			Shortcut:    "p",
			Category:    CategoryViews,
			Aliases:     []string{"priority", "priority mode", "toggle priority mode", "high priority", "cross-project priority"},
			Handler: func() tea.Msg {
				if m.currentProject != nil {
					return ShowOutlinerMsg{
						Project: m.currentProject,
						Options: OutlinerOpenOptions{
							Focus: OutlinerHighPriority, SetFocus: true, Sort: SortByScore, SetSort: true,
						},
					}
				}
				return nil
			},
		},
		{
			Name:        "inbox processor",
			Description: "Triage inbox items for this project one by one",
			Shortcut:    "W",
			Category:    CategoryViews,
			Views:       []PresentationID{ViewOutliner, ViewCrossProjectOutliner},
			Aliases:     []string{"triage inbox", "triage project inbox", "process inbox", "inbox triage"},
			Handler: func() tea.Msg {
				project := m.paletteCursorProject()
				if project == nil {
					if m.nonPaletteOrigin() == ViewCrossProjectOutliner {
						return StatusMsg(statusNoProjectUnderCursor)
					}
					return nil
				}
				return ShowInboxProcessorMsg{Project: project}
			},
		},
		{
			Name:        "triage inbox",
			Description: "Triage inbox items for the highlighted project",
			Shortcut:    "O",
			Category:    CategoryViews,
			Views:       []PresentationID{ViewDashboard},
			Aliases:     []string{"inbox triage", "process inbox", "open inbox processor"},
			Handler: func() tea.Msg {
				var project *models.Project
				if m.dashboard != nil {
					project = m.dashboard.SelectedProject()
				}
				if project == nil {
					return nil
				}
				return ShowInboxProcessorMsg{Project: project}
			},
		},
		{
			Name:        "global inbox processor",
			Description: "Triage inbox items across all projects",
			Shortcut:    "ctrl+i",
			Category:    CategoryViews,
			Aliases:     []string{"global inbox", "all inbox", "process all inbox", "triage all inboxes", "triage global inbox"},
			Handler: func() tea.Msg {
				return ShowGlobalInboxProcessorMsg{}
			},
		},
		{
			Name:        "global notes",
			Description: "Open global notes (not tied to any project)",
			Category:    CategoryViews,
			Aliases:     []string{"global notes", "notes", "scratch notes"},
			Handler: func() tea.Msg {
				return ShowGlobalNotesMsg{}
			},
		},
		{
			Name:        "global blockers",
			Description: "View blockers across all projects",
			Category:    CategoryViews,
			Aliases:     []string{"all blockers", "blockers overview"},
			Handler: func() tea.Msg {
				return ShowGlobalBlockersMsg{}
			},
		},
		{
			Name:        "metrics dashboard",
			Description: "View completion rates, cycle time, and throughput",
			Category:    CategoryViews,
			Aliases:     []string{"metrics", "analytics", "stats", "statistics", "MetricsDashboard", "review metrics", "ReviewHub/Metrics"},
			Handler: func() tea.Msg {
				return ShowMetricsDashboardMsg{}
			},
		},
		{
			Name:        "roadmap",
			Description: "View Gantt chart, critical path, and forecasts",
			Category:    CategoryViews,
			Aliases:     []string{"gantt", "timeline", "schedule", "critical path", "forecast", "review roadmap", "ReviewHub/Roadmap"},
			Handler: func() tea.Msg {
				return ShowRoadmapMsg{}
			},
		},
		{
			Name:        "project metrics",
			Description: "View metrics for current project",
			Category:    CategoryViews,
			Views:       projectTreeAndOutlinerViews(),
			Aliases:     []string{"metrics", "analytics", "stats", "statistics"},
			Handler: func() tea.Msg {
				if m.currentProject != nil {
					return ShowMetricsDashboardMsg{Project: m.currentProject}
				}
				return nil
			},
		},
	}
}
