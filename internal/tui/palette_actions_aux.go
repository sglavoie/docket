package tui

import tea "charm.land/bubbletea/v2"

// getTagActions returns inline actions for a tag item.
func (m *CommandPaletteModel) getTagActions(item PaletteItem) []PaletteAction {
	if item.Tag == "" {
		return nil
	}
	tag := item.Tag
	project := m.findProjectWithTag(tag)

	return []PaletteAction{
		{
			Name:    "Filter by tag",
			Icon:    "🏷",
			KeyHint: "enter",
			Handler: func() tea.Msg {
				if project != nil {
					return GlobalTagNavigateMsg{Project: project, Tag: tag}
				}
				return nil
			},
		},
		{
			Name:    "Manage tags",
			Icon:    "⚙",
			KeyHint: "t",
			Handler: func() tea.Msg {
				return ShowGlobalTagManagerMsg{}
			},
		},
	}
}

// getSectionActions returns inline actions for a section item.
func (m *CommandPaletteModel) getSectionActions(item PaletteItem) []PaletteAction {
	if item.Section == "" {
		return nil
	}
	section := item.Section
	project := m.findProjectWithSection(section)

	return []PaletteAction{
		{
			Name:    "Filter by section",
			Icon:    "📂",
			KeyHint: "enter",
			Handler: func() tea.Msg {
				if project != nil {
					return CommandPaletteSectionSelectMsg{Section: section, Project: project}
				}
				return nil
			},
		},
	}
}

// getNoteActions returns inline actions for a note item.
func (m *CommandPaletteModel) getNoteActions(item PaletteItem) []PaletteAction {
	if item.Note == nil || item.Project == nil {
		return nil
	}
	project := item.Project

	return []PaletteAction{
		{
			Name:    "Open notes view",
			Icon:    "→",
			KeyHint: "enter",
			Handler: func() tea.Msg {
				return ShowNotesMsg{Project: project}
			},
		},
	}
}

// getViewActions returns inline actions for a view item.
func (m *CommandPaletteModel) getViewActions(item PaletteItem) []PaletteAction {
	viewType := item.ViewTarget
	return []PaletteAction{
		{
			Name:    "Navigate",
			Icon:    "→",
			KeyHint: "enter",
			Handler: func() tea.Msg {
				return NavigateToViewMsg{ViewType: viewType}
			},
		},
	}
}

// getSavedSearchActions returns inline actions for a saved search item.
func (m *CommandPaletteModel) getSavedSearchActions(item PaletteItem) []PaletteAction {
	if item.SavedSearch == "" {
		return nil
	}
	name := item.Name
	query := item.SavedSearch

	return []PaletteAction{
		{
			Name:    "Run search",
			Icon:    "🔍",
			KeyHint: "enter",
			Handler: func() tea.Msg {
				return ExecuteSavedSearchMsg{Query: query}
			},
		},
		{
			Name:    "Delete",
			Icon:    "✕",
			KeyHint: "d",
			Handler: func() tea.Msg {
				return DeleteSavedSearchMsg{Name: name}
			},
		},
	}
}

// getSavedFilterActions returns inline actions for a saved filter item.
func (m *CommandPaletteModel) getSavedFilterActions(item PaletteItem) []PaletteAction {
	if item.SavedFilter == nil {
		return nil
	}
	sf := *item.SavedFilter

	actions := []PaletteAction{
		{
			Name:    "Apply",
			Icon:    "→",
			KeyHint: "enter",
			Handler: func() tea.Msg {
				return LoadSavedFilterMsg{Filter: sf}
			},
		},
		{
			Name:    "Overwrite with current",
			Icon:    "✏",
			KeyHint: "o",
			Handler: func() tea.Msg {
				return SaveFilterMsg{Name: sf.Name}
			},
		},
	}

	tabName := "Show as tab"
	tabIcon := "☐"
	if sf.ShowAsTab {
		tabName = "Hide from tabs"
		tabIcon = "☑"
	}
	filterID := sf.ID
	currentTab := sf.ShowAsTab
	actions = append(actions,
		PaletteAction{
			Name:    tabName,
			Icon:    tabIcon,
			KeyHint: "t",
			Handler: func() tea.Msg {
				return toggleSavedFilterTabMsg{ID: filterID, ShowAsTab: !currentTab}
			},
		},
		PaletteAction{
			Name:    "Delete",
			Icon:    "✕",
			KeyHint: "d",
			Handler: func() tea.Msg {
				return DeleteSavedFilterMsg{ID: sf.ID}
			},
		},
	)

	return actions
}
