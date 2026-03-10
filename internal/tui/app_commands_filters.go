package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

func (m *AppModel) filterBarCommands() []Command {
	return []Command{
		{
			Name:        "save current view",
			Description: "Save filters/sort/view-mode as a named view preset",
			Shortcut:    "Fvs",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"save filter", "saved filter", "save view", "view preset"},
			Handler: func() tea.Msg {
				return ShowSaveFilterDialogMsg{}
			},
		},
		{
			Name:        "filter by type",
			Description: "Cycle through type filter options",
			Shortcut:    "t",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"type filter", "cycle type"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.filter.Type = cycleFilter(m.projectTree.filter.Type, models.AllTaskTypes())
					m.projectTree.updateVisibility()
					m.projectTree.pruneInvisibleSelections()
					m.projectTree.clampCursor()
				}
				return nil
			},
		},
		{
			Name:        "filter by status",
			Description: "Cycle through status filter options",
			Shortcut:    "Fs",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"status filter", "cycle status"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.filter.Status = cycleFilter(m.projectTree.filter.Status, models.AllTaskStatuses())
					m.projectTree.updateVisibility()
					m.projectTree.pruneInvisibleSelections()
					m.projectTree.clampCursor()
				}
				return nil
			},
		},
		{
			Name:        "completed view toggle",
			Description: "Toggle Completed view (F1)",
			Shortcut:    "F1",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"f1 filter", "show completed"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					if m.projectTree.viewMode == ViewModeCompleted {
						m.projectTree.viewMode = ViewModeActive
					} else {
						m.projectTree.viewMode = ViewModeCompleted
					}
					m.projectTree.updateVisibility()
					m.projectTree.pruneInvisibleSelections()
					m.projectTree.clampCursor()
				}
				return nil
			},
		},
		{
			Name:        "high priority filter",
			Description: "Toggle high priority filter (F2)",
			Shortcut:    "F2",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"f2 filter", "priority filter toggle"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					if m.projectTree.filter.Priority == 0 {
						m.projectTree.filter.Priority = 1
					} else {
						m.projectTree.filter.Priority = 0
					}
					m.projectTree.updateVisibility()
					m.projectTree.pruneInvisibleSelections()
					m.projectTree.clampCursor()
				}
				return nil
			},
		},
		{
			Name:        "type filter cycle",
			Description: "Cycle type filter (F3)",
			Shortcut:    "F3",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"f3 filter"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.filter.Type = cycleFilter(m.projectTree.filter.Type, models.AllTaskTypes())
					m.projectTree.updateVisibility()
					m.projectTree.pruneInvisibleSelections()
					m.projectTree.clampCursor()
				}
				return nil
			},
		},
		{
			Name:        "status filter cycle",
			Description: "Cycle status filter (F4)",
			Shortcut:    "F4",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"f4 filter"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.filter.Status = cycleFilter(m.projectTree.filter.Status, models.AllTaskStatuses())
					m.projectTree.updateVisibility()
					m.projectTree.pruneInvisibleSelections()
					m.projectTree.clampCursor()
				}
				return nil
			},
		},
		{
			Name:        "tag filter selector",
			Description: "Open tag filter (F5)",
			Shortcut:    "F5",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"f5 filter"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					tags := BuildTagList(m.projectTree.project.Tasks, false)
					return ShowTagFilterMsg{Tags: tags, Selected: m.projectTree.filter.Tags, Source: ViewProjectTree, Title: "Filter by Tags"}
				}
				return nil
			},
		},
		{
			Name:        "section filter selector",
			Description: "Open section filter (F6)",
			Shortcut:    "F6",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"f6 filter"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					return ShowSectionFilterSelectorMsg{ProjectTree: m.projectTree}
				}
				return nil
			},
		},
		{
			Name:        "due date filter cycle",
			Description: "Cycle due date filter (F7)",
			Shortcut:    "F7",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"f7 filter"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.filter.DueDate = (m.projectTree.filter.DueDate + 1) % 5
					m.projectTree.updateVisibility()
					m.projectTree.pruneInvisibleSelections()
					m.projectTree.clampCursor()
				}
				return nil
			},
		},
		{
			Name:        "actionable filter",
			Description: "Toggle actionable filter (F8)",
			Shortcut:    "F8",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"f8 filter", "actionable only"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.filter.Actionable = !m.projectTree.filter.Actionable
					m.projectTree.updateVisibility()
					m.projectTree.pruneInvisibleSelections()
					m.projectTree.clampCursor()
				}
				return nil
			},
		},
	}
}
