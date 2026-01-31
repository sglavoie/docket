package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *AppModel) dueDateCommands() []Command {
	return []Command{
		{
			Name:        "due today",
			Description: "Set due date to today",
			Shortcut:    "dt",
			Category:    CategoryDueDate,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"today", "due now"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				task := m.projectTree.getSelectedTask()
				if task != nil {
					now := time.Now()
					today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
					task.DueDate = &today
					task.UpdatedAt = time.Now()
					return TaskUpdatedMsg{Task: task}
				}
				return StatusMsg("No task selected")
			},
		},
		{
			Name:        "due tomorrow",
			Description: "Set due date to tomorrow",
			Shortcut:    "dm",
			Category:    CategoryDueDate,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"tomorrow"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				task := m.projectTree.getSelectedTask()
				if task != nil {
					now := time.Now()
					tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
					task.DueDate = &tomorrow
					task.UpdatedAt = time.Now()
					return TaskUpdatedMsg{Task: task}
				}
				return StatusMsg("No task selected")
			},
		},
		{
			Name:        "due next week",
			Description: "Set due date to +7 days",
			Shortcut:    "dw",
			Category:    CategoryDueDate,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"next week"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				task := m.projectTree.getSelectedTask()
				if task != nil {
					now := time.Now()
					nextWeek := time.Date(now.Year(), now.Month(), now.Day()+7, 0, 0, 0, 0, now.Location())
					task.DueDate = &nextWeek
					task.UpdatedAt = time.Now()
					return TaskUpdatedMsg{Task: task}
				}
				return StatusMsg("No task selected")
			},
		},
		{
			Name:        "due next month",
			Description: "Set due date to +30 days",
			Shortcut:    "dn",
			Category:    CategoryDueDate,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"next month"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				task := m.projectTree.getSelectedTask()
				if task != nil {
					now := time.Now()
					nextMonth := time.Date(now.Year(), now.Month(), now.Day()+30, 0, 0, 0, 0, now.Location())
					task.DueDate = &nextMonth
					task.UpdatedAt = time.Now()
					return TaskUpdatedMsg{Task: task}
				}
				return StatusMsg("No task selected")
			},
		},
		{
			Name:        "clear due date",
			Description: "Remove due date from task",
			Shortcut:    "d0",
			Category:    CategoryDueDate,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"remove due date", "no due date"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				task := m.projectTree.getSelectedTask()
				if task != nil {
					task.DueDate = nil
					task.UpdatedAt = time.Now()
					return TaskUpdatedMsg{Task: task}
				}
				return StatusMsg("No task selected")
			},
		},
	}
}
