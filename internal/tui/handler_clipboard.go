package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/external"
)

// handleClipboardMessages handles clipboard-related messages (copy task title/details).
// Returns true if the message was handled.
func (m *AppModel) handleClipboardMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case TaskClipboardProjectedMsg:
		if err := copyTextToClipboard(msg.Projection.Text); err != nil {
			return true, errorCmd("copy failed: %w", err)
		}
		switch msg.Projection.Operation {
		case app.TaskOperationCopyTitle:
			return true, func() tea.Msg { return TaskCopiedMsg{Count: 1} }
		case app.TaskOperationCopyDescription:
			return true, func() tea.Msg { return TaskCopiedMsg{Count: 1, FullDetail: true} }
		case app.TaskOperationCopyTitleDescription:
			return true, func() tea.Msg { return TaskCopiedMsg{Count: 1, TitleAndDesc: true} }
		default:
			return true, errorCmd("copy failed: unknown clipboard projection %q", msg.Projection.Operation)
		}

	case CopyTaskTitleMsg:
		if msg.Task == nil {
			return true, nil
		}
		_, err := external.CopyTaskTitle(msg.Task)
		if err != nil {
			return true, errorCmd("copy failed: %w", err)
		}
		return true, func() tea.Msg {
			return TaskCopiedMsg{Count: 1, FullDetail: false}
		}

	case CopyTaskDetailsMsg:
		if msg.Task == nil {
			return true, nil
		}
		if msg.Task.Description == "" {
			m.setInfoStatus("No description to copy")
			return true, defaultStatusTimeout()
		}
		_, err := external.CopyTaskDetails(msg.Task)
		if err != nil {
			return true, errorCmd("copy failed: %w", err)
		}
		return true, func() tea.Msg {
			return TaskCopiedMsg{Count: 1, FullDetail: true}
		}

	case CopyTaskTitleAndDescMsg:
		if msg.Task == nil {
			return true, nil
		}
		_, err := external.CopyTaskTitleAndDescription(msg.Task)
		if err != nil {
			return true, errorCmd("copy failed: %w", err)
		}
		return true, func() tea.Msg {
			return TaskCopiedMsg{Count: 1, TitleAndDesc: true}
		}

	case TaskCopiedMsg:
		var what string
		if msg.TitleAndDesc {
			what = "title and description"
		} else if msg.FullDetail {
			what = "description"
		} else {
			what = "title"
		}
		if msg.Count == 1 {
			m.setSuccessStatus(fmt.Sprintf("Copied %s to clipboard", what))
		} else {
			m.setSuccessStatus(fmt.Sprintf("Copied %s for %d tasks to clipboard", what, msg.Count))
		}
		return true, defaultStatusTimeout()
	}
	return false, nil
}
