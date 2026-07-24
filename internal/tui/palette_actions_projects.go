package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/icons"
)

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
			Icon:    icons.Get("edit"),
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
				Icon:    icons.Get("recur"),
				KeyHint: "r",
				Handler: func() tea.Msg {
					return ProjectArchiveRequestMsg{Project: project, Archive: false}
				},
			})
		} else {
			actions = append(actions, PaletteAction{
				Name:    "Archive",
				Icon:    "📦",
				KeyHint: "x",
				Handler: func() tea.Msg {
					return ProjectArchiveRequestMsg{Project: project, Archive: true}
				},
			})
		}
	}

	pinName := "Pin"
	if project.Pinned {
		pinName = "Unpin"
	}
	pinIcon := icons.Get("pinned")

	actions = append(actions, PaletteAction{
		Name:    pinName,
		Icon:    pinIcon,
		KeyHint: "p",
		Handler: func() tea.Msg {
			return ProjectPinRequestMsg{Project: project, Pinned: !project.Pinned}
		},
	})

	return actions
}
