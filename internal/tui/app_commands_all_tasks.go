package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m *AppModel) allTasksCommands() []Command {
	return []Command{
		{
			Name:        "go to task in project",
			Description: "Navigate to task in project tree",
			Shortcut:    "gp",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewAllTasks},
			Aliases:     []string{"open in project", "show in tree"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
			},
		},
		{
			Name:        "cycle grouping",
			Description: "Cycle task grouping mode",
			Shortcut:    "ctrl+g",
			Category:    CategoryActions,
			Views:       []ViewType{ViewAllTasks},
			Aliases:     []string{"group by", "change grouping"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyCtrlG}
			},
		},
		{
			Name:        "toggle multi-select",
			Description: "Enter or exit multi-select mode",
			Shortcut:    "v",
			Category:    CategoryActions,
			Views:       []ViewType{ViewAllTasks},
			Aliases:     []string{"multi select", "bulk select"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}}
			},
		},
		{
			Name:        "collapse/expand group",
			Description: "Toggle group collapse/expand",
			Shortcut:    "tab",
			Category:    CategoryActions,
			Views:       []ViewType{ViewAllTasks},
			Aliases:     []string{"fold group", "toggle group"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyTab}
			},
		},
		{
			Name:        "refresh all tasks",
			Description: "Reload tasks from all projects",
			Shortcut:    "r",
			Category:    CategoryActions,
			Views:       []ViewType{ViewAllTasks},
			Aliases:     []string{"reload tasks"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
			},
		},
		{
			Name:        "copy task title",
			Description: "Copy task title to clipboard",
			Shortcut:    "y",
			Category:    CategoryActions,
			Views:       []ViewType{ViewAllTasks},
			Aliases:     []string{"yank title"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
			},
		},
		{
			Name:        "copy task details",
			Description: "Copy task with details to clipboard",
			Shortcut:    "Y",
			Category:    CategoryActions,
			Views:       []ViewType{ViewAllTasks},
			Aliases:     []string{"yank details"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}}
			},
		},
		{
			Name:        "copy task with description",
			Description: "Copy task title and description",
			Shortcut:    "ctrl+y",
			Category:    CategoryActions,
			Views:       []ViewType{ViewAllTasks},
			Aliases:     []string{"yank description"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyCtrlY}
			},
		},
	}
}
