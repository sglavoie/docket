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
			Shortcut:    ";/",
			Category:    CategorySearch,
			Aliases:     []string{"find tasks", "search all"},
			Handler: func() tea.Msg {
				return ShowGlobalSearchMsg{}
			},
		},
		{
			Name:        "settings",
			Description: "Open application settings",
			Shortcut:    ";s",
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
			Views:       projectTreeAndOutlinerViews(),
			Handler: func() tea.Msg {
				if m.baseProjectView() == ViewOutliner && m.outliner != nil {
					tags := BuildTagList(m.outliner.project.Tasks, false)
					return ShowTagFilterMsg{Tags: tags, Selected: m.outliner.filter.Tags, Source: ViewOutliner, Title: "Filter by Tags"}
				}
				if m.projectTree == nil {
					return nil
				}
				tags := BuildTagList(m.projectTree.project.Tasks, false)
				return ShowTagFilterMsg{Tags: tags, Selected: m.projectTree.filter.Tags, Source: ViewProjectTree, Title: "Filter by Tags"}
			},
		},
		{
			Name:        "filter by section",
			Description: "Open section filter selector",
			Shortcut:    "N",
			Category:    CategoryFilters,
			Views:       projectTreeAndOutlinerViews(),
			Handler: func() tea.Msg {
				if m.baseProjectView() == ViewOutliner && m.outliner != nil {
					return ShowSectionFilterSelectorMsg{Project: m.outliner.project}
				}
				return ShowSectionFilterSelectorMsg{ProjectTree: m.projectTree}
			},
		},
		{
			Name:        "view completed",
			Description: "Switch to Completed view",
			Category:    CategoryFilters,
			Views:       projectTreeAndOutlinerViews(),
			Handler: func() tea.Msg {
				if m.baseProjectView() == ViewOutliner && m.outliner != nil {
					if m.outliner.activePerspective == 8 {
						m.outliner.activePerspective = 0
					} else {
						m.outliner.activePerspective = 8
					}
					m.outliner.rebuildRows()
					return nil
				}
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
			Category:    CategoryFilters,
			Views:       projectTreeAndOutlinerViews(),
			Handler: func() tea.Msg {
				if m.baseProjectView() == ViewOutliner && m.outliner != nil {
					if m.outliner.filter.Priority == 0 {
						m.outliner.filter.Priority = 1
					} else {
						m.outliner.filter.Priority = 0
					}
					m.outliner.rebuildRows()
					return nil
				}
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
			Shortcut:    "Fx",
			Category:    CategoryFilters,
			Views:       projectTreeAndOutlinerViews(),
			Handler: func() tea.Msg {
				if m.baseProjectView() == ViewOutliner && m.outliner != nil {
					clearCommonFilters(&m.outliner.filter.CommonFilterState)
					m.outliner.filter.Sections = make(map[string]bool)
					m.outliner.filter.Actionable = false
					m.outliner.ClearSearch()
					m.outliner.rebuildRows()
					return nil
				}
				if m.projectTree != nil {
					clearCommonFilters(&m.projectTree.filter.CommonFilterState)
					m.projectTree.filter.Sections = make(map[string]bool)
					m.projectTree.filter.Actionable = false
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
			Views:       projectTreeAndOutlinerViews(),
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
			Views:       projectTreeAndOutlinerViews(),
			Handler: func() tea.Msg {
				if m.baseProjectView() == ViewOutliner && m.outliner != nil {
					return ShowTagManagerMsg{Project: m.outliner.project}
				}
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
			Views:       projectTreeAndOutlinerViews(),
			Handler: func() tea.Msg {
				if m.baseProjectView() == ViewOutliner && m.outliner != nil {
					return ShowSectionManagerMsg{Project: m.outliner.project}
				}
				return ShowSectionManagerMsg{ProjectTree: m.projectTree}
			},
		},
		{
			Name:        "add section",
			Description: "Add a new section to project",
			Category:    CategoryActions,
			Views:       projectTreeAndOutlinerViews(),
			Aliases:     []string{"create section", "new section"},
			Handler: func() tea.Msg {
				if m.baseProjectView() == ViewOutliner && m.outliner != nil {
					return ShowSectionManagerMsg{Project: m.outliner.project, StartAddMode: true}
				}
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
			Name:        "open log directory",
			Description: "Open log directory in Finder",
			Category:    CategoryActions,
			Aliases:     []string{"logs", "log files", "open logs"},
			Handler: func() tea.Msg {
				return OpenLogDirMsg{}
			},
		},
		{
			Name:        "open project folder",
			Description: "Open project data folder in editor",
			Category:    CategoryActions,
			Views:       projectTreeAndOutlinerViews(),
			Aliases:     []string{"open folder", "reveal data", "data directory", "open data folder"},
			Handler: func() tea.Msg {
				return OpenDataFolderMsg{Project: m.currentProject}
			},
		},
		{
			Name:        "search projects",
			Description: "Filter projects by name",
			Shortcut:    "/",
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
			Shortcut:    ";a",
			Category:    CategoryViews,
			Views:       []ViewType{ViewProjectTree, ViewOutliner},
			Aliases:     []string{"cross-project tasks", "every task"},
			Handler: func() tea.Msg {
				return ShowAllTasksMsg{}
			},
		},
		{
			Name:        "due date overview",
			Description: "View tasks by due date",
			Shortcut:    ";D",
			Category:    CategoryViews,
			Views:       []ViewType{ViewProjectTree, ViewDashboard, ViewOutliner},
			Handler: func() tea.Msg {
				return ShowDueDateOverviewMsg{}
			},
		},
		{
			Name:        "daily review",
			Description: "Daily planning and review view",
			Shortcut:    ";d",
			Category:    CategoryViews,
			Aliases:     []string{"planning", "today", "morning review"},
			Handler: func() tea.Msg {
				return ShowDailyReviewMsg{}
			},
		},
		{
			Name:        "weekly review",
			Description: "Strategic project health review",
			Shortcut:    ";w",
			Category:    CategoryViews,
			Aliases:     []string{"weekly", "project health", "strategic review"},
			Handler: func() tea.Msg {
				return ShowWeeklyReviewMsg{}
			},
		},
		{
			Name:        "latest activity",
			Description: "View recently modified tasks across all projects",
			Shortcut:    ";l",
			Category:    CategoryViews,
			Aliases:     []string{"recent", "activity", "latest", "recent tasks"},
			Handler: func() tea.Msg {
				return ShowRecentActivityMsg{}
			},
		},
		{
			Name:        "what's next",
			Description: "Cross-project task suggestions and work queue",
			Shortcut:    ";f",
			Category:    CategoryViews,
			Aliases:     []string{"active tasks", "focus", "today tasks", "focus view", "suggestions"},
			Handler: func() tea.Msg {
				return ShowFocusViewMsg{}
			},
		},
		{
			Name:        "focus board",
			Description: "Personal command center with external items and tasks",
			Shortcut:    ";F",
			Category:    CategoryViews,
			Aliases:     []string{"board", "command center", "external items"},
			Handler: func() tea.Msg {
				return ShowFocusBoardMsg{}
			},
		},
		{
			Name:        "priority view",
			Description: "View tasks by priority",
			Shortcut:    "p",
			Category:    CategoryViews,
			Views:       projectTreeAndOutlinerViews(),
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
			Views:       projectTreeAndOutlinerViews(),
			Handler: func() tea.Msg {
				if m.currentProject != nil {
					return ShowInboxProcessorMsg{Project: m.currentProject}
				}
				return nil
			},
		},
		{
			Name:        "global inbox processor",
			Description: "Process inbox items across all projects",
			Shortcut:    ";i",
			Category:    CategoryViews,
			Aliases:     []string{"global inbox", "all inbox", "process all inbox"},
			Handler: func() tea.Msg {
				return ShowGlobalInboxProcessorMsg{}
			},
		},
		{
			Name:        "global notes",
			Description: "Open global notes (not tied to any project)",
			Shortcut:    ";N",
			Category:    CategoryViews,
			Aliases:     []string{"global notes", "notes", "scratch notes"},
			Handler: func() tea.Msg {
				return ShowGlobalNotesMsg{}
			},
		},
		{
			Name:        "global blockers",
			Description: "View blockers across all projects",
			Shortcut:    ";B",
			Category:    CategoryViews,
			Aliases:     []string{"all blockers", "blockers overview"},
			Handler: func() tea.Msg {
				return ShowGlobalBlockersMsg{}
			},
		},
		{
			Name:        "metrics dashboard",
			Description: "View completion rates, cycle time, and throughput",
			Shortcut:    ";m",
			Category:    CategoryViews,
			Aliases:     []string{"metrics", "analytics", "stats", "statistics"},
			Handler: func() tea.Msg {
				return ShowMetricsDashboardMsg{}
			},
		},
		{
			Name:        "project metrics",
			Description: "View metrics for current project",
			Category:    CategoryViews,
			Views:       projectTreeAndOutlinerViews(),
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
			Views:       projectTreeAndOutlinerViews(),
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
			Views:       []ViewType{ViewProjectTree, ViewProjectList, ViewOutliner},
			Handler: func() tea.Msg {
				return BackMsg{}
			},
		},
		{
			Name:        "project cycle",
			Description: "Configure project cycle, jump to project",
			Shortcut:    "@",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewDashboard, ViewProjectTree, ViewOutliner},
			Handler: func() tea.Msg {
				return ShowProjectCycleManagerMsg{}
			},
		},
		{
			Name:        "next project",
			Description: "Switch to next project in cycle",
			Shortcut:    "`",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewDashboard, ViewProjectTree, ViewOutliner},
			Handler: func() tea.Msg {
				return CycleProjectMsg{Forward: true}
			},
		},
		{
			Name:        "previous project",
			Description: "Switch to previous project in cycle",
			Shortcut:    "~",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewDashboard, ViewProjectTree, ViewOutliner},
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
		{
			Name:        "recent tasks",
			Description: "Quick-switch between recently visited tasks",
			Shortcut:    "|",
			Category:    CategoryNavigation,
			Aliases:     []string{"quick switch", "task switcher", "recent visits"},
			Handler: func() tea.Msg {
				m.openQuickSwitch()
				return nil
			},
		},
	}
}
