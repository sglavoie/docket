package tui

import tea "charm.land/bubbletea/v2"

func (m *AppModel) exportCommands() []Command {
	return []Command{{
		Name:        "export project",
		Description: "Export current project to Markdown",
		Shortcut:    "ctrl+e",
		Category:    CategoryProjects,
		Views: []PresentationID{
			ViewDashboard, ViewOutliner,
			ViewCrossProjectOutliner,
		},
		Aliases: []string{"export markdown", "save markdown"},
		Handler: func() tea.Msg { return ExportProjectMsg{} },
	}}
}
