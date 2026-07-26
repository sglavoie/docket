package tui

import tea "charm.land/bubbletea/v2"

func (m *AppModel) filterCommands() []Command { //nolint:gocyclo // command registry closures encode view-specific filter actions.
	// The cross Outliner is the sole per-project surface (project selection is a
	// scope frame), so per-project filter commands must be reachable from it in
	// both global and project scopes as well as the legacy single-project view.
	outlinerFilterViews := []PresentationID{ViewOutliner, ViewCrossProjectOutliner}
	return []Command{
		{
			Name:        "filter by tag",
			Description: "Open tag filter selector",
			Shortcut:    "T",
			Category:    CategoryFilters,
			Views:       outlinerFilterViews,
			Handler: func() tea.Msg {
				if m.baseProjectView() == ViewCrossProjectOutliner && m.crossOutliner != nil {
					return m.crossOutliner.scopedTagFilterMsg()
				}
				if m.baseProjectView() == ViewOutliner && m.outliner != nil {
					return m.outliner.scopedTagFilterMsg()
				}
				return nil
			},
		},
		{
			Name:        "filter by section",
			Description: "Open section filter selector",
			Shortcut:    "N",
			Category:    CategoryFilters,
			Views:       outlinerFilterViews,
			Handler: func() tea.Msg {
				if m.baseProjectView() == ViewCrossProjectOutliner && m.crossOutliner != nil {
					if p := m.crossOutliner.scopedProject(); p != nil {
						return ShowSectionFilterSelectorMsg{Project: p}
					}
					return ShowSectionFilterSelectorMsg{Projects: m.crossOutliner.scopeProjects()}
				}
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
			Views:       outlinerFilterViews,
			Handler: func() tea.Msg {
				if outliner, _, ok := m.outlinerPrimary(); ok {
					target := ModeComplete
					if outliner.smartFocus == nil && outliner.activePerspective == PerspectiveCompleted {
						target = ModeSnoozed
					}
					outliner.selectOutlinerMode(target)
					return nil
				}
				return nil
			},
		},
		{
			Name:        "toggle high priority",
			Description: "Filter high priority tasks only",
			Category:    CategoryFilters,
			Views:       outlinerFilterViews,
			Handler: func() tea.Msg {
				if out := m.visibleOutliner(); out != nil {
					if out.filter.Priority == 0 {
						out.filter.Priority = 1
					} else {
						out.filter.Priority = 0
					}
					out.crossForceRebuild = true
					out.rebuildRows()
				}
				return nil
			},
		},
		{
			Name:        "clear filters",
			Description: "Reset all task filters",
			Shortcut:    "Fx",
			Category:    CategoryFilters,
			Views:       outlinerFilterViews,
			Handler: func() tea.Msg {
				if out := m.visibleOutliner(); out != nil {
					clearCommonFilters(&out.filter.CommonFilterState)
					out.filter.Actionable = false
					out.ClearSearch()
					out.crossForceRebuild = true
					out.rebuildRows()
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
					return ShowTaskJumperMsg{Projects: m.crossOutliner.scopeProjects(), CrossProject: true}
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
			Views:       outlinerFilterViews,
			Handler: func() tea.Msg {
				project := m.paletteCursorProject()
				if project == nil {
					if m.nonPaletteOrigin() == ViewCrossProjectOutliner {
						return StatusMsg(statusNoProjectUnderCursor)
					}
					return nil
				}
				return ShowTagManagerMsg{Project: project}
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
				project := m.paletteCursorProject()
				if project == nil {
					if m.nonPaletteOrigin() == ViewCrossProjectOutliner {
						return StatusMsg(statusNoProjectUnderCursor)
					}
					return nil
				}
				return ShowSectionManagerMsg{Project: project}
			},
		},
		{
			Name:        "open data folder",
			Description: "Open root data folder in editor",
			Category:    CategoryActions,
			Views:       []PresentationID{ViewCrossProjectOutliner},
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
			Views:       outlinerFilterViews,
			Aliases:     []string{"open folder", "reveal data", "data directory", "open data folder"},
			Handler: func() tea.Msg {
				project := m.paletteCursorProject()
				if project == nil {
					if m.nonPaletteOrigin() == ViewCrossProjectOutliner {
						return StatusMsg(statusNoProjectUnderCursor)
					}
					return nil
				}
				return OpenDataFolderMsg{Project: project}
			},
		},
	}
}
