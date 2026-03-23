package tui

import (
	tea "charm.land/bubbletea/v2"
)

func (m *AppModel) multiSelectCommands() []Command {
	return []Command{
		{
			Name:        "toggle multi-select",
			Description: "Toggle multi-select mode",
			Shortcut:    "v",
			Category:    CategoryMultiSelect,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"multi select", "select mode"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				if m.projectTree.multiSelectMode {
					m.projectTree.clearSelection()
				} else {
					m.projectTree.multiSelectMode = true
					task := m.projectTree.getSelectedTask()
					if task != nil {
						m.projectTree.toggleSelection(task)
					}
				}
				return nil
			},
		},
		{
			Name:        "select all visible",
			Description: "Select all visible tasks",
			Shortcut:    "a",
			Category:    CategoryMultiSelect,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"select all"},
			Handler: func() tea.Msg {
				if m.projectTree != nil && m.projectTree.multiSelectMode {
					m.projectTree.selectAllVisible()
				}
				return nil
			},
		},
		{
			Name:        "deselect all",
			Description: "Deselect all tasks",
			Shortcut:    "A",
			Category:    CategoryMultiSelect,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"clear selection", "deselect"},
			Handler: func() tea.Msg {
				if m.projectTree != nil && m.projectTree.multiSelectMode {
					m.projectTree.deselectAllVisible()
				}
				return nil
			},
		},
	}
}
