package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// handleDependencyMessages handles dependency-related messages (manage, view, navigate).
// Returns true if the message was handled.
func (m *AppModel) handleDependencyMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ManageDependenciesMsg:
		if msg.Task != nil && msg.Project != nil {
			m.showView(ViewDependencySelector, NewDependencySelectorModel(msg.Task, msg.Project))
		}
		return true, nil

	case DependencySelectorDoneMsg:
		m.popView()
		if msg.HasChanges && msg.Task != nil && msg.Project != nil {
			if !msg.Project.MutateTask(msg.Task.ID, func(t *models.Task) {
				t.DependsOn = msg.NewDepsOn
			}) {
				return true, errorCmd("task no longer exists in project")
			}
			return true, tea.Batch(
				startLoading("Saving..."),
				SaveProjectAsync(msg.Project, "Dependencies updated"),
			)
		}
		return true, nil

	case ShowDependencyViewerMsg:
		if msg.Task != nil && msg.Project != nil {
			m.showView(ViewDependencyViewer, NewDependencyViewerModel(msg.Task, msg.Project))
		}
		return true, nil

	case CloseDependencyViewerMsg:
		m.popView()
		return true, nil

	case DependencyViewerSelectMsg:
		m.popView()
		if msg.Task != nil && m.projectTree != nil {
			// If the task is completed, switch to Completed view so it's visible
			if msg.Task.IsComplete() && m.projectTree.viewMode != ViewModeCompleted {
				m.projectTree.viewMode = ViewModeCompleted
				m.projectTree.updateVisibility()
			}
			m.projectTree.NavigateToTask(msg.Task)
		}
		return true, nil

	case DependencyViewerViewDepsMsg:
		// Replace current view with new dependency viewer for selected task
		m.popView()
		if msg.Task != nil && msg.Project != nil {
			m.showView(ViewDependencyViewer, NewDependencyViewerModel(msg.Task, msg.Project))
		}
		return true, nil
	}
	return false, nil
}
