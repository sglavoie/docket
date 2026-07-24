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
			Views:       []PresentationID{},
			Aliases:     []string{"context filter", "workspace"},
			Handler: func() tea.Msg {
				return ShowContextPickerMsg{}
			},
		},
		{
			Name:        "open sandbox",
			Description: "Open the sandbox project",
			Category:    CategoryProjects,
			Aliases:     []string{"sandbox", "scratch", "scratchpad"},
			Handler: func() tea.Msg {
				sandbox := m.projectByID("_sandbox")
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
			Views:       []PresentationID{ViewCrossProjectOutliner},
			Aliases:     []string{"status", "project status"},
			Handler: func() tea.Msg {
				if project := m.paletteCursorProject(); project != nil {
					return ShowStatusIconPickerMsg{Project: project}
				}
				return StatusMsg(statusNoProjectUnderCursor)
			},
		},
		{
			Name:        "rename project",
			Description: "Rename the selected project",
			Category:    CategoryProjects,
			Views:       []PresentationID{ViewCrossProjectOutliner, ViewOutliner},
			Aliases:     []string{"edit project", "rename"},
			Handler: func() tea.Msg {
				project := m.paletteCursorProject()
				if project == nil {
					if m.nonPaletteOrigin() == ViewCrossProjectOutliner {
						return StatusMsg(statusNoProjectUnderCursor)
					}
					return nil
				}
				return EditProjectMsg{Project: project}
			},
		},
		{
			Name:        "restore from trash",
			Description: "View and restore deleted projects",
			Shortcut:    "T",
			Category:    CategoryProjects,
			Views:       []PresentationID{ViewDashboard},
			Aliases:     []string{"trash", "undelete", "recover project"},
			Handler: func() tea.Msg {
				return ShowTrashedProjectsMsg{}
			},
		},
		{
			Name:        "reset sandbox",
			Description: "Clear tasks, notes, and sections from sandbox",
			Category:    CategoryProjects,
			Views:       []PresentationID{},
			Aliases:     []string{"clear sandbox"},
			Handler: func() tea.Msg {
				return SandboxResetRequestMsg{}
			},
		},
	}
}
