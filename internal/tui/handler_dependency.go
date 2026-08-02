package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
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
			// The selector produced the complete post-edit dependency set (only
			// non-cyclic, non-self candidates are selectable), so replace the whole
			// set through the application layer as one ledger-undoable ChangeSet.
			return true, errorCmd("%s", app.RejectRetiredCapability(app.RetiredCapabilityDependency))
		}
		return true, nil

	case ShowDependencyViewerMsg:
		if msg.Task != nil && msg.Project != nil {
			m.showView(ViewDependencyViewer, NewDependencyViewerModel(msg.Task, msg.Project))
		}
		return true, nil

	case DependencyViewerSelectMsg:
		m.popView()
		if msg.Task != nil && m.crossOutliner != nil {
			m.crossOutliner.NavigateToTask(msg.Task)
			return true, nil
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
