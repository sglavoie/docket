package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m *AppModel) blockersCommands() []Command {
	return []Command{
		{Name: "add blocker", Description: "Add a new blocker", Shortcut: "a", Category: CategoryBlockers, Views: []ViewType{ViewBlockers}, Aliases: []string{"new blocker", "n"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}} }},
		{Name: "edit blocker", Description: "Edit selected blocker", Shortcut: "e", Category: CategoryBlockers, Views: []ViewType{ViewBlockers}, Aliases: []string{"modify blocker"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}} }},
		{Name: "edit in editor", Description: "Edit blocker in external editor", Shortcut: "E", Category: CategoryBlockers, Views: []ViewType{ViewBlockers}, Aliases: []string{"external editor"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}} }},
		{Name: "toggle resolve", Description: "Toggle resolve/unresolve blocker", Shortcut: "R", Category: CategoryBlockers, Views: []ViewType{ViewBlockers}, Aliases: []string{"resolve blocker", "unresolve"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}} }},
		{Name: "delete blocker", Description: "Delete selected blocker", Shortcut: "d", Category: CategoryBlockers, Views: []ViewType{ViewBlockers}, Aliases: []string{"remove blocker", "x"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}} }},
		{Name: "toggle show resolved", Description: "Toggle showing resolved blockers", Shortcut: "tab", Category: CategoryBlockers, Views: []ViewType{ViewBlockers}, Aliases: []string{"show resolved"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyTab} }},
		{Name: "expand blocker", Description: "Expand/view blocker details", Shortcut: "enter", Category: CategoryBlockers, Views: []ViewType{ViewBlockers}, Aliases: []string{"view blocker"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyEnter} }},
	}
}

func (m *AppModel) taskCommentsCommands() []Command {
	return []Command{
		{Name: "add comment", Description: "Add a new comment", Shortcut: "a", Category: CategoryComments, Views: []ViewType{ViewTaskComments}, Aliases: []string{"new comment", "n"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}} }},
		{Name: "edit comment", Description: "Edit selected comment", Shortcut: "e", Category: CategoryComments, Views: []ViewType{ViewTaskComments}, Aliases: []string{"modify comment"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}} }},
		{Name: "edit in editor", Description: "Edit comment in external editor", Shortcut: "E", Category: CategoryComments, Views: []ViewType{ViewTaskComments}, Aliases: []string{"external editor"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}} }},
		{Name: "delete comment", Description: "Delete selected comment", Shortcut: "d", Category: CategoryComments, Views: []ViewType{ViewTaskComments}, Aliases: []string{"remove comment", "x"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}} }},
		{Name: "expand comment", Description: "Expand/view comment details", Shortcut: "enter", Category: CategoryComments, Views: []ViewType{ViewTaskComments}, Aliases: []string{"view comment"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyEnter} }},
		{Name: "copy comment", Description: "Copy comment text to clipboard", Shortcut: "y", Category: CategoryComments, Views: []ViewType{ViewTaskComments}, Aliases: []string{"yank comment"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}} }},
	}
}
