package tui

import tea "charm.land/bubbletea/v2"

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
					if m.outliner.activePerspective == PerspectiveCompleted {
						m.outliner.activePerspective = PerspectiveSnoozed
					} else {
						m.outliner.activePerspective = PerspectiveCompleted
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