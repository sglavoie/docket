package tui

import (
	tea "charm.land/bubbletea/v2"
)

func (m *AppModel) foldCommands() []Command {
	return []Command{
		{
			Name:        "toggle fold",
			Description: "Toggle expand/collapse current node",
			Shortcut:    "za",
			Category:    CategoryFold,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"toggle expand", "toggle collapse"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				node := m.projectTree.getSelectedNode()
				if node != nil {
					if len(node.Task.Children) > 0 {
						node.Expanded = !node.Expanded
						m.projectTree.updateVisibility()
					}
				}
				return nil
			},
		},
		{
			Name:        "open fold",
			Description: "Expand current node",
			Shortcut:    "zo",
			Category:    CategoryFold,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"expand node", "unfold"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				node := m.projectTree.getSelectedNode()
				if node != nil {
					node.Expanded = true
					m.projectTree.updateVisibility()
				}
				return nil
			},
		},
		{
			Name:        "close fold",
			Description: "Collapse current node",
			Shortcut:    "zc",
			Category:    CategoryFold,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"collapse node", "fold"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				node := m.projectTree.getSelectedNode()
				if node != nil {
					node.Expanded = false
					m.projectTree.updateVisibility()
				}
				return nil
			},
		},
		{
			Name:        "expand all folds",
			Description: "Expand all nodes in tree",
			Shortcut:    "zR",
			Category:    CategoryFold,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"unfold all", "open all folds"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					for _, node := range m.projectTree.nodes {
						node.Expanded = true
					}
					m.projectTree.updateVisibility()
				}
				return nil
			},
		},
		{
			Name:        "collapse all folds",
			Description: "Collapse all nodes in tree",
			Shortcut:    "zM",
			Category:    CategoryFold,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"fold all", "close all folds"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					for _, node := range m.projectTree.nodes {
						if node.Level > 0 {
							node.Expanded = false
						}
					}
					m.projectTree.updateVisibility()
				}
				return nil
			},
		},
		{
			Name:        "focus subtree",
			Description: "Focus on current subtree only",
			Shortcut:    "zv",
			Category:    CategoryFold,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"zoom subtree", "subtree focus"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				task := m.projectTree.getSelectedTask()
				if task != nil {
					if task.Type.CanHaveChildren() {
						m.projectTree.focusedTask = task
						m.projectTree.cursor = 0
						m.projectTree.buildTree()
					}
				}
				return nil
			},
		},
		{
			Name:        "toggle preview",
			Description: "Toggle inline preview for cursor task",
			Shortcut:    "zp",
			Category:    CategoryFold,
			Views:       []ViewType{ViewProjectTree},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.previewMode = !m.projectTree.previewMode
				}
				return nil
			},
		},
	}
}
