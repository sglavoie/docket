package tui

import (
	tea "charm.land/bubbletea/v2"
)

func (m *AppModel) blockersCommands() []Command {
	return []Command{
		{Name: "add blocker", Description: "Add a new blocker", Shortcut: "a", Category: CategoryBlockers, Views: []PresentationID{ViewBlockers}, Aliases: []string{"new blocker", "n"}, Handler: func() tea.Msg { return syntheticKey('a') }},
		{Name: "edit blocker", Description: "Edit selected blocker", Shortcut: "e", Category: CategoryBlockers, Views: []PresentationID{ViewBlockers}, Aliases: []string{"modify blocker"}, Handler: func() tea.Msg { return syntheticKey('e') }},
		{Name: "edit in editor", Description: "Edit blocker in external editor", Shortcut: "E", Category: CategoryBlockers, Views: []PresentationID{ViewBlockers}, Aliases: []string{"external editor"}, Handler: func() tea.Msg { return syntheticKey('E') }},
		{Name: "toggle resolve", Description: "Toggle resolve/unresolve blocker", Shortcut: "R", Category: CategoryBlockers, Views: []PresentationID{ViewBlockers}, Aliases: []string{"resolve blocker", "unresolve"}, Handler: func() tea.Msg { return syntheticKey('R') }},
		{Name: "delete blocker", Description: "Delete selected blocker", Shortcut: "d", Category: CategoryBlockers, Views: []PresentationID{ViewBlockers}, Aliases: []string{"remove blocker", "x"}, Handler: func() tea.Msg { return syntheticKey('d') }},
		{Name: "toggle show resolved", Description: "Toggle showing resolved blockers", Shortcut: "tab", Category: CategoryBlockers, Views: []PresentationID{ViewBlockers}, Aliases: []string{"show resolved"}, Handler: func() tea.Msg { return syntheticSpecialKey(tea.KeyTab) }},
		{Name: "expand blocker", Description: "Expand/view blocker details", Shortcut: "enter", Category: CategoryBlockers, Views: []PresentationID{ViewBlockers}, Aliases: []string{"view blocker"}, Handler: func() tea.Msg { return syntheticSpecialKey(tea.KeyEnter) }},
	}
}

func (m *AppModel) taskCommentsCommands() []Command {
	return []Command{
		{Name: "add comment", Description: "Add a new comment", Shortcut: "a", Category: CategoryComments, Views: []PresentationID{ViewTaskComments}, Aliases: []string{"new comment", "n"}, Handler: func() tea.Msg { return syntheticKey('a') }},
		{Name: "edit comment", Description: "Edit selected comment", Shortcut: "e", Category: CategoryComments, Views: []PresentationID{ViewTaskComments}, Aliases: []string{"modify comment"}, Handler: func() tea.Msg { return syntheticKey('e') }},
		{Name: "edit in editor", Description: "Edit comment in external editor", Shortcut: "E", Category: CategoryComments, Views: []PresentationID{ViewTaskComments}, Aliases: []string{"external editor"}, Handler: func() tea.Msg { return syntheticKey('E') }},
		{Name: "delete comment", Description: "Delete selected comment", Shortcut: "d", Category: CategoryComments, Views: []PresentationID{ViewTaskComments}, Aliases: []string{"remove comment", "x"}, Handler: func() tea.Msg { return syntheticKey('d') }},
		{Name: "expand comment", Description: "Expand/view comment details", Shortcut: "enter", Category: CategoryComments, Views: []PresentationID{ViewTaskComments}, Aliases: []string{"view comment"}, Handler: func() tea.Msg { return syntheticSpecialKey(tea.KeyEnter) }},
		{Name: "copy comment", Description: "Copy comment text to clipboard", Shortcut: "y", Category: CategoryComments, Views: []PresentationID{ViewTaskComments}, Aliases: []string{"yank comment"}, Handler: func() tea.Msg { return syntheticKey('y') }},
	}
}
