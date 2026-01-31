package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m *AppModel) focusViewCommands() []Command {
	return []Command{
		{
			Name:        "remove from focus",
			Description: "Remove active flag from task",
			Shortcut:    "*",
			Category:    CategoryActions,
			Views:       []ViewType{ViewFocusView, ViewProjectFocusView},
			Aliases:     []string{"unflag task", "unstar task", "deactivate"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'*'}}
			},
		},
		{
			Name:        "go to task in project",
			Description: "Navigate to task in project tree",
			Shortcut:    "gp",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewFocusView, ViewProjectFocusView},
			Aliases:     []string{"open in project", "show in tree"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
			},
		},
		{
			Name:        "copy task title",
			Description: "Copy task title to clipboard",
			Shortcut:    "y",
			Category:    CategoryActions,
			Views:       []ViewType{ViewFocusView, ViewProjectFocusView},
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
			Views:       []ViewType{ViewFocusView, ViewProjectFocusView},
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
			Views:       []ViewType{ViewFocusView, ViewProjectFocusView},
			Aliases:     []string{"yank description"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyCtrlY}
			},
		},
	}
}

func (m *AppModel) dailyReviewCommands() []Command {
	return []Command{
		{
			Name:        "focus on due today",
			Description: "Focus on due today section",
			Shortcut:    "sd",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"due today section"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
			},
		},
		{
			Name:        "focus on top priority",
			Description: "Focus on top priority section",
			Shortcut:    "st",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"top priority section"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
			},
		},
		{
			Name:        "focus on stale tasks",
			Description: "Focus on stale tasks section",
			Shortcut:    "ss",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"stale section"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
			},
		},
		{
			Name:        "focus on recently logged",
			Description: "Focus on recently logged section",
			Shortcut:    "sl",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"recently logged section"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
			},
		},
		{
			Name:        "focus on flagged tasks",
			Description: "Focus on flagged/active tasks section",
			Shortcut:    "sf",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"flagged section", "active section"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
			},
		},
		{
			Name:        "focus on overdue tasks",
			Description: "Focus on overdue tasks section",
			Shortcut:    "so",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"overdue section"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
			},
		},
		{
			Name:        "collapse/expand section",
			Description: "Toggle section collapse/expand",
			Shortcut:    "tab",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"fold section", "toggle section"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyTab}
			},
		},
		{
			Name:        "next section",
			Description: "Jump to next section",
			Shortcut:    "n",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"section forward"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
			},
		},
		{
			Name:        "previous section",
			Description: "Jump to previous section",
			Shortcut:    "N",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"section backward"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}}
			},
		},
		{
			Name:        "copy task title",
			Description: "Copy task title to clipboard",
			Shortcut:    "y",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDailyReview},
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
			Views:       []ViewType{ViewDailyReview},
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
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"yank description"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyCtrlY}
			},
		},
	}
}

func (m *AppModel) priorityViewCommands() []Command {
	return []Command{
		{
			Name:        "start suggested task",
			Description: "Start the suggested task",
			Shortcut:    "s",
			Category:    CategoryActions,
			Views:       []ViewType{ViewPriorityView},
			Aliases:     []string{"begin suggestion", "accept suggestion"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
			},
		},
		{
			Name:        "dismiss suggestion",
			Description: "Dismiss the current suggestion",
			Shortcut:    "x",
			Category:    CategoryActions,
			Views:       []ViewType{ViewPriorityView},
			Aliases:     []string{"skip suggestion", "reject suggestion"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
			},
		},
		{
			Name:        "go to task in tree",
			Description: "Navigate to task in project tree",
			Shortcut:    "gt",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewPriorityView},
			Aliases:     []string{"open in tree", "show in tree"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
			},
		},
		{
			Name:        "toggle waiting section",
			Description: "Toggle waiting-on section visibility",
			Shortcut:    "tab",
			Category:    CategoryActions,
			Views:       []ViewType{ViewPriorityView},
			Aliases:     []string{"show waiting", "hide waiting"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyTab}
			},
		},
		{
			Name:        "refresh priority view",
			Description: "Refresh priority data",
			Shortcut:    "r",
			Category:    CategoryActions,
			Views:       []ViewType{ViewPriorityView},
			Aliases:     []string{"reload priorities"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
			},
		},
		{
			Name:        "copy task title",
			Description: "Copy task title to clipboard",
			Shortcut:    "y",
			Category:    CategoryActions,
			Views:       []ViewType{ViewPriorityView},
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
			Views:       []ViewType{ViewPriorityView},
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
			Views:       []ViewType{ViewPriorityView},
			Aliases:     []string{"yank description"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyCtrlY}
			},
		},
	}
}

func (m *AppModel) inboxProcessorCommands() []Command {
	return []Command{
		{
			Name:        "skip task",
			Description: "Skip current inbox task",
			Shortcut:    "n",
			Category:    CategoryActions,
			Views:       []ViewType{ViewInboxProcessor},
			Aliases:     []string{"next inbox task", "pass"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
			},
		},
		{
			Name:        "assign section",
			Description: "Assign task to a section",
			Shortcut:    "s",
			Category:    CategoryActions,
			Views:       []ViewType{ViewInboxProcessor},
			Aliases:     []string{"pick section", "move to section"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
			},
		},
		{
			Name:        "send to waiting",
			Description: "Send task to Waiting section",
			Shortcut:    "w",
			Category:    CategoryActions,
			Views:       []ViewType{ViewInboxProcessor},
			Aliases:     []string{"waiting", "defer task"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}}
			},
		},
		{
			Name:        "manage inbox task tags",
			Description: "Add or remove tags on task",
			Shortcut:    "t",
			Category:    CategoryActions,
			Views:       []ViewType{ViewInboxProcessor},
			Aliases:     []string{"tag inbox task"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}}
			},
		},
		{
			Name:        "create subtask from inbox",
			Description: "Create a subtask for current task",
			Shortcut:    "c",
			Category:    CategoryActions,
			Views:       []ViewType{ViewInboxProcessor},
			Aliases:     []string{"add subtask inbox"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}
			},
		},
		{
			Name:        "set inbox task due date",
			Description: "Set due date for current task",
			Shortcut:    "D",
			Category:    CategoryActions,
			Views:       []ViewType{ViewInboxProcessor},
			Aliases:     []string{"inbox due date", "deadline"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}}
			},
		},
		{
			Name:        "manage inbox dependencies",
			Description: "Manage task dependencies",
			Shortcut:    "b",
			Category:    CategoryActions,
			Views:       []ViewType{ViewInboxProcessor},
			Aliases:     []string{"inbox dependencies", "add dependency"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}}
			},
		},
		{
			Name:        "delete inbox task",
			Description: "Delete current inbox task",
			Shortcut:    "x",
			Category:    CategoryActions,
			Views:       []ViewType{ViewInboxProcessor},
			Aliases:     []string{"remove inbox task"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
			},
		},
		{
			Name:        "undo inbox action",
			Description: "Undo last inbox processing action",
			Shortcut:    "u",
			Category:    CategoryActions,
			Views:       []ViewType{ViewInboxProcessor},
			Aliases:     []string{"inbox undo", "revert inbox"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}}
			},
		},
	}
}

func (m *AppModel) dueDateOverviewCommands() []Command {
	return []Command{
		{
			Name:        "refresh due dates",
			Description: "Refresh due date overview data",
			Shortcut:    "r",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDueDateOverview},
			Aliases:     []string{"reload due dates"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
			},
		},
		{
			Name:        "go to task project",
			Description: "Navigate to task in project tree",
			Shortcut:    "gp",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewDueDateOverview},
			Aliases:     []string{"open in project", "show in tree"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
			},
		},
		{
			Name:        "copy task title",
			Description: "Copy task title to clipboard",
			Shortcut:    "y",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDueDateOverview},
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
			Views:       []ViewType{ViewDueDateOverview},
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
			Views:       []ViewType{ViewDueDateOverview},
			Aliases:     []string{"yank description"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyCtrlY}
			},
		},
	}
}
