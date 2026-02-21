package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// commonActionView extends taskOpsView with the additional methods needed
// by handleCommonTaskAction to dispatch shared task actions.
type commonActionView interface {
	taskOpsView
	clampCursor()
	viewID() ViewType
	viewName() string
	GetHelpItems() []HelpItem
	showDeleteConfirm(shortID int)
}

// jumpCursor moves cursor by delta within [0, total-1].
func jumpCursor(cursor *int, total, delta int) {
	*cursor += delta
	if *cursor >= total {
		*cursor = total - 1
	}
	if *cursor < 0 {
		*cursor = 0
	}
}

// handleListNavigation handles standard list navigation keys (up/k, down/j,
// G, ctrl+u/pgup, ctrl+d/pgdown). Returns true if the key was handled.
func handleListNavigation(key string, cursor *int, total, height int) bool {
	switch key {
	case "up", "k":
		if *cursor > 0 {
			*cursor--
		}
		return true

	case "down", "j":
		if *cursor < total-1 {
			*cursor++
		}
		return true

	case "G":
		if total > 0 {
			*cursor = total - 1
		}
		return true

	case "ctrl+u", "pgup":
		jumpCursor(cursor, total, -HalfPage(height))
		return true

	case "ctrl+d", "pgdown":
		jumpCursor(cursor, total, HalfPage(height))
		return true
	}

	return false
}

// handleCommonTaskAction handles task actions shared across secondary views:
// t (tags), Z (snooze), D (delete confirm), e (edit), r (refresh),
// y/Y/ctrl+y (clipboard), ? (help).
//
// Returns (model, cmd, handled). When handled is false, the key was not
// recognized and the caller should continue with view-specific handling.
func handleCommonTaskAction(key string, v commonActionView) (tea.Model, tea.Cmd, bool) {
	switch key {
	case "t":
		task, project := v.getTaskRef()
		if task != nil {
			return v.(tea.Model), func() tea.Msg {
				return ShowTagSelectorMsg{Task: task, Project: project, CurrentTags: append([]string{}, task.Tags...)}
			}, true
		}
		return v.(tea.Model), nil, true

	case "Z":
		task, project := v.getTaskRef()
		if task != nil {
			return v.(tea.Model), func() tea.Msg {
				return SetSnoozeDateMsg{Task: task, Project: project}
			}, true
		}
		return v.(tea.Model), nil, true

	case "D":
		task, _ := v.getTaskRef()
		if task != nil {
			v.showDeleteConfirm(task.ShortID)
		}
		return v.(tea.Model), nil, true

	case "e":
		task, project := v.getTaskRef()
		if task != nil {
			return v.(tea.Model), func() tea.Msg {
				return EditTaskMsg{Task: task, Project: project}
			}, true
		}
		return v.(tea.Model), nil, true

	case "r":
		v.reloadData()
		v.clampCursor()
		v.setStatus("Refreshed")
		return v.(tea.Model), nil, true

	case "y", "Y", "ctrl+y":
		task, _ := v.getTaskRef()
		if cmd, handled := clipboardCmd(key, task); handled {
			return v.(tea.Model), cmd, true
		}
		return v.(tea.Model), nil, true

	case "?":
		vid := v.viewID()
		vname := v.viewName()
		items := v.GetHelpItems()
		return v.(tea.Model), func() tea.Msg {
			return ShowHelpMsg{
				SourceView: vid,
				ViewName:   vname,
				Items:      items,
			}
		}, true
	}

	return nil, nil, false
}
