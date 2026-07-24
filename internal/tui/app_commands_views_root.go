package tui

import (
	tea "charm.land/bubbletea/v2"
)

func (m *AppModel) viewCommands() []Command {
	return []Command{
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
			Name:        "weekly review",
			Description: "Strategic project health review",
			Category:    CategoryViews,
			Aliases:     []string{"weekly", "project health", "strategic review", "WeeklyReview", "review weekly"},
			Handler: func() tea.Msg {
				return ShowWeeklyReviewMsg{}
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
