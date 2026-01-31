package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m *AppModel) linkedTaskCommands() []Command {
	return []Command{
		{
			Name:        "create blocker",
			Description: "Create a new blocker linked task",
			Shortcut:    "cb",
			Category:    CategoryLinkedTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"add blocker", "new blocker"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				task := m.projectTree.getSelectedTask()
				if task != nil {
					return CreateLinkedTaskMsg{SourceTask: task, Project: m.projectTree.project, LinkType: LinkTypeBlocker}
				}
				return nil
			},
		},
		{
			Name:        "create dependent",
			Description: "Create a new dependent linked task",
			Shortcut:    "cd",
			Category:    CategoryLinkedTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"add dependent", "new dependent"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				task := m.projectTree.getSelectedTask()
				if task != nil {
					return CreateLinkedTaskMsg{SourceTask: task, Project: m.projectTree.project, LinkType: LinkTypeDependent}
				}
				return nil
			},
		},
		{
			Name:        "create subtask",
			Description: "Create a new subtask linked task",
			Shortcut:    "cs",
			Category:    CategoryLinkedTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"add subtask", "new subtask"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				task := m.projectTree.getSelectedTask()
				if task != nil {
					return CreateLinkedTaskMsg{SourceTask: task, Project: m.projectTree.project, LinkType: LinkTypeSubtask}
				}
				return nil
			},
		},
		{
			Name:        "create follow-up",
			Description: "Create a new follow-up linked task",
			Shortcut:    "cf",
			Category:    CategoryLinkedTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"add follow-up", "new follow-up"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				task := m.projectTree.getSelectedTask()
				if task != nil {
					return CreateLinkedTaskMsg{SourceTask: task, Project: m.projectTree.project, LinkType: LinkTypeFollowUp}
				}
				return nil
			},
		},
		{
			Name:        "navigate linked tasks",
			Description: "Navigate to linked tasks",
			Shortcut:    "cn",
			Category:    CategoryLinkedTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"show linked", "go to linked"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				task := m.projectTree.getSelectedTask()
				if task != nil {
					return ShowLinkedTaskSelectorMsg{Task: task, Project: m.projectTree.project}
				}
				return nil
			},
		},
		{
			Name:        "unlink task",
			Description: "Remove a linked task relationship",
			Shortcut:    "cx",
			Category:    CategoryLinkedTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"remove link", "delete link"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				task := m.projectTree.getSelectedTask()
				if task != nil {
					return ShowLinkedTaskSelectorMsg{Task: task, Project: m.projectTree.project, ForUnlink: true}
				}
				return nil
			},
		},
		{
			Name:        "cross-project link",
			Description: "Link task across projects",
			Shortcut:    "cp",
			Category:    CategoryLinkedTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"cross link", "inter-project link"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				task := m.projectTree.getSelectedTask()
				if task != nil {
					return ShowCrossProjectLinkerMsg{SourceTask: task, SourceProject: m.projectTree.project}
				}
				return nil
			},
		},
	}
}
