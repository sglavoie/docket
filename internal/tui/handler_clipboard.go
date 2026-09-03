package tui

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/tui/status"
)

var errClipboardUnsupported = errors.New("clipboard not supported on this OS")

// copyTextToClipboard is a package var so tests can inject clipboard success,
// unsupported, and error paths without shelling out to the platform tool.
var copyTextToClipboard = func(text string) error {
	var clipCmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		clipCmd = exec.Command("pbcopy")
	case "linux":
		clipCmd = exec.Command("xclip", "-selection", "clipboard")
	case "windows":
		clipCmd = exec.Command("clip")
	default:
		return errClipboardUnsupported
	}

	clipCmd.Stdin = strings.NewReader(text)
	return clipCmd.Run()
}

// handleClipboardMessages handles clipboard-related messages (copy task title/details).
// Returns true if the message was handled.
func (m *AppModel) handleClipboardMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case TaskClipboardProjectedMsg:
		if err := copyTextToClipboard(msg.Projection.Text); err != nil {
			return true, status.ErrorCmd("copy failed: %w", err)
		}
		switch msg.Projection.Operation {
		case app.TaskOperationCopyTitle:
			return true, func() tea.Msg { return TaskCopiedMsg{Count: 1} }
		case app.TaskOperationCopyDescription:
			return true, func() tea.Msg { return TaskCopiedMsg{Count: 1, FullDetail: true} }
		case app.TaskOperationCopyTitleDescription:
			return true, func() tea.Msg { return TaskCopiedMsg{Count: 1, TitleAndDesc: true} }
		default:
			return true, status.ErrorCmd("copy failed: unknown clipboard projection %q", msg.Projection.Operation)
		}

	case CopyTaskTitleMsg:
		if msg.Task == nil {
			return true, nil
		}
		_, err := external.CopyTaskTitle(msg.Task)
		if err != nil {
			return true, status.ErrorCmd("copy failed: %w", err)
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
			return true, status.DefaultStatusTimeout()
		}
		_, err := external.CopyTaskDetails(msg.Task)
		if err != nil {
			return true, status.ErrorCmd("copy failed: %w", err)
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
			return true, status.ErrorCmd("copy failed: %w", err)
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
		return true, status.DefaultStatusTimeout()
	}
	return false, nil
}
