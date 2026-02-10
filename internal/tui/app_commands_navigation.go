package tui

import tea "github.com/charmbracelet/bubbletea"

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
			Shortcut:    "/",
			Category:    CategorySearch,
			Aliases:     []string{"find tasks", "search all"},
			Handler: func() tea.Msg {
				return ShowGlobalSearchMsg{}
			},
		},
		{
			Name:        "settings",
			Description: "Open application settings",
			Shortcut:    ",",
			Category:    CategoryNavigation,
			Aliases:     []string{"preferences", "config", "options"},
			Handler: func() tea.Msg {
				return SettingsMsg{}
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
			Shortcut:    "ctrl+W",
			Category:    CategoryNavigation,
			Aliases:     []string{"automation", "rules", "workflow"},
			Views:       []ViewType{ViewDashboard},
			Handler: func() tea.Msg {
				return ShowWorkflowRulesMsg{}
			},
		},
		{
			Name:        "create workflow rule",
			Description: "Create a new workflow automation rule",
			Category:    CategoryActions,
			Aliases:     []string{"new rule", "add rule", "new automation"},
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

func (m *AppModel) filterCommands() []Command {
	return []Command{
		{
			Name:        "filter by tag",
			Description: "Open tag filter selector",
			Shortcut:    "T",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				tags := buildTagList(m.projectTree)
				return ShowTagFilterMsg{Tags: tags, Selected: m.projectTree.filter.Tags, Source: ViewProjectTree, Title: "Filter by Tags"}
			},
		},
		{
			Name:        "filter by section",
			Description: "Open section filter selector",
			Shortcut:    "N",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Handler: func() tea.Msg {
				return ShowSectionFilterSelectorMsg{ProjectTree: m.projectTree}
			},
		},
		{
			Name:        "view completed",
			Description: "Switch to Completed view",
			Shortcut:    "C",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.viewMode = ViewModeCompleted
					m.projectTree.updateVisibility()
				}
				return nil
			},
		},
		{
			Name:        "toggle high priority",
			Description: "Filter high priority tasks only",
			Shortcut:    "P",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					if m.projectTree.filter.Priority == 0 {
						m.projectTree.filter.Priority = 1
					} else {
						m.projectTree.filter.Priority = 0
					}
					m.projectTree.updateVisibility()
				}
				return nil
			},
		},
		{
			Name:        "clear filters",
			Description: "Reset all task filters",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.filter.Type = ""
					m.projectTree.filter.Status = ""
					m.projectTree.filter.Priority = 0
					m.projectTree.filter.Tags = make(map[string]bool)
					m.projectTree.filter.Sections = make(map[string]bool)
					m.projectTree.filter.DueDate = 0
					m.projectTree.updateVisibility()
				}
				return nil
			},
		},
		{
			Name:        "jump to task",
			Description: "Jump to task by ID or name",
			Shortcut:    "\\",
			Category:    CategorySearch,
			Views:       []ViewType{ViewProjectTree},
			Handler: func() tea.Msg {
				if m.currentProject != nil {
					return ShowTaskJumperMsg{Project: m.currentProject}
				}
				return nil
			},
		},
		{
			Name:        "tag manager",
			Description: "Manage project tags",
			Shortcut:    "ctrl+t",
			Category:    CategoryActions,
			Views:       []ViewType{ViewProjectTree},
			Handler: func() tea.Msg {
				return ShowTagManagerMsg{ProjectTree: m.projectTree}
			},
		},
		{
			Name:        "global tag manager",
			Description: "Manage tags across all projects",
			Shortcut:    "ctrl+t",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDashboard},
			Handler: func() tea.Msg {
				return ShowGlobalTagManagerMsg{}
			},
		},
		{
			Name:        "custom section manager",
			Description: "Manage custom project sections",
			Shortcut:    "ctrl+s",
			Category:    CategoryActions,
			Views:       []ViewType{ViewProjectTree},
			Handler: func() tea.Msg {
				return ShowSectionManagerMsg{ProjectTree: m.projectTree}
			},
		},
		{
			Name:        "add section",
			Description: "Add a new section to project",
			Category:    CategoryActions,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"create section", "new section"},
			Handler: func() tea.Msg {
				return ShowSectionManagerMsg{ProjectTree: m.projectTree, StartAddMode: true}
			},
		},
		{
			Name:        "open data folder",
			Description: "Open root data folder in editor",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDashboard},
			Aliases:     []string{"open folder", "reveal data", "data directory"},
			Handler: func() tea.Msg {
				return OpenDataFolderMsg{Project: nil}
			},
		},
		{
			Name:        "open project folder",
			Description: "Open project data folder in editor",
			Category:    CategoryActions,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"open folder", "reveal data", "data directory", "open data folder"},
			Handler: func() tea.Msg {
				return OpenDataFolderMsg{Project: m.currentProject}
			},
		},
		{
			Name:        "search projects",
			Description: "Filter projects by name",
			Shortcut:    "\\",
			Category:    CategorySearch,
			Views:       []ViewType{ViewDashboard},
			Handler: func() tea.Msg {
				if m.dashboard != nil {
					return m.dashboard.ActivateSearch()
				}
				return nil
			},
		},
	}
}

func (m *AppModel) viewCommands() []Command {
	return []Command{
		{
			Name:        "all tasks",
			Description: "View all tasks across every project",
			Shortcut:    "A",
			Category:    CategoryViews,
			Views:       []ViewType{ViewDashboard, ViewProjectTree},
			Aliases:     []string{"cross-project tasks", "every task"},
			Handler: func() tea.Msg {
				return ShowAllTasksMsg{}
			},
		},
		{
			Name:        "due date overview",
			Description: "View tasks by due date",
			Shortcut:    "ctrl+d",
			Category:    CategoryViews,
			Views:       []ViewType{ViewProjectTree, ViewDashboard},
			Handler: func() tea.Msg {
				return ShowDueDateOverviewMsg{}
			},
		},
		{
			Name:        "daily review",
			Description: "Daily planning and review view",
			Shortcut:    "r",
			Category:    CategoryViews,
			Views:       []ViewType{ViewDashboard},
			Aliases:     []string{"planning", "today", "morning review"},
			Handler: func() tea.Msg {
				return ShowDailyReviewMsg{}
			},
		},
		{
			Name:        "latest activity",
			Description: "View recently modified tasks across all projects",
			Shortcut:    "l",
			Category:    CategoryViews,
			Views:       []ViewType{ViewDashboard},
			Aliases:     []string{"recent", "activity", "latest", "recent tasks"},
			Handler: func() tea.Msg {
				return ShowRecentActivityMsg{}
			},
		},
		{
			Name:        "focus view",
			Description: "View all active-flagged tasks across projects",
			Shortcut:    "f",
			Category:    CategoryViews,
			Views:       []ViewType{ViewDashboard},
			Aliases:     []string{"active tasks", "focus", "today tasks"},
			Handler: func() tea.Msg {
				return ShowFocusViewMsg{}
			},
		},
		{
			Name:        "priority view",
			Description: "View tasks by priority",
			Shortcut:    "p",
			Category:    CategoryViews,
			Views:       []ViewType{ViewProjectTree},
			Handler: func() tea.Msg {
				if m.currentProject != nil {
					return ShowPriorityViewMsg{Project: m.currentProject}
				}
				return nil
			},
		},
		{
			Name:        "inbox processor",
			Description: "Process inbox items one by one",
			Shortcut:    "w",
			Category:    CategoryViews,
			Views:       []ViewType{ViewProjectTree},
			Handler: func() tea.Msg {
				if m.currentProject != nil {
					return ShowInboxProcessorMsg{Project: m.currentProject}
				}
				return nil
			},
		},
		{
			Name:        "kanban board",
			Description: "View tasks in Kanban board layout",
			Shortcut:    "ctrl+k",
			Category:    CategoryViews,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"kanban", "board", "columns"},
			Handler: func() tea.Msg {
				if m.currentProject != nil {
					return ShowKanbanMsg{Project: m.currentProject}
				}
				return nil
			},
		},
		{
			Name:        "global inbox processor",
			Description: "Process inbox items across all projects",
			Shortcut:    "w",
			Category:    CategoryViews,
			Views:       []ViewType{ViewDashboard},
			Aliases:     []string{"global inbox", "all inbox", "process all inbox"},
			Handler: func() tea.Msg {
				return ShowGlobalInboxProcessorMsg{}
			},
		},
		{
			Name:        "global notes",
			Description: "Open global notes (not tied to any project)",
			Shortcut:    "N",
			Category:    CategoryViews,
			Views:       []ViewType{ViewDashboard},
			Aliases:     []string{"global notes", "notes", "scratch notes"},
			Handler: func() tea.Msg {
				return ShowGlobalNotesMsg{}
			},
		},
		{
			Name:        "global blockers",
			Description: "View blockers across all projects",
			Shortcut:    "B",
			Category:    CategoryViews,
			Views:       []ViewType{ViewDashboard},
			Aliases:     []string{"all blockers", "blockers overview"},
			Handler: func() tea.Msg {
				return ShowGlobalBlockersMsg{}
			},
		},
		{
			Name:        "metrics dashboard",
			Description: "View completion rates, cycle time, and throughput",
			Shortcut:    "m",
			Category:    CategoryViews,
			Views:       []ViewType{ViewDashboard},
			Aliases:     []string{"metrics", "analytics", "stats", "statistics"},
			Handler: func() tea.Msg {
				return ShowMetricsDashboardMsg{}
			},
		},
		{
			Name:        "project metrics",
			Description: "View metrics for current project",
			Category:    CategoryViews,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"metrics", "analytics", "stats", "statistics"},
			Handler: func() tea.Msg {
				if m.currentProject != nil {
					return ShowMetricsDashboardMsg{Project: m.currentProject}
				}
				return nil
			},
		},
		{
			Name:        "undo history",
			Description: "View and restore from undo history",
			Shortcut:    "ctrl+z",
			Category:    CategoryViews,
			Views:       []ViewType{ViewProjectTree},
			Handler: func() tea.Msg {
				if m.currentProject != nil {
					return ShowUndoHistoryMsg{Project: m.currentProject}
				}
				return nil
			},
		},
		{
			Name:        "filter popup",
			Description: "Open the filter popup",
			Shortcut:    "F",
			Category:    CategoryViews,
			Views:       []ViewType{ViewProjectTree},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.pendingPrefix = "F"
				}
				return nil
			},
		},
	}
}

func (m *AppModel) navCommands() []Command {
	return []Command{
		{
			Name:        "dashboard",
			Description: "Go to dashboard",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewProjectTree, ViewProjectList},
			Handler: func() tea.Msg {
				return BackMsg{}
			},
		},
		{
			Name:        "project cycle",
			Description: "Configure project cycle, jump to project",
			Shortcut:    "@",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewDashboard, ViewProjectTree},
			Handler: func() tea.Msg {
				return ShowProjectCycleManagerMsg{}
			},
		},
		{
			Name:        "next project",
			Description: "Switch to next project in cycle",
			Shortcut:    "`",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewDashboard, ViewProjectTree},
			Handler: func() tea.Msg {
				return CycleProjectMsg{Forward: true}
			},
		},
		{
			Name:        "previous project",
			Description: "Switch to previous project in cycle",
			Shortcut:    "~",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewDashboard, ViewProjectTree},
			Handler: func() tea.Msg {
				return CycleProjectMsg{Forward: false}
			},
		},
		{
			Name:        "history back",
			Description: "Navigate back through view history",
			Shortcut:    "alt+O",
			Category:    CategoryNavigation,
			Aliases:     []string{"go back history", "previous view", "back history"},
			Handler: func() tea.Msg {
				m.navigateHistoryBack()
				return nil
			},
		},
		{
			Name:        "history forward",
			Description: "Navigate forward through view history",
			Shortcut:    "alt+I",
			Category:    CategoryNavigation,
			Aliases:     []string{"go forward history", "next view", "forward history"},
			Handler: func() tea.Msg {
				m.navigateHistoryForward()
				return nil
			},
		},
	}
}
