package tui

import tea "charm.land/bubbletea/v2"

// getProjectActions returns inline actions for a project item.
func (m *CommandPaletteModel) getProjectActions(item PaletteItem) []PaletteAction {
	if item.Project == nil {
		return nil
	}
	project := item.Project

	actions := []PaletteAction{
		{
			Name:    "Open",
			Icon:    "→",
			KeyHint: "enter",
			Handler: func() tea.Msg {
				return SelectProjectMsg{Project: project}
			},
		},
		{
			Name:    "Edit",
			Icon:    "✏",
			KeyHint: "e",
			Handler: func() tea.Msg {
				return EditProjectMsg{Project: project}
			},
		},
	}

	if !project.Sandbox {
		if project.Archived {
			actions = append(actions, PaletteAction{
				Name:    "Restore",
				Icon:    "↺",
				KeyHint: "r",
				Handler: func() tea.Msg {
					project.Unarchive()
					return ProjectRestoredMsg{Project: project}
				},
			})
		} else {
			actions = append(actions, PaletteAction{
				Name:    "Archive",
				Icon:    "📦",
				KeyHint: "x",
				Handler: func() tea.Msg {
					project.Archive()
					return ProjectArchivedMsg{Project: project}
				},
			})
		}
	}

	pinName := "Pin"
	pinIcon := "📌"
	if project.Pinned {
		pinName = "Unpin"
		pinIcon = "📍"
	}

	actions = append(actions, PaletteAction{
		Name:    pinName,
		Icon:    pinIcon,
		KeyHint: "p",
		Handler: func() tea.Msg {
			project.TogglePinned()
			return ProjectPinnedToggledMsg{Project: project}
		},
	})

	return actions
}