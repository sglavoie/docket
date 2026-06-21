package tui

import tea "charm.land/bubbletea/v2"

func (m *AppModel) globalCommands() []Command {
	return []Command{
		{
			Name:        "help",
			Description: "Show help for keyboard shortcuts",
			Shortcut:    "?",
			Category:    CategoryHelp,
			Aliases:     []string{"keyboard shortcuts", "keybindings", "hotkeys"},
			Handler: func() tea.Msg {
				return ShowHelpMsg{
					SourceView: ViewDashboard,
					ViewName:   "tt",
					Items:      getGlobalHelpItems(),
				}
			},
		},
		{
			Name:        "keybinding explorer",
			Description: "Browse all keybindings across all views",
			Shortcut:    `ctrl+\`,
			Category:    CategoryHelp,
			Aliases:     []string{"shortcuts", "keys", "bindings", "hotkeys", "explorer"},
			Handler: func() tea.Msg {
				return ShowKeybindingExplorerMsg{}
			},
		},
		{
			Name:        "quit",
			Description: "Quit the application",
			Shortcut:    "ctrl+c",
			Category:    CategoryNavigation,
			Aliases:     []string{"exit", "close"},
			Handler: func() tea.Msg {
				return QuitMsg{}
			},
		},
		{
			Name:        "global search",
			Description: "Search tasks across all projects",
			Shortcut:    "ctrl+/",
			Category:    CategorySearch,
			Aliases:     []string{"find tasks", "search all"},
			Handler: func() tea.Msg {
				return ShowGlobalSearchMsg{}
			},
		},
		{
			Name:        "settings",
			Description: "Open application settings",
			Shortcut:    "ctrl+,",
			Category:    CategoryNavigation,
			Aliases:     []string{"preferences", "config", "options"},
			Handler: func() tea.Msg {
				return SettingsMsg{}
			},
		},
		{
			Name:        "notifications",
			Description: "Open the Notification Center",
			Shortcut:    ";n",
			Category:    CategoryNavigation,
			Aliases:     []string{"notification center", "messages", "alerts"},
			Handler: func() tea.Msg {
				return ShowNotificationCenterMsg{}
			},
		},
		{
			Name:        "message log",
			Description: "Show recent status messages",
			Shortcut:    "alt+l",
			Category:    CategoryNavigation,
			Aliases:     []string{"status log", "status history", "log"},
			Handler: func() tea.Msg {
				return ShowMessageLogMsg{}
			},
		},
		{
			Name:        "tutor",
			Description: "Open the interactive tutor",
			Category:    CategoryNavigation,
			Aliases:     []string{"tutorial", "learn", "guide"},
			Handler: func() tea.Msg {
				return ShowTutorMsg{}
			},
		},
		{
			Name:        "ask ai",
			Description: "Ask the AI assistant about your tasks",
			Category:    CategoryNavigation,
			Aliases:     []string{"ask", "ai", "claude"},
			Handler: func() tea.Msg {
				return ShowAskMsg{}
			},
		},
		{
			Name:        "stages settings",
			Description: "Open stages configuration",
			Category:    CategoryNavigation,
			Aliases:     []string{"manage stages", "workflow stages"},
			Handler: func() tea.Msg {
				return SettingsStagesMsg{}
			},
		},
		{
			Name:        "back",
			Description: "Go back to previous view",
			Shortcut:    "esc",
			Category:    CategoryNavigation,
			Aliases:     []string{"go back", "return", "previous"},
			Handler: func() tea.Msg {
				return BackMsg{}
			},
		},
		{
			Name:        "switch project",
			Description: "Open project picker to switch projects",
			Shortcut:    "ctrl+X",
			Category:    CategoryNavigation,
			Aliases:     []string{"project picker", "change project", "open project"},
			Handler: func() tea.Msg {
				return ShowProjectPickerMsg{}
			},
		},
		{
			Name:        "workflow rules",
			Description: "Manage workflow automation rules",
			Shortcut:    ";W",
			Category:    CategoryNavigation,
			Aliases:     []string{"automation", "rules", "workflow"},
			Handler: func() tea.Msg {
				return ShowWorkflowRulesMsg{}
			},
		},
		{
			Name:        "new workflow rule",
			Description: "Create a new workflow automation rule",
			Category:    CategoryActions,
			Aliases:     []string{"create rule", "add rule", "new automation"},
			Handler: func() tea.Msg {
				return ShowWorkflowRuleCreatorMsg{}
			},
		},
		{
			Name:        "task templates",
			Description: "Manage task templates",
			Category:    CategoryNavigation,
			Aliases:     []string{"templates", "template list"},
			Handler: func() tea.Msg {
				return ShowTaskTemplatesMsg{}
			},
		},
		{
			Name:        "create task template",
			Description: "Create a new task template",
			Category:    CategoryActions,
			Aliases:     []string{"new template", "add template"},
			Handler: func() tea.Msg {
				return ShowTaskTemplateCreatorMsg{}
			},
		},
	}
}
