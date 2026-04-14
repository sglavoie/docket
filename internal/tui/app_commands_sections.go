package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *AppModel) sectionSendCommands() []Command {
	return []Command{
		{
			Name:        "send to active",
			Description: "Move task to Active section",
			Shortcut:    "gA",
			Category:    CategoryActions,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"active section", "move to active"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				m.projectTree.sendTasksToSection("", visible)
				return nil
			},
		},
		{
			Name:        "send to next",
			Description: "Move task to Next section",
			Shortcut:    "gN",
			Category:    CategoryActions,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"next section", "move to next"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				m.projectTree.sendTasksToSection(models.SectionNext, visible)
				return nil
			},
		},
		{
			Name:        "send to inbox",
			Description: "Move task to Inbox section",
			Shortcut:    "gI",
			Category:    CategoryActions,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"inbox section", "move to inbox"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				m.projectTree.sendTasksToSection(models.SectionInbox, visible)
				return nil
			},
		},
		{
			Name:        "send to waiting",
			Description: "Move task to Waiting section",
			Shortcut:    "gW",
			Category:    CategoryActions,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"waiting section", "move to waiting"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				m.projectTree.sendTasksToSection(models.SectionWaiting, visible)
				return nil
			},
		},
		{
			Name:        "send to someday",
			Description: "Move task to Someday section",
			Shortcut:    "gS",
			Category:    CategoryActions,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"someday section", "move to someday"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				m.projectTree.sendTasksToSection(models.SectionSomeday, visible)
				return nil
			},
		},
		{
			Name:        "completed view",
			Description: "Go to Completed view mode",
			Shortcut:    "gc",
			Category:    CategoryViews,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"show completed", "view completed tasks"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.viewMode = ViewModeCompleted
					m.projectTree.updateVisibility()
					m.projectTree.pruneInvisibleSelections()
					m.projectTree.clampCursor()
				}
				return nil
			},
		},
		{
			Name:        "jump to home section",
			Description: "Jump to context node's home section",
			Shortcut:    "go",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"home section", "origin section"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				node := m.projectTree.getSelectedNode()
				if node != nil {
					section := node.Task.Section
					if node.Task.CustomSection != "" {
						section = node.Task.CustomSection
					}
					if section == "" {
						section = models.SectionInbox
					}
					for i, n := range m.projectTree.visibleNodes() {
						if n.Task.InSection(section) && n.Task.IsRoot() {
							m.projectTree.cursor = i
							break
						}
					}
				}
				return nil
			},
		},
		{
			Name:        "cycle due date filter",
			Description: "Cycle through due date filter options",
			Shortcut:    "gd",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"due date filter", "filter due"},
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
	}
}
