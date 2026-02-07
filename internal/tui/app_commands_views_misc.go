package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m *AppModel) undoHistoryCommands() []Command {
	return []Command{
		{Name: "perform undo", Description: "Undo selected entry or view detail", Shortcut: "enter", Category: CategoryUndoHistory, Views: []ViewType{ViewUndoHistory}, Aliases: []string{"undo entry", "view detail"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyEnter} }},
		{Name: "switch pane", Description: "Switch between entry list and diff pane", Shortcut: "tab", Category: CategoryUndoHistory, Views: []ViewType{ViewUndoHistory}, Aliases: []string{"toggle pane"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyTab} }},
		{Name: "scroll diff left", Description: "Scroll diff pane left", Shortcut: "h", Category: CategoryUndoHistory, Views: []ViewType{ViewUndoHistory}, Aliases: []string{"diff left"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}} }},
		{Name: "scroll diff right", Description: "Scroll diff pane right", Shortcut: "l", Category: CategoryUndoHistory, Views: []ViewType{ViewUndoHistory}, Aliases: []string{"diff right"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}} }},
	}
}

func (m *AppModel) globalTagManagerCommands() []Command {
	return []Command{
		{Name: "navigate to tag project", Description: "Navigate to tag's project", Shortcut: "enter", Category: CategoryActions, Views: []ViewType{ViewGlobalTagManager}, Aliases: []string{"open tag project"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyEnter} }},
		{Name: "rename tag", Description: "Rename selected tag", Shortcut: "r", Category: CategoryActions, Views: []ViewType{ViewGlobalTagManager}, Aliases: []string{"edit tag name"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}} }},
	}
}

func (m *AppModel) attachmentCommands() []Command {
	return []Command{
		{
			Name:        "add attachment",
			Description: "Add a new attachment to task",
			Shortcut:    "a",
			Category:    CategoryAttachments,
			Views:       []ViewType{ViewTaskAttachments},
			Aliases:     []string{"attach file", "new attachment"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
			},
		},
		{
			Name:        "open attachment",
			Description: "Open selected attachment",
			Shortcut:    "o",
			Category:    CategoryAttachments,
			Views:       []ViewType{ViewTaskAttachments},
			Aliases:     []string{"view attachment", "launch attachment"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}}
			},
		},
		{
			Name:        "open attachments directory",
			Description: "Open directory containing attachments",
			Shortcut:    "O",
			Category:    CategoryAttachments,
			Views:       []ViewType{ViewTaskAttachments},
			Aliases:     []string{"reveal attachments", "attachments folder"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'O'}}
			},
		},
		{
			Name:        "open original path",
			Description: "Open attachment at its original path",
			Shortcut:    "ctrl+e",
			Category:    CategoryAttachments,
			Views:       []ViewType{ViewTaskAttachments},
			Aliases:     []string{"original location", "source path"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyCtrlE}
			},
		},
		{
			Name:        "delete attachment",
			Description: "Delete selected attachment",
			Shortcut:    "d",
			Category:    CategoryAttachments,
			Views:       []ViewType{ViewTaskAttachments},
			Aliases:     []string{"remove attachment"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}
			},
		},
		{
			Name:        "unlink attachment",
			Description: "Unlink attachment but keep file",
			Shortcut:    "u",
			Category:    CategoryAttachments,
			Views:       []ViewType{ViewTaskAttachments},
			Aliases:     []string{"detach file", "keep and unlink"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}}
			},
		},
		{
			Name:        "copy attachment path",
			Description: "Copy attachment path to clipboard",
			Shortcut:    "y",
			Category:    CategoryAttachments,
			Views:       []ViewType{ViewTaskAttachments},
			Aliases:     []string{"yank path", "clipboard path"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
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
			Views:       []ViewType{ViewWorkflowRules},
			Aliases:     []string{"new rule", "add rule"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
			},
		},
		{
			Name:        "edit workflow rule",
			Description: "Edit selected workflow rule",
			Shortcut:    "e",
			Category:    CategoryWorkflowRules,
			Views:       []ViewType{ViewWorkflowRules},
			Aliases:     []string{"modify rule"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}
			},
		},
		{
			Name:        "delete workflow rule",
			Description: "Delete selected workflow rule",
			Shortcut:    "d",
			Category:    CategoryWorkflowRules,
			Views:       []ViewType{ViewWorkflowRules},
			Aliases:     []string{"remove rule"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}
			},
		},
		{
			Name:        "view workflow rule",
			Description: "View rule details",
			Shortcut:    "enter",
			Category:    CategoryWorkflowRules,
			Views:       []ViewType{ViewWorkflowRules},
			Aliases:     []string{"open rule", "rule details"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyEnter}
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
			Views:       []ViewType{},
			Aliases:     []string{"sidebar on all", "show sidebar all"},
			Handler: func() tea.Msg {
				for _, p := range m.config.GetActiveProjects() {
					state, _ := m.config.GetProjectViewState(p.ID)
					state.SidebarEnabled = true
					m.config.SetProjectViewState(p.ID, state)
				}
				_ = m.configManager.Save(m.config)
				if m.projectTree != nil {
					m.projectTree.sidebar.SetEnabled(true)
				}
				return nil
			},
		},
		{
			Name:        "disable sidebar for all projects",
			Description: "Turn off sidebar for every active project",
			Category:    CategoryView,
			Views:       []ViewType{},
			Aliases:     []string{"sidebar off all", "hide sidebar all"},
			Handler: func() tea.Msg {
				for _, p := range m.config.GetActiveProjects() {
					state, _ := m.config.GetProjectViewState(p.ID)
					state.SidebarEnabled = false
					m.config.SetProjectViewState(p.ID, state)
				}
				_ = m.configManager.Save(m.config)
				if m.projectTree != nil {
					m.projectTree.sidebar.SetEnabled(false)
				}
				return nil
			},
		},
	}
}
