package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// activateSecondaryPrefix checks whether key is a valid prefix key for a
// secondary view. Keys listed in always activate unconditionally; keys listed
// in gated activate only when hasItem is true. Returns the prefix string and
// true when the prefix was activated.
func activateSecondaryPrefix(key string, always, gated string, hasItem bool) (string, bool) {
	if len(key) != 1 {
		return "", false
	}
	r := rune(key[0])
	if strings.ContainsRune(always, r) {
		return key, true
	}
	if hasItem && strings.ContainsRune(gated, r) {
		return key, true
	}
	return "", false
}

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

// SectionMoveCallbacks provides the view-specific operations needed by
// handleSectionMovePrefix. Each primary view populates this with its own
// move methods so the dispatch logic can be shared.
type SectionMoveCallbacks struct {
	// MoveToSection moves the cursor task to the given section.
	MoveToSection func(section string) (tea.Model, tea.Cmd)
	// BulkMoveToSection moves all selected tasks to the given section.
	BulkMoveToSection func(section string) (tea.Model, tea.Cmd)
	// MoveToTop moves the cursor task to the top of its group.
	MoveToTop func() (tea.Model, tea.Cmd)
	// MoveToBottom moves the cursor task to the bottom of its group.
	MoveToBottom func() (tea.Model, tea.Cmd)
	// IsMultiSelect returns true when multi-select mode is active with selections.
	IsMultiSelect func() bool
	// CanMoveSection returns true when section moves are allowed (e.g. Kanban
	// only allows them in section group-by mode). Nil means always allowed.
	CanMoveSection func() bool
}

// handleSectionMovePrefix dispatches the m-prefix key to the appropriate
// section move or position move callback. Returns (model, cmd, handled).
func handleSectionMovePrefix(key string, cb SectionMoveCallbacks) (tea.Model, tea.Cmd, bool) {
	if section, ok := sectionForMoveKey(key); ok {
		if cb.CanMoveSection != nil && !cb.CanMoveSection() {
			return nil, nil, true
		}
		if cb.IsMultiSelect != nil && cb.IsMultiSelect() {
			model, cmd := cb.BulkMoveToSection(section)
			return model, cmd, true
		}
		model, cmd := cb.MoveToSection(section)
		return model, cmd, true
	}
	switch key {
	case "K":
		model, cmd := cb.MoveToTop()
		return model, cmd, true
	case "J":
		model, cmd := cb.MoveToBottom()
		return model, cmd, true
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
