package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/icons"
)

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
			Icon:    icons.Get("tag"),
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
			Icon:    icons.Get("settings"),
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
			Icon:    icons.Get("folder"),
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
				return NavigateToViewMsg{PresentationID: viewType}
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

	actions = append(actions,
		PaletteAction{
			Name:    "Delete",
			Icon:    icons.Get("close"),
			KeyHint: "d",
			Handler: func() tea.Msg {
				return DeleteSavedFilterMsg{ID: sf.ID}
			},
		},
	)

	return actions
}
