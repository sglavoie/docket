package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/taskdetail"
)

// handleViewSupportMessages handles attachment views.
func (m *AppModel) handleViewSupportMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case taskdetail.ShowTaskAttachmentsMsg:
		if msg.Task == nil || msg.Project == nil {
			return true, nil
		}
		view := taskdetail.NewTaskAttachmentsModel(
			msg.Task,
			msg.Project,
			m.runtime,
			m.secondaryFeatures.Attachment,
		)
		view.SetOperationDispatch(m.dispatchOperationKey)
		return m.showViewInit(presentation.ViewTaskAttachments, view)
	}

	return false, nil
}
