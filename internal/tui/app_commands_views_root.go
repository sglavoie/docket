package tui

import tea "charm.land/bubbletea/v2"

func (m *AppModel) viewCommands() []Command {
	return []Command{
		{
			Name:        "all tasks",
			Description: "View all tasks across every project",
			Shortcut:    ";a",
			Category:    CategoryViews,
			Views:       []ViewType{ViewProjectTree, ViewOutliner},
			Aliases:     []string{"cross-project tasks", "every task"},
			Handler: func() tea.Msg {
				return ShowAllTasksMsg{}
			},
		},
		{
			Name:        "due date overview",
			Description: "View tasks by due date",
			Shortcut:    ";D",
			Category:    CategoryViews,
			Views:       []ViewType{ViewProjectTree, ViewDashboard, ViewOutliner},
			Handler: func() tea.Msg {
				return ShowDueDateOverviewMsg{}
			},
		},
		{
			Name:        "daily review",
			Description: "Daily planning and review view",
			Shortcut:    ";d",
			Category:    CategoryViews,
			Aliases:     []string{"planning", "today", "morning review"},
			Handler: func() tea.Msg {
				return ShowDailyReviewMsg{}
			},
		},
		{
			Name:        "weekly review",
			Description: "Strategic project health review",
			Shortcut:    ";r",
			Category:    CategoryViews,
			Aliases:     []string{"weekly", "project health", "strategic review"},
			Handler: func() tea.Msg {
				return ShowWeeklyReviewMsg{}
			},
		},
		{
			Name:        "latest activity",
			Description: "View recently modified tasks across all projects",
			Shortcut:    ";l",
			Category:    CategoryViews,
			Aliases:     []string{"recent", "activity", "latest", "recent tasks"},
			Handler: func() tea.Msg {
				return ShowRecentActivityMsg{}
			},
		},
		{
			Name:        "backlog",
			Description: "Cross-project task suggestions and backlog",
			Shortcut:    ";q",
			Category:    CategoryViews,
			Aliases:     []string{"active tasks", "focus", "today tasks", "focus view", "suggestions", "what's next"},
			Handler: func() tea.Msg {
				return ShowFocusViewMsg{}
			},
		},
		{
			Name:        "today",
			Description: "Daily execution board",
			Shortcut:    ";w",
			Category:    CategoryViews,
			Aliases:     []string{"focus board", "daily", "board", "command center", "external items"},
			Handler: func() tea.Msg {
				return ShowFocusBoardMsg{}
			},
		},
		{
			Name:        "priority view",
			Description: "View tasks by priority (toggles cross-project Priority mode in the cross-outliner)",
			Shortcut:    "p",
			Category:    CategoryViews,
			Views:       []ViewType{ViewProjectTree, ViewOutliner, ViewCrossProjectOutliner},
			Aliases:     []string{"priority mode", "toggle priority mode", "cross-project priority"},
			Handler: func() tea.Msg {
				if m.baseProjectView() == ViewCrossProjectOutliner && m.crossOutliner != nil {
					m.crossOutliner.toggleCrossPriorityMode()
					return nil
				}
				if m.currentProject != nil {
					return ShowPriorityViewMsg{Project: m.currentProject}
				}
				return nil
			},
		},
		{
			Name:        "inbox processor",
			Description: "Process inbox items one by one",
			Shortcut:    "W",
			Category:    CategoryViews,
			Views:       projectTreeAndOutlinerViews(),
			Handler: func() tea.Msg {
				if m.currentProject != nil {
					return ShowInboxProcessorMsg{Project: m.currentProject}
				}
				return nil
			},
		},
		{
			Name:        "global inbox processor",
			Description: "Process inbox items across all projects",
			Shortcut:    "ctrl+i",
			Category:    CategoryViews,
			Aliases:     []string{"global inbox", "all inbox", "process all inbox"},
			Handler: func() tea.Msg {
				return ShowGlobalInboxProcessorMsg{}
			},
		},
		{
			Name:        "global notes",
			Description: "Open global notes (not tied to any project)",
			Shortcut:    ";N",
			Category:    CategoryViews,
			Aliases:     []string{"global notes", "notes", "scratch notes"},
			Handler: func() tea.Msg {
				return ShowGlobalNotesMsg{}
			},
		},
		{
			Name:        "global blockers",
			Description: "View blockers across all projects",
			Shortcut:    ";B",
			Category:    CategoryViews,
			Aliases:     []string{"all blockers", "blockers overview"},
			Handler: func() tea.Msg {
				return ShowGlobalBlockersMsg{}
			},
		},
		{
			Name:        "metrics dashboard",
			Description: "View completion rates, cycle time, and throughput",
			Shortcut:    ";m",
			Category:    CategoryViews,
			Aliases:     []string{"metrics", "analytics", "stats", "statistics"},
			Handler: func() tea.Msg {
				return ShowMetricsDashboardMsg{}
			},
		},
		{
			Name:        "roadmap",
			Description: "View Gantt chart, critical path, and forecasts",
			Shortcut:    ";R",
			Category:    CategoryViews,
			Aliases:     []string{"gantt", "timeline", "schedule", "critical path", "forecast"},
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
		{
			Name:        "filter popup",
			Description: "Open the filter popup",
			Shortcut:    "F",
			Category:    CategoryViews,
			Views:       []ViewType{ViewProjectTree},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.pendingPrefix = "F"
				}
				return nil
			},
		},
	}
}
