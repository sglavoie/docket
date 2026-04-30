package tui

import (
	tea "charm.land/bubbletea/v2"

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
		if getMultiTasks != nil {
			if tasks := getMultiTasks(); len(tasks) > 0 {
				if clearSel != nil {
					clearSel()
				}
				return true, func() tea.Msg { return CopyTasksTitleMsg{Tasks: tasks} }
			}
		}
		if task := getTask(); task != nil {
			return true, func() tea.Msg { return CopyTaskTitleMsg{Task: task} }
		}
		return true, nil

	case "Y":
		if getMultiTasks != nil {
			if tasks := getMultiTasks(); len(tasks) > 0 {
				if clearSel != nil {
					clearSel()
				}
				return true, func() tea.Msg { return CopyTasksDetailsMsg{Tasks: tasks} }
			}
		}
		if task := getTask(); task != nil {
			return true, func() tea.Msg { return CopyTaskDetailsMsg{Task: task} }
		}
		return true, nil

	case "ctrl+y":
		if getMultiTasks != nil {
			if tasks := getMultiTasks(); len(tasks) > 0 {
				if clearSel != nil {
					clearSel()
				}
				return true, func() tea.Msg { return CopyTasksTitleAndDescMsg{Tasks: tasks} }
			}
		}
		if task := getTask(); task != nil {
			return true, func() tea.Msg { return CopyTaskTitleAndDescMsg{Task: task} }
		}
		return true, nil
	}

	return false, nil
}


