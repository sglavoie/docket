package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *AppModel) structureCommands() []Command {
	return []Command{
		{
			Name:        "promote type",
			Description: "Promote task type (task → story → epic)",
			Shortcut:    "<",
			Category:    CategoryStructure,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"promote", "type up"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				m.projectTree.handleTypeKeys("<")
				return nil
			},
		},
		{
			Name:        "demote type",
			Description: "Demote task type (epic → story → task)",
			Shortcut:    ">",
			Category:    CategoryStructure,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"demote", "type down"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				m.projectTree.handleTypeKeys(">")
				return nil
			},
		},
		{
			Name:        "cycle leaf type left",
			Description: "Cycle leaf type left (task ← bug ← idea)",
			Shortcut:    "H",
			Category:    CategoryStructure,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"type left", "previous type"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				m.projectTree.handleTypeKeys("H")
				return nil
			},
		},
		{
			Name:        "cycle leaf type right",
			Description: "Cycle leaf type right (task → bug → idea)",
			Shortcut:    "L",
			Category:    CategoryStructure,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"type right", "next type"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				m.projectTree.handleTypeKeys("L")
				return nil
			},
		},
		{
			Name:        "move task up",
			Description: "Move task up among siblings",
			Shortcut:    "K",
			Category:    CategoryStructure,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"reorder up", "task up"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				m.projectTree.handleMovementKeys("K", visible)
				return nil
			},
		},
		{
			Name:        "move task down",
			Description: "Move task down among siblings",
			Shortcut:    "J",
			Category:    CategoryStructure,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"reorder down", "task down"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				m.projectTree.handleMovementKeys("J", visible)
				return nil
			},
		},
		{
			Name:        "indent task",
			Description: "Indent task (nest under task above)",
			Shortcut:    "→",
			Category:    CategoryStructure,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"nest task", "make child"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				m.projectTree.handleMovementKeys("right", visible)
				return nil
			},
		},
		{
			Name:        "outdent task",
			Description: "Outdent task (move to parent's level)",
			Shortcut:    "←",
			Category:    CategoryStructure,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"unnest task", "make sibling"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				m.projectTree.handleMovementKeys("left", visible)
				return nil
			},
		},
		{
			Name:        "quick note",
			Description: "Append a quick note/comment to the selected task",
			Shortcut:    "ctrl+.",
			Category:    CategoryStructure,
			Views:       []ViewType{ViewProjectTree, ViewOutliner},
			Aliases:     []string{"note", "comment quick"},
			Handler: func() tea.Msg {
				getTaskAndProject := func() (*models.Task, *models.Project) {
					if m.projectTree != nil {
						if task := m.projectTree.getCursorTask(); task != nil {
							return task, m.projectTree.getProject()
						}
					}
					if m.outliner != nil {
						if task := m.outliner.getCursorTask(); task != nil {
							return task, m.outliner.getProject()
						}
					}
					return nil, nil
				}
				task, project := getTaskAndProject()
				if task == nil || project == nil {
					return nil
				}
				return OpenQuickNoteMsg{Task: task, Project: project}
			},
		},
		{
			Name:        "move/reparent task",
			Description: "Move selected task to a new parent",
			Shortcut:    "ctrl+m",
			Category:    CategoryStructure,
			Views:       []ViewType{ViewProjectTree, ViewOutliner},
			Aliases:     []string{"reparent", "move to parent"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					task := m.projectTree.getCursorTask()
					project := m.projectTree.getProject()
					if task != nil && project != nil {
						validParents := buildReparentValidParents(project, task)
						var currentParentID *string
						if task.ParentID != nil {
							id := *task.ParentID
							currentParentID = &id
						}
						return OpenParentSelectorMsg{
							ValidParents:   validParents,
							CurrentID:      currentParentID,
							AllowNone:      true,
							Source:         ViewProjectTree,
							ReparentTaskID: task.ID,
						}
					}
				}
				if m.outliner != nil {
					task := m.outliner.getCursorTask()
					project := m.outliner.getProject()
					if task != nil && project != nil {
						validParents := buildReparentValidParents(project, task)
						var currentParentID *string
						if task.ParentID != nil {
							id := *task.ParentID
							currentParentID = &id
						}
						return OpenParentSelectorMsg{
							ValidParents:   validParents,
							CurrentID:      currentParentID,
							AllowNone:      true,
							Source:         ViewOutliner,
							ReparentTaskID: task.ID,
						}
					}
				}
				return nil
			},
		},
	}
}
