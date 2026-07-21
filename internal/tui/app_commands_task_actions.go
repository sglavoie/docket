package tui

import (
	tea "charm.land/bubbletea/v2"
)

func (m *AppModel) taskActionCommands() []Command { //nolint:gocyclo
	return []Command{
		{
			Name:        "task activity",
			Description: "View task activity log",
			Shortcut:    "gh",
			Category:    CategoryTasks,
			Views:       []PresentationID{ViewOutliner, ViewCrossProjectOutliner},
			Aliases:     []string{"activity log", "task history"},
			Handler: func() tea.Msg {
				task, project := m.paletteTaskTarget()
				if task == nil || project == nil {
					return StatusMsg(statusNoTaskSelected)
				}
				return OpenTaskActivityMsg{
					Task:    task,
					Project: project,
				}
			},
		},
		{
			Name:        "park task",
			Description: "Park selected task with a resume note",
			Shortcut:    "tP",
			Category:    CategoryTasks,
			Views:       []PresentationID{ViewOutliner, ViewCrossProjectOutliner},
			Aliases:     []string{"parking note", "pause task", "resume note"},
			Handler: func() tea.Msg {
				task, project := m.paletteTaskTarget()
				if task == nil || project == nil {
					return StatusMsg(statusNoTaskSelected)
				}
				return ShowParkInputMsg{
					Task:    task,
					Project: project,
				}
			},
		},
		{
			Name:        "project recent activity",
			Description: "Show recent activity for the current project",
			Shortcut:    "gl",
			Category:    CategoryViews,
			Views:       []PresentationID{ViewOutliner, ViewCrossProjectOutliner},
			Aliases:     []string{"recent activity", "project history"},
			Handler: func() tea.Msg {
				project := m.paletteCursorProject()
				if project == nil {
					if m.nonPaletteOrigin() == ViewCrossProjectOutliner {
						return StatusMsg(statusNoProjectUnderCursor)
					}
					return StatusMsg("No project selected")
				}
				return ShowProjectRecentActivityMsg{Project: project}
			},
		},
	}
}
