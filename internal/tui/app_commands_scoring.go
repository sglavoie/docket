package tui

import tea "charm.land/bubbletea/v2"

func (m *AppModel) scoringCommands() []Command {
	return []Command{
		{
			Name:        "scoring profiles",
			Description: "Manage scoring profiles and project assignment",
			Category:    CategoryNavigation,
			Aliases:     []string{"score profiles", "scoring", "score"},
			Handler: func() tea.Msg {
				return ShowScoreProfileManagerMsg{}
			},
		},
	}
}
