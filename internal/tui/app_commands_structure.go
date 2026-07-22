package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *AppModel) structureCommands() []Command {
	return []Command{m.quickNoteCommand(), m.moveReparentCommand()}
}

func (m *AppModel) quickNoteCommand() Command {
	return Command{
		Name:        "quick note",
		Description: "Append a quick note/comment to the selected task",
		Shortcut:    "ctrl+.",
		Category:    CategoryStructure,
		Views:       []PresentationID{ViewOutliner, ViewCrossProjectOutliner, ViewDailyReview, ViewWeeklyReview, ViewRecentActivity, ViewFocusBoard},
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
		Description: "Move selected task to a new parent or clear its parent",
		Shortcut:    "ctrl+m",
		Category:    CategoryStructure,
		Views:       []PresentationID{ViewOutliner, ViewCrossProjectOutliner},
		Aliases:     []string{"reparent", "move to parent", "clear parent", "remove parent"},
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
	if view, _, ok := m.outlinerPrimary(); ok {
		return view.getTaskRef()
	}
	origin := m.nonPaletteOrigin()
	switch origin {
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

func (m *AppModel) reparentTarget() (*models.Task, *models.Project, PresentationID) {
	if view, params, ok := m.outlinerPrimary(); ok {
		task, project := view.getTaskRef()
		return task, project, taskWorkspacePresentation(params)
	}
	origin := m.structureOrigin()
	switch origin {
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

func (m *AppModel) nonPaletteOrigin() PresentationID {
	for i := len(m.viewStack) - 1; i >= 0; i-- {
		if view := m.viewStack[i]; view != ViewCommandPalette {
			return view
		}
	}
	return ViewDashboard
}

func (m *AppModel) structureOrigin() PresentationID {
	for i := len(m.viewStack) - 1; i >= 0; i-- {
		view := m.viewStack[i]
		if view == ViewOutliner || view == ViewCrossProjectOutliner {
			return view
		}
	}
	return ViewDashboard
}

func buildOpenParentSelectorMsg(task *models.Task, project *models.Project, source PresentationID) tea.Msg {
	validParents := buildReparentValidParents(project, task)
	var currentParentID *string
	if task.ParentID != nil {
		id := *task.ParentID
		currentParentID = &id
	}
	return OpenParentSelectorMsg{
		ValidParents: validParents,
		CurrentID:    currentParentID,
		AllowNone:    true,
		Source:       source,
		ReparentTask: taskIdentityFor(task, project),
	}
}
