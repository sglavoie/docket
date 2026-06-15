package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *AppModel) statusCommands() []Command {
	return []Command{
		{
			Name:        "start task",
			Description: "Set task status to in progress",
			Shortcut:    ",s",
			Category:    CategoryStatus,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"begin task", "in progress"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					return m.runProjectTreeBulkStatusTransition(models.StatusInProgress)
				}
				return nil
			},
		},
		{
			Name:        "complete task",
			Description: "Set task status to done",
			Shortcut:    ",d",
			Category:    CategoryStatus,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"done task", "finish task", "mark done"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					return m.runProjectTreeBulkStatusTransition(models.StatusDone)
				}
				return nil
			},
		},
		{
			Name:        "block task",
			Description: "Set task status to blocked",
			Shortcut:    ",b",
			Category:    CategoryStatus,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"blocked task", "mark blocked"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					return m.runProjectTreeBulkStatusTransition(models.StatusBlocked)
				}
				return nil
			},
		},
		{
			Name:        "standby task",
			Description: "Set task status to standby",
			Shortcut:    ",S",
			Category:    CategoryStatus,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"standby", "pause task"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					return m.runProjectTreeBulkStatusTransition(models.StatusStandby)
				}
				return nil
			},
		},
		{
			Name:        "cancel task",
			Description: "Set task status to canceled",
			Shortcut:    ",x",
			Category:    CategoryStatus,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"canceled task", "mark canceled"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					return m.runProjectTreeBulkStatusTransition(models.StatusCanceled)
				}
				return nil
			},
		},
		{
			Name:        "reopen task",
			Description: "Reopen a completed or canceled task",
			Shortcut:    ",o",
			Category:    CategoryStatus,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"reopen", "undo complete", "mark todo"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					return m.runProjectTreeBulkStatusTransition(models.StatusTodo)
				}
				return nil
			},
		},
		{
			Name:        "review task",
			Description: "Set task status to review",
			Shortcut:    ",r",
			Category:    CategoryStatus,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"mark review", "in review"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					return m.runProjectTreeBulkStatusTransition(models.StatusReview)
				}
				return nil
			},
		},
		{
			Name:        "maybe task",
			Description: "Set task status to maybe",
			Shortcut:    ",m",
			Category:    CategoryStatus,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"maybe", "someday task"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					return m.runProjectTreeBulkStatusTransition(models.StatusMaybe)
				}
				return nil
			},
		},
	}
}
