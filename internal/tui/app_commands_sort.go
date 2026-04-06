package tui

import tea "charm.land/bubbletea/v2"

func (m *AppModel) sortCommands() []Command {
	return []Command{
		{
			Name:        "sort by priority",
			Description: "Sort tasks by priority",
			Shortcut:    "1",
			Category:    CategorySort,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"priority sort"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.toggleSort(SortByPriority)
				}
				return nil
			},
		},
		{
			Name:        "sort by due date",
			Description: "Sort tasks by due date",
			Shortcut:    "2",
			Category:    CategorySort,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"due date sort", "deadline sort"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.toggleSort(SortByDueDate)
				}
				return nil
			},
		},
		{
			Name:        "sort by created",
			Description: "Sort tasks by creation date",
			Shortcut:    "3",
			Category:    CategorySort,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"created sort", "creation date sort"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.toggleSort(SortByCreatedAt)
				}
				return nil
			},
		},
		{
			Name:        "sort by status",
			Description: "Sort tasks by status",
			Shortcut:    "4",
			Category:    CategorySort,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"status sort"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.toggleSort(SortByStatus)
				}
				return nil
			},
		},
		{
			Name:        "sort by score",
			Description: "Sort tasks by composite score",
			Shortcut:    "5",
			Category:    CategorySort,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"score sort", "scoring sort"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.toggleSort(SortByScore)
				}
				return nil
			},
		},
		{
			Name:        "reset sort",
			Description: "Reset sort to default order",
			Shortcut:    "0",
			Category:    CategorySort,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"default sort", "clear sort"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.sort.Reset()
					m.projectTree.buildTree()
				}
				return nil
			},
		},
	}
}
