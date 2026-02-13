package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// prefixDispatchView provides the methods needed by the shared prefix
// dispatcher. Views that handle prefix key sequences implement this to
// share the common `,`, `d`, `g`, `m`, `c`, and `F` prefix logic.
type prefixDispatchView interface {
	taskOpsView

	// getTaskAndProject returns the task and project at cursor.
	getTaskAndProject() (*models.Task, *models.Project)

	// selectMsg returns the tea.Msg to emit when gp (go to project) is used.
	selectMsg(task *models.Task, project *models.Project) tea.Msg

	// ResetCursor sets cursor to 0 (for gg).
	ResetCursor()

	// handleFilterPrefix handles the F prefix key.
	handleFilterPrefix(key string) (tea.Model, tea.Cmd)

	// handleFilterSubPrefix handles filter sub-prefix keys.
	handleFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd)
}

// dispatchCommonPrefix handles prefix key sequences shared across secondary
// views: , (status), d (due date), g (goto), m (section move), c (linked
// task), F (filter), and filter sub-prefixes.
//
// viewSpecific is called first for prefixes the view handles differently (like
// z, s, bulk ops). It returns (model, cmd, handled). If handled is true, the
// dispatcher returns that result.
//
// Returns (model, cmd, handled). When handled is false, the prefix was not
// recognized by the common dispatcher.
func dispatchCommonPrefix(
	prefix, key string,
	v prefixDispatchView,
	viewSpecific func(prefix, key string) (tea.Model, tea.Cmd, bool),
) (tea.Model, tea.Cmd, bool) {
	// Let the view handle its specific prefixes first
	if viewSpecific != nil {
		if model, cmd, handled := viewSpecific(prefix, key); handled {
			return model, cmd, true
		}
	}

	// Filter sub-prefixes
	if isFilterSubPrefix(prefix) {
		model, cmd := v.handleFilterSubPrefix(prefix, key)
		return model, cmd, true
	}

	switch prefix {
	case "F":
		model, cmd := v.handleFilterPrefix(key)
		return model, cmd, true

	case ",":
		if status, ok := StatusForPrefixKey(key); ok {
			model, cmd := taskOpsChangeStatus(v, status)
			return model, cmd, true
		}

	case "d":
		if days, isClear, ok := DueDateDaysForPrefixKey(key); ok {
			if isClear {
				model, cmd := taskOpsClearDueDate(v)
				return model, cmd, true
			}
			model, cmd := taskOpsSetQuickDueDate(v, days)
			return model, cmd, true
		}
		if key == "d" {
			task, project := v.getTaskAndProject()
			if task != nil {
				return v.(tea.Model), func() tea.Msg {
					return SetDueDateMsg{Task: task, Project: project}
				}, true
			}
			return v.(tea.Model), nil, true
		}

	case "g":
		switch key {
		case "g":
			v.ResetCursor()
			return v.(tea.Model), nil, true
		case "p":
			task, project := v.getTaskAndProject()
			if task != nil {
				msg := v.selectMsg(task, project)
				return v.(tea.Model), func() tea.Msg { return msg }, true
			}
			return v.(tea.Model), nil, true
		}

	case "m":
		section, ok := sectionForMoveKey(key)
		if !ok {
			return v.(tea.Model), nil, true
		}
		model, cmd := taskOpsSectionMove(v, section)
		return model, cmd, true

	case "c":
		task, project := v.getTaskAndProject()
		if task == nil {
			return v.(tea.Model), nil, true
		}
		if handled, cmd := handleLinkedTaskPrefixKey(key, task, project, false, false); handled {
			return v.(tea.Model), cmd, true
		}
		return v.(tea.Model), nil, true
	}

	return nil, nil, false
}

// isFilterSubPrefix returns true if the prefix is a filter sub-prefix (Ft, Fs, etc).
func isFilterSubPrefix(prefix string) bool {
	switch prefix {
	case "Ft", "Fs", "Fd", "Fa", "Fas", "Far", "Fae", "Fac", "Fav", "Faw", "Fao", "Fap", "FS":
		return true
	}
	return false
}
