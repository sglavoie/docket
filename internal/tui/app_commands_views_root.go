package tui

import (
	tea "charm.land/bubbletea/v2"
)

func (m *AppModel) viewCommands() []Command {
	return []Command{
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
			Description: "Open the global Outliner",
			Category:    CategoryViews,
			Aliases:     []string{"global outliner", "all tasks outliner"},
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
			Aliases:     []string{"planning", "morning review", "DailyReview", "review daily"},
			Handler: func() tea.Msg {
				return ShowDailyReviewMsg{}
			},
		},
		{
			Name:        "weekly review",
			Description: "Strategic project health review",
			Category:    CategoryViews,
			Aliases:     []string{"weekly", "project health", "strategic review", "WeeklyReview", "review weekly"},
			Handler: func() tea.Msg {
				return ShowWeeklyReviewMsg{}
			},
		},
		{
			Name:        "latest activity",
			Description: "View recently modified tasks across all projects",
			Category:    CategoryViews,
			Aliases:     []string{"recent", "activity", "latest", "recent tasks", "recent activity", "RecentActivity", "project recent activity", "ProjectRecentActivity", "review activity", "completed tasks"},
			Handler: func() tea.Msg {
				return ShowRecentActivityMsg{}
			},
		},
		{
			Name:        "priority view",
			Description: "Open the High Priority smart focus in the Outliner",
			Shortcut:    "p",
			Category:    CategoryViews,
			Aliases:     []string{"high priority"},
			Handler: func() tea.Msg {
				// Resolve to the single catalog High Priority mode on the global
				// Outliner rather than a project-scoped focus keyed off the stale
				// m.currentProject. The bare "priority" aliases are retired so that
				// term resolves solely to the catalog Priority lens command.
				return ShowGlobalOutlinerMsg{Options: OutlinerOpenOptions{SetMode: true, Mode: ModeHighPriority}}
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
			Aliases:     []string{"metrics", "analytics", "stats", "statistics", "MetricsDashboard", "review metrics"},
			Handler: func() tea.Msg {
				return ShowMetricsDashboardMsg{}
			},
		},
		{
			Name:        "roadmap",
			Description: "View Gantt chart, critical path, and forecasts",
			Category:    CategoryViews,
			Aliases:     []string{"gantt", "timeline", "schedule", "critical path", "forecast", "review roadmap"},
			Handler: func() tea.Msg {
				return ShowRoadmapMsg{}
			},
		},
		{
			Name:        "project metrics",
			Description: "View metrics for current project",
			Category:    CategoryViews,
			Views:       []PresentationID{ViewOutliner, ViewCrossProjectOutliner},
			Aliases:     []string{"metrics", "analytics", "stats", "statistics"},
			Handler: func() tea.Msg {
				project := m.paletteCursorProject()
				if project == nil {
					if m.nonPaletteOrigin() == ViewCrossProjectOutliner {
						return StatusMsg(statusNoProjectUnderCursor)
					}
					return nil
				}
				return ShowMetricsDashboardMsg{Project: project}
			},
		},
	}
}
