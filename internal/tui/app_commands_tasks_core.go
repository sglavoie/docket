package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/storage"
)

func (m *AppModel) taskCommands() []Command {
	return []Command{
		{
			Name:        "new task",
			Description: "Create a new task",
			Shortcut:    "n",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"create task", "add task"},
			Handler: func() tea.Msg {
				return CreateTaskMsg{}
			},
		},
		{
			Name:        "quick capture",
			Description: "Quick capture a single task",
			Shortcut:    "i",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"capture", "inbox"},
			Handler: func() tea.Msg {
				if m.currentProject != nil {
					return ShowQuickCaptureMsg{Project: m.currentProject, AllProjects: m.config.Projects}
				}
				return StatusMsg("No project selected")
			},
		},
		{
			Name:        "bulk capture",
			Description: "Bulk capture multiple tasks",
			Shortcut:    "I",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"multi capture", "batch add"},
			Handler: func() tea.Msg {
				if m.currentProject != nil {
					return ShowBulkCaptureMsg{Project: m.currentProject}
				}
				return StatusMsg("No project selected")
			},
		},
		{
			Name:        "edit last task",
			Description: "Edit the most recently modified task",
			Shortcut:    "ctrl+l",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"edit recent", "last modified", "recent task"},
			Handler: func() tea.Msg {
				if m.currentProject == nil {
					return StatusMsg("No project selected")
				}
				recentTasks := storage.GetRecentTasks(m.currentProject.Tasks, 1)
				if len(recentTasks) == 0 {
					return StatusMsg("No recent tasks")
				}
				return EditTaskMsg{Task: recentTasks[0]}
			},
		},
		{
			Name:        "expand all",
			Description: "Expand all tasks in tree",
			Shortcut:    "E",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"unfold all", "open all"},
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
			Name:        "collapse all",
			Description: "Collapse all tasks in tree",
			Shortcut:    "z",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"fold all", "close all"},
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
			Name:        "archive completed tasks",
			Description: "Move completed tasks to archive file",
			Category:    CategoryActions,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"archive", "archive done", "archive completed"},
			Handler: func() tea.Msg {
				return ArchiveCompletedTasksMsg{}
			},
		},
		{
			Name:        "archive all completed tasks",
			Description: "Archive completed tasks across all projects",
			Category:    CategoryActions,
			Aliases:     []string{"archive all", "archive everything"},
			Handler: func() tea.Msg {
				return ArchiveAllCompletedTasksMsg{}
			},
		},
	}
}
