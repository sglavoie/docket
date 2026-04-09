package tui

import tea "charm.land/bubbletea/v2"

func (m *AppModel) projectCommands() []Command {
	return []Command{
		{
			Name:        "new project",
			Description: "Create a new project",
			Shortcut:    "n",
			Category:    CategoryProjects,
			Views:       nil,
			Aliases:     []string{"create project", "add project"},
			Handler: func() tea.Msg {
				return CreateProjectMsg{}
			},
		},
		{
			Name:        "filter by context",
			Description: "Filter projects by context",
			Shortcut:    "C",
			Category:    CategoryFilters,
			Views:       []ViewType{},
			Aliases:     []string{"context filter", "workspace"},
			Handler: func() tea.Msg {
				if m.dashboard != nil {
					return ShowContextPickerMsg{Dashboard: m.dashboard}
				}
				return nil
			},
		},
		{
			Name:        "toggle info panel",
			Description: "Show/hide info panel",
			Shortcut:    "R",
			Category:    CategoryViews,
			Views:       []ViewType{ViewDashboard},
			Aliases:     []string{"recent", "show recent", "info"},
			Handler: func() tea.Msg {
				return ToggleInfoPanelMsg{}
			},
		},
		{
			Name:        "open sandbox",
			Description: "Open the sandbox project",
			Category:    CategoryProjects,
			Aliases:     []string{"sandbox", "scratch", "scratchpad"},
			Handler: func() tea.Msg {
				sandbox := m.config.GetSandboxProject()
				if sandbox != nil {
					return SelectProjectMsg{Project: sandbox}
				}
				return nil
			},
		},
		{
			Name:        "edit project status",
			Description: "Edit status note for the selected project",
			Shortcut:    "S",
			Category:    CategoryProjects,
			Views:       []ViewType{ViewDashboard},
			Aliases:     []string{"status", "project status"},
			Handler: func() tea.Msg {
				if m.dashboard != nil {
					visible := m.dashboard.visibleProjects()
					if CursorInBounds(m.dashboard.cursor, len(visible)) {
						return ShowStatusIconPickerMsg{Project: visible[m.dashboard.cursor]}
					}
				}
				return nil
			},
		},
		{
			Name:        "rename project",
			Description: "Rename the selected project",
			Category:    CategoryProjects,
			Views:       []ViewType{ViewDashboard, ViewProjectTree, ViewOutliner},
			Aliases:     []string{"edit project", "rename"},
			Handler: func() tea.Msg {
				if m.currentView() == ViewDashboard {
					if m.dashboard != nil {
						visible := m.dashboard.visibleProjects()
						if CursorInBounds(m.dashboard.cursor, len(visible)) {
							return EditProjectMsg{Project: visible[m.dashboard.cursor]}
						}
					}
				} else if m.currentProject != nil {
					return EditProjectMsg{Project: m.currentProject}
				}
				return nil
			},
		},
		{
			Name:        "restore from trash",
			Description: "View and restore deleted projects",
			Shortcut:    "T",
			Category:    CategoryProjects,
			Views:       []ViewType{ViewDashboard},
			Aliases:     []string{"trash", "undelete", "recover project"},
			Handler: func() tea.Msg {
				return ShowTrashedProjectsMsg{}
			},
		},
		{
			Name:        "reset sandbox",
			Description: "Clear tasks, notes, and sections from sandbox",
			Category:    CategoryProjects,
			Views:       []ViewType{},
			Aliases:     []string{"clear sandbox"},
			Handler: func() tea.Msg {
				return SandboxResetRequestMsg{}
			},
		},
	}
}
