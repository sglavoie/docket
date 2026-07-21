package tui

import (
	tea "charm.land/bubbletea/v2"
)

func (m *AppModel) globalTagManagerCommands() []Command {
	return []Command{
		{Name: "navigate to tag project", Description: "Navigate to tag's project", Shortcut: "enter", Category: CategoryActions, Views: []PresentationID{ViewGlobalTagManager}, Aliases: []string{"open tag project"}, Handler: func() tea.Msg { return syntheticSpecialKey(tea.KeyEnter) }},
		{Name: "rename tag", Description: "Rename selected tag", Shortcut: "r", Category: CategoryActions, Views: []PresentationID{ViewGlobalTagManager}, Aliases: []string{"edit tag name"}, Handler: func() tea.Msg { return syntheticKey('r') }},
	}
}

func (m *AppModel) attachmentCommands() []Command {
	return []Command{
		{
			Name:        "add attachment",
			Description: "Add a new attachment to task",
			Shortcut:    "a",
			Category:    CategoryAttachments,
			Views:       []PresentationID{ViewTaskAttachments},
			Aliases:     []string{"attach file", "new attachment"},
			Handler: func() tea.Msg {
				return syntheticKey('a')
			},
		},
		{
			Name:        "open attachment",
			Description: "Open selected attachment (system default)",
			Shortcut:    "o",
			Category:    CategoryAttachments,
			Views:       []PresentationID{ViewTaskAttachments},
			Aliases:     []string{"view attachment", "launch attachment"},
			Handler: func() tea.Msg {
				return syntheticKey('o')
			},
		},
		{
			Name:        "edit attachment",
			Description: "Open selected attachment in editor",
			Shortcut:    "e",
			Category:    CategoryAttachments,
			Views:       []PresentationID{ViewTaskAttachments},
			Aliases:     []string{"edit in editor", "open in editor"},
			Handler: func() tea.Msg {
				return syntheticKey('e')
			},
		},
		{
			Name:        "open attachments directory",
			Description: "Open directory containing attachments",
			Shortcut:    "O",
			Category:    CategoryAttachments,
			Views:       []PresentationID{ViewTaskAttachments},
			Aliases:     []string{"reveal attachments", "attachments folder"},
			Handler: func() tea.Msg {
				return syntheticKey('O')
			},
		},
		{
			Name:        "open original path",
			Description: "Open attachment at its original path",
			Shortcut:    "ctrl+e",
			Category:    CategoryAttachments,
			Views:       []PresentationID{ViewTaskAttachments},
			Aliases:     []string{"original location", "source path"},
			Handler: func() tea.Msg {
				return syntheticCtrlKey('e')
			},
		},
		{
			Name:        "delete attachment",
			Description: "Delete selected attachment",
			Shortcut:    "d",
			Category:    CategoryAttachments,
			Views:       []PresentationID{ViewTaskAttachments},
			Aliases:     []string{"remove attachment"},
			Handler: func() tea.Msg {
				return syntheticKey('d')
			},
		},
		{
			Name:        "unlink attachment",
			Description: "Unlink attachment but keep file",
			Shortcut:    "u",
			Category:    CategoryAttachments,
			Views:       []PresentationID{ViewTaskAttachments},
			Aliases:     []string{"detach file", "keep and unlink"},
			Handler: func() tea.Msg {
				return syntheticKey('u')
			},
		},
		{
			Name:        "copy attachment path",
			Description: "Copy attachment path to clipboard",
			Shortcut:    "y",
			Category:    CategoryAttachments,
			Views:       []PresentationID{ViewTaskAttachments},
			Aliases:     []string{"yank path", "clipboard path"},
			Handler: func() tea.Msg {
				return syntheticKey('y')
			},
		},
	}
}

func (m *AppModel) workflowRulesCommands() []Command {
	return []Command{
		{
			Name:        "create workflow rule",
			Description: "Create a new workflow rule",
			Shortcut:    "n",
			Category:    CategoryWorkflowRules,
			Views:       []PresentationID{ViewWorkflowRules},
			Aliases:     []string{"new rule", "add rule"},
			Handler: func() tea.Msg {
				return syntheticKey('n')
			},
		},
		{
			Name:        "edit workflow rule",
			Description: "Edit selected workflow rule",
			Shortcut:    "e",
			Category:    CategoryWorkflowRules,
			Views:       []PresentationID{ViewWorkflowRules},
			Aliases:     []string{"modify rule"},
			Handler: func() tea.Msg {
				return syntheticKey('e')
			},
		},
		{
			Name:        "delete workflow rule",
			Description: "Delete selected workflow rule",
			Shortcut:    "d",
			Category:    CategoryWorkflowRules,
			Views:       []PresentationID{ViewWorkflowRules},
			Aliases:     []string{"remove rule"},
			Handler: func() tea.Msg {
				return syntheticKey('d')
			},
		},
		{
			Name:        "view workflow rule",
			Description: "View rule details",
			Shortcut:    "enter",
			Category:    CategoryWorkflowRules,
			Views:       []PresentationID{ViewWorkflowRules},
			Aliases:     []string{"open rule", "rule details"},
			Handler: func() tea.Msg {
				return syntheticSpecialKey(tea.KeyEnter)
			},
		},
	}
}

func (m *AppModel) sidebarCommands() []Command {
	return []Command{
		{
			Name:        "enable sidebar for all projects",
			Description: "Turn on sidebar for every active project",
			Category:    CategoryView,
			Views:       []PresentationID{},
			Aliases:     []string{"sidebar on all", "show sidebar all"},
			Available: func() bool {
				return !m.determiningProjectSidebarEnabled()
			},
			Handler: func() tea.Msg {
				m.setAllProjectSidebarsEnabled(true)
				return nil
			},
		},
		{
			Name:        "disable sidebar for all projects",
			Description: "Turn off sidebar for every active project",
			Category:    CategoryView,
			Views:       []PresentationID{},
			Aliases:     []string{"sidebar off all", "hide sidebar all"},
			Available: func() bool {
				return m.determiningProjectSidebarEnabled()
			},
			Handler: func() tea.Msg {
				m.setAllProjectSidebarsEnabled(false)
				return nil
			},
		},
	}
}
