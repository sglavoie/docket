package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
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
				if m.projectTreeHasBulkSelection() {
					return m.runProjectTreeBulkDueDatePrefix("t")
				}
				if task := m.selectedTreeTask(); task != nil {
					now := time.Now()
					today := util.StartOfDay(now)
					m.projectTree.project.MutateTask(task.ID, func(t *models.Task) {
						t.DueDate = &today
						t.HasTime = false
					})
					return TaskPropertyChangedMsg{Task: task, NeedsSave: true}
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
				if m.projectTreeHasBulkSelection() {
					return m.runProjectTreeBulkDueDatePrefix("m")
				}
				if task := m.selectedTreeTask(); task != nil {
					now := time.Now()
					tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
					m.projectTree.project.MutateTask(task.ID, func(t *models.Task) {
						t.DueDate = &tomorrow
						t.HasTime = false
					})
					return TaskPropertyChangedMsg{Task: task, NeedsSave: true}
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
				if m.projectTreeHasBulkSelection() {
					return m.runProjectTreeBulkDueDatePrefix("w")
				}
				if task := m.selectedTreeTask(); task != nil {
					now := time.Now()
					nextWeek := time.Date(now.Year(), now.Month(), now.Day()+7, 0, 0, 0, 0, now.Location())
					m.projectTree.project.MutateTask(task.ID, func(t *models.Task) {
						t.DueDate = &nextWeek
						t.HasTime = false
					})
					return TaskPropertyChangedMsg{Task: task, NeedsSave: true}
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
				if m.projectTreeHasBulkSelection() {
					return m.runProjectTreeBulkDueDatePrefix("n")
				}
				if task := m.selectedTreeTask(); task != nil {
					now := time.Now()
					nextMonth := time.Date(now.Year(), now.Month(), now.Day()+30, 0, 0, 0, 0, now.Location())
					m.projectTree.project.MutateTask(task.ID, func(t *models.Task) {
						t.DueDate = &nextMonth
						t.HasTime = false
					})
					return TaskPropertyChangedMsg{Task: task, NeedsSave: true}
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
				if m.projectTreeHasBulkSelection() {
					return m.runProjectTreeBulkDueDatePrefix("0")
				}
				if task := m.selectedTreeTask(); task != nil {
					m.projectTree.project.MutateTask(task.ID, func(t *models.Task) {
						t.DueDate = nil
						t.HasTime = false
					})
					return TaskPropertyChangedMsg{Task: task, NeedsSave: true}
				}
				return StatusMsg("No task selected")
			},
		},
	}
}
