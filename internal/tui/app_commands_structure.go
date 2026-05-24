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
			Views:       []ViewType{ViewProjectTree, ViewOutliner, ViewCrossProjectOutliner, ViewAllTasks, ViewDailyReview, ViewWeeklyReview, ViewFocusView, ViewRecentActivity, ViewFocusBoard, ViewPriorityView, ViewDueDateOverview},
			Aliases:     []string{"note", "comment quick"},
			Handler: func() tea.Msg {
				getTaskAndProject := func() (*models.Task, *models.Project) {
					// Dispatch by the underlying view (palette is on top of stack).
					origin := ViewDashboard
					for i := len(m.viewStack) - 1; i >= 0; i-- {
						if v := m.viewStack[i]; v != ViewCommandPalette {
							origin = v
							break
						}
					}
					switch origin {
					case ViewProjectTree:
						if m.projectTree != nil {
							if task := m.projectTree.getCursorTask(); task != nil {
								return task, m.projectTree.getProject()
							}
						}
					case ViewOutliner:
						if m.outliner != nil {
							return m.outliner.getTaskRef()
						}
					case ViewCrossProjectOutliner:
						// crossOutliner.getProject() returns nil; use getTaskRef
						// which resolves the row's project from loadedProjects.
						if m.crossOutliner != nil {
							return m.crossOutliner.getTaskRef()
						}
					}
					// Secondary views that implement getTaskRef
					if v := m.views.Get(origin); v != nil {
						type taskRefProvider interface {
							getTaskRef() (*models.Task, *models.Project)
						}
						if trp, ok := v.(taskRefProvider); ok {
							return trp.getTaskRef()
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
			Views:       []ViewType{ViewProjectTree, ViewOutliner, ViewCrossProjectOutliner},
			Aliases:     []string{"reparent", "move to parent"},
			Handler: func() tea.Msg {
				// Dispatch based on the underlying project view rather than first-non-nil
				// model, so ViewCrossProjectOutliner routes through m.crossOutliner even
				// when m.outliner is also non-nil. The palette is on top of the view stack
				// when this Handler runs, so walk the stack to find the originating view.
				origin := ViewDashboard
				for i := len(m.viewStack) - 1; i >= 0; i-- {
					switch m.viewStack[i] {
					case ViewProjectTree, ViewOutliner, ViewCrossProjectOutliner:
						origin = m.viewStack[i]
					}
					if origin != ViewDashboard {
						break
					}
				}
				switch origin {
				case ViewProjectTree:
					if m.projectTree == nil {
						return nil
					}
					task := m.projectTree.getCursorTask()
					project := m.projectTree.getProject()
					if task == nil || project == nil {
						return nil
					}
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
				case ViewOutliner:
					if m.outliner == nil {
						return nil
					}
					task, project := m.outliner.getTaskRef()
					if task == nil || project == nil {
						return nil
					}
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
				case ViewCrossProjectOutliner:
					if m.crossOutliner == nil {
						return nil
					}
					task, project := m.crossOutliner.getTaskRef()
					if task == nil || project == nil {
						return nil
					}
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
						Source:         ViewCrossProjectOutliner,
						ReparentTaskID: task.ID,
					}
				}
				return nil
			},
		},
	}
}
