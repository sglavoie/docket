package tui

import tea "charm.land/bubbletea/v2"

func (m *AppModel) filterCommands() []Command { //nolint:gocyclo // command registry closures encode view-specific filter actions.
	return []Command{
		{
			Name:        "filter by tag",
			Description: "Open tag filter selector",
			Shortcut:    "T",
			Category:    CategoryFilters,
			Views:       projectTreeAndOutlinerViews(),
			Handler: func() tea.Msg {
				if m.baseProjectView() == ViewCrossProjectOutliner && m.crossOutliner != nil {
					tags := BuildProjectsTagList(m.crossOutliner.loadedProjects, false)
					return ShowTagFilterMsg{Tags: tags, Selected: m.crossOutliner.filter.Tags, Source: ViewCrossProjectOutliner, Title: "Filter by Tags"}
				}
				if m.baseProjectView() == ViewOutliner && m.outliner != nil {
					tags := BuildProjectTagList(m.outliner.project, false)
					return ShowTagFilterMsg{Tags: tags, Selected: m.outliner.filter.Tags, Source: ViewOutliner, Title: "Filter by Tags"}
				}
				return nil
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
				return nil
			},
		},
		{
			Name:        "view completed",
			Description: "Switch to Completed view",
			Category:    CategoryFilters,
			Views:       projectTreeAndOutlinerViews(),
			Handler: func() tea.Msg {
				if _, params, ok := m.taskWorkspacePrimary(); ok && m.taskWorkspace != nil {
					params.Focus = WorkspaceCompleted
					params.SavedFilterID = ""
					if err := m.taskWorkspace.SetRouteParams(params); err != nil {
						return ErrorMsg{Err: err}
					}
					if err := m.taskWorkspace.Refresh(); err != nil {
						return ErrorMsg{Err: err}
					}
					return TaskWorkspaceRouteChangedMsg{Params: params}
				}
				if m.baseProjectView() == ViewOutliner && m.outliner != nil {
					if m.outliner.activePerspective == PerspectiveCompleted {
						m.outliner.activePerspective = PerspectiveSnoozed
					} else {
						m.outliner.activePerspective = PerspectiveCompleted
					}
					m.outliner.rebuildRows()
					return nil
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
					m.outliner.filterProjects = make(map[string]bool)
					m.outliner.ClearSearch()
					m.outliner.rebuildRows()
					return nil
				}
				return nil
			},
		},
		{
			Name:        "jump to task",
			Description: "Jump to task by ID or name",
			Shortcut:    "\\",
			Category:    CategorySearch,
			Views:       []PresentationID{ViewOutliner, ViewCrossProjectOutliner},
			Handler: func() tea.Msg {
				if m.baseProjectView() == ViewCrossProjectOutliner && m.crossOutliner != nil {
					return ShowTaskJumperMsg{Projects: m.crossOutliner.loadedProjects, CrossProject: true}
				}
				if m.currentProject != nil {
					return ShowTaskJumperMsg{Project: m.currentProject}
				}
				return nil
			},
		},
		{
			Name:        "tag manager",
			Description: "Manage project tags and pinned tag tabs",
			Shortcut:    "ctrl+t",
			Category:    CategoryActions,
			Views:       projectTreeAndOutlinerViews(),
			Handler: func() tea.Msg {
				if m.baseProjectView() == ViewOutliner && m.outliner != nil {
					return ShowTagManagerMsg{Project: m.outliner.project}
				}
				return nil
			},
		},
		{
			Name:        "global tag manager",
			Description: "Manage global pinned tag tabs",
			Shortcut:    "ctrl+t",
			Category:    CategoryActions,
			Views:       []PresentationID{ViewDashboard, ViewCrossProjectOutliner},
			Handler: func() tea.Msg {
				return ShowGlobalTagManagerMsg{}
			},
		},
		{
			Name:        "section manager",
			Description: "Browse built-in GTD sections",
			Shortcut:    "ctrl+s",
			Category:    CategoryActions,
			Views:       []PresentationID{ViewOutliner, ViewCrossProjectOutliner},
			Handler: func() tea.Msg {
				if m.baseProjectView() == ViewCrossProjectOutliner {
					if m.crossOutliner == nil {
						return StatusMsg("No task selected for section manager")
					}
					task, project := m.crossOutliner.getTaskRef()
					if task == nil || project == nil {
						return StatusMsg("No task selected for section manager")
					}
					return ShowSectionManagerMsg{Project: project}
				}
				if m.baseProjectView() == ViewOutliner && m.outliner != nil {
					return ShowSectionManagerMsg{Project: m.outliner.project}
				}
				return nil
			},
		},
		{
			Name:        "open data folder",
			Description: "Open root data folder in editor",
			Category:    CategoryActions,
			Views:       []PresentationID{ViewDashboard},
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
			Views:       []PresentationID{ViewDashboard},
			Handler: func() tea.Msg {
				if m.dashboard != nil {
					return m.dashboard.ActivateSearch()
				}
				return nil
			},
		},
	}
}
