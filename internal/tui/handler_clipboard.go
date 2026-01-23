package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/external"
)

// handleClipboardMessages handles clipboard-related messages (copy task title/details).
// Returns true if the message was handled.
func (m *AppModel) handleClipboardMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
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
			m.statusMessage = "No description to copy"
			return true, statusTimeout(2 * time.Second)
		}
		_, err := external.CopyTaskDetails(msg.Task)
		if err != nil {
			return true, errorCmd("copy failed: %w", err)
		}
		return true, func() tea.Msg {
			return TaskCopiedMsg{Count: 1, FullDetail: true}
		}

	case CopyTaskReferenceMsg:
		if msg.Task == nil {
			return true, nil
		}
		ref, err := external.CopyTaskReference(msg.Task)
		if err != nil {
			return true, errorCmd("copy failed: %w", err)
		}
		m.statusMessage = fmt.Sprintf("Copied \"%s\" to clipboard", ref)
		return true, statusTimeout(2 * time.Second)

	case CopyTasksTitleMsg:
		if len(msg.Tasks) == 0 {
			return true, nil
		}
		_, err := external.CopyTaskTitles(msg.Tasks)
		if err != nil {
			return true, errorCmd("copy failed: %w", err)
		}
		count := len(msg.Tasks)
		return true, func() tea.Msg {
			return TaskCopiedMsg{Count: count, FullDetail: false}
		}

	case CopyTasksDetailsMsg:
		if len(msg.Tasks) == 0 {
			return true, nil
		}
		hasDescription := false
		for _, task := range msg.Tasks {
			if task.Description != "" {
				hasDescription = true
				break
			}
		}
		if !hasDescription {
			m.statusMessage = "No descriptions to copy"
			return true, statusTimeout(2 * time.Second)
		}
		_, err := external.CopyTasksDetails(msg.Tasks)
		if err != nil {
			return true, errorCmd("copy failed: %w", err)
		}
		count := len(msg.Tasks)
		return true, func() tea.Msg {
			return TaskCopiedMsg{Count: count, FullDetail: true}
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

	case CopyTasksTitleAndDescMsg:
		if len(msg.Tasks) == 0 {
			return true, nil
		}
		_, err := external.CopyTasksTitleAndDescription(msg.Tasks)
		if err != nil {
			return true, errorCmd("copy failed: %w", err)
		}
		count := len(msg.Tasks)
		return true, func() tea.Msg {
			return TaskCopiedMsg{Count: count, TitleAndDesc: true}
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
			m.statusMessage = fmt.Sprintf("Copied %s to clipboard", what)
		} else {
			m.statusMessage = fmt.Sprintf("Copied %s for %d tasks to clipboard", what, msg.Count)
		}
		return true, statusTimeout(2 * time.Second)
	}
	return false, nil
}
