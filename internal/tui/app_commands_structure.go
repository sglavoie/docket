package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *AppModel) structureCommands() []Command {
	commands := append([]Command{}, m.structureTypeCommands()...)
	commands = append(commands, m.structureMovementCommands()...)
	return append(commands, m.quickNoteCommand(), m.moveReparentCommand())
}

func (m *AppModel) structureTypeCommands() []Command {
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
	}
}

func (m *AppModel) structureMovementCommands() []Command {
	return []Command{
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
	}
}

func (m *AppModel) quickNoteCommand() Command {
	return Command{
		Name:        "quick note",
		Description: "Append a quick note/comment to the selected task",
		Shortcut:    "ctrl+.",
		Category:    CategoryStructure,
		Views:       []ViewType{ViewProjectTree, ViewOutliner, ViewCrossProjectOutliner, ViewAllTasks, ViewDailyReview, ViewWeeklyReview, ViewFocusView, ViewRecentActivity, ViewFocusBoard, ViewPriorityView, ViewDueDateOverview},
		Aliases:     []string{"note", "comment quick"},
		Handler: func() tea.Msg {
			task, project := m.quickNoteTarget()
			if task == nil || project == nil {
				return nil
			}
			return OpenQuickNoteMsg{Task: task, Project: project}
		},
	}
}

func (m *AppModel) moveReparentCommand() Command {
	return Command{
		Name:        "move/reparent task",
		Description: "Move selected task to a new parent",
		Shortcut:    "ctrl+m",
		Category:    CategoryStructure,
		Views:       []ViewType{ViewProjectTree, ViewOutliner, ViewCrossProjectOutliner},
		Aliases:     []string{"reparent", "move to parent"},
		Handler: func() tea.Msg {
			task, project, source := m.reparentTarget()
			if task == nil || project == nil {
				return nil
			}
			return buildOpenParentSelectorMsg(task, project, source)
		},
	}
}

func (m *AppModel) quickNoteTarget() (*models.Task, *models.Project) {
	origin := m.nonPaletteOrigin()
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
		if m.crossOutliner != nil {
			return m.crossOutliner.getTaskRef()
		}
	default:
	}

	type taskRefProvider interface {
		getTaskRef() (*models.Task, *models.Project)
	}
	if v := m.views.Get(origin); v != nil {
		if trp, ok := v.(taskRefProvider); ok {
			return trp.getTaskRef()
		}
	}
	return nil, nil
}

func (m *AppModel) reparentTarget() (*models.Task, *models.Project, ViewType) {
	origin := m.structureOrigin()
	switch origin {
	case ViewProjectTree:
		if m.projectTree != nil {
			return m.projectTree.getCursorTask(), m.projectTree.getProject(), ViewProjectTree
		}
	case ViewOutliner:
		if m.outliner != nil {
			task, project := m.outliner.getTaskRef()
			return task, project, ViewOutliner
		}
	case ViewCrossProjectOutliner:
		if m.crossOutliner != nil {
			task, project := m.crossOutliner.getTaskRef()
			return task, project, ViewCrossProjectOutliner
		}
	default:
	}
	return nil, nil, origin
}

func (m *AppModel) nonPaletteOrigin() ViewType {
	for i := len(m.viewStack) - 1; i >= 0; i-- {
		if view := m.viewStack[i]; view != ViewCommandPalette {
			return view
		}
	}
	return ViewDashboard
}

func (m *AppModel) structureOrigin() ViewType {
	for i := len(m.viewStack) - 1; i >= 0; i-- {
		view := m.viewStack[i]
		if view == ViewProjectTree || view == ViewOutliner || view == ViewCrossProjectOutliner {
			return view
		}
	}
	return ViewDashboard
}

func buildOpenParentSelectorMsg(task *models.Task, project *models.Project, source ViewType) tea.Msg {
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
		Source:         source,
		ReparentTaskID: task.ID,
	}
}
