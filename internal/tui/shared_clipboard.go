package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// handleClipboardKey handles y/Y/ctrl+y clipboard operations.
// getTask returns the single task at cursor (nil if none).
// getMultiTasks returns the selected tasks in multi-select mode (nil if not applicable).
// clearSel clears multi-select state after copying.
// Returns (handled, cmd).
func handleClipboardKey(key string, getTask func() *models.Task, getMultiTasks func() []*models.Task, clearSel func()) (bool, tea.Cmd) {
	switch key {
	case "y":
		if tasks := getMultiTasks(); len(tasks) > 0 {
			clearSel()
			return true, func() tea.Msg { return CopyTasksTitleMsg{Tasks: tasks} }
		}
		if task := getTask(); task != nil {
			return true, func() tea.Msg { return CopyTaskTitleMsg{Task: task} }
		}
		return true, nil

	case "Y":
		if tasks := getMultiTasks(); len(tasks) > 0 {
			clearSel()
			return true, func() tea.Msg { return CopyTasksDetailsMsg{Tasks: tasks} }
		}
		if task := getTask(); task != nil {
			return true, func() tea.Msg { return CopyTaskDetailsMsg{Task: task} }
		}
		return true, nil

	case "ctrl+y":
		if tasks := getMultiTasks(); len(tasks) > 0 {
			clearSel()
			return true, func() tea.Msg { return CopyTasksTitleAndDescMsg{Tasks: tasks} }
		}
		if task := getTask(); task != nil {
			return true, func() tea.Msg { return CopyTaskTitleAndDescMsg{Task: task} }
		}
		return true, nil
	}

	return false, nil
}

// clipboardCmd is a convenience wrapper for views without multi-select.
// It dispatches y/Y/ctrl+y clipboard operations for a single task.
// Returns (cmd, true) when the key was consumed, or (nil, false) otherwise.
func clipboardCmd(key string, task *models.Task) (tea.Cmd, bool) {
	handled, cmd := handleClipboardKey(key, func() *models.Task { return task }, nil, nil)
	return cmd, handled
}
