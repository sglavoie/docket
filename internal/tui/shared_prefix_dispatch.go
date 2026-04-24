package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

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

	// selectMsg returns the tea.Msg to emit when gp (go to project) is used.
	selectMsg(task *models.Task, project *models.Project) tea.Msg

	// JumpToTop resets cursor to 0 and returns flash cmd (for gg).
	JumpToTop() tea.Cmd

	// handleFilterPrefix handles the F prefix key.
	handleFilterPrefix(key string) (tea.Model, tea.Cmd)

	// handleFilterSubPrefix handles filter sub-prefix keys.
	handleFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd)
}

// dispatchCommonPrefix handles prefix key sequences shared across secondary
// views: , (status), ! (priority), d (due date), s (snooze), g (goto),
// m (section move), c (linked task), F (filter), and filter sub-prefixes.
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
		if model, cmd, handled := dispatchDueDatePrefix(key, v); handled {
			return model, cmd, true
		}

	case "g":
		switch key {
		case "g":
			cmd := v.JumpToTop()
			return v, cmd, true
		case "p":
			task, project := v.getTaskRef()
			if task != nil {
				msg := v.selectMsg(task, project)
				return v, func() tea.Msg { return msg }, true
			}
			return v, nil, true
		}

	case "m":
		section, ok := sectionForMoveKey(key)
		if !ok {
			return v, nil, true
		}
		model, cmd := taskOpsSectionMove(v, section)
		return model, cmd, true

	case "!":
		if _, ok := PriorityForPrefixKey(key); ok {
			model, cmd := taskOpsChangePriority(v, key)
			return model, cmd, true
		}
		return v, nil, true

	case "s":
		if key == "d" {
			task, project := v.getTaskRef()
			if task != nil {
				return v, func() tea.Msg {
					return SetSnoozeDateMsg{Task: task, Project: project}
				}, true
			}
			return v, nil, true
		}
		snoozeDate, _, ok := resolveQuickSnoozeDate(key)
		if !ok {
			return v, nil, true
		}
		model, cmd := taskOpsQuickSnooze(v, snoozeDate)
		return model, cmd, true

	case "c":
		task, project := v.getTaskRef()
		if task == nil {
			return v, nil, true
		}
		if handled, cmd := handleLinkedTaskPrefixKey(key, task, project, false, false); handled {
			return v, cmd, true
		}
		return v, nil, true

	case "t":
		model, cmd := dispatchTaskPrefix(key, v)
		return model, cmd, true

	case ".":
		stage := StageForPrefixKey(key, v.getConfig().Stages)
		if stage != nil {
			model, cmd := taskOpsChangeStage(v, *stage)
			return model, cmd, true
		}
		return v, nil, true
	}

	return nil, nil, false
}

// dispatchDueDatePrefix handles the d-prefix key sequence for due date operations.
func dispatchDueDatePrefix(key string, v prefixDispatchView) (tea.Model, tea.Cmd, bool) {
	if days, isClear, ok := DueDateDaysForPrefixKey(key); ok {
		if isClear {
			model, cmd := taskOpsClearDueDate(v)
			return model, cmd, true
		}
		model, cmd := taskOpsSetQuickDueDate(v, days)
		return model, cmd, true
	}
	if key == "d" {
		task, project := v.getTaskRef()
		if task != nil {
			return v, func() tea.Msg {
				return SetDueDateMsg{Task: task, Project: project}
			}, true
		}
		return v, nil, true
	}
	return nil, nil, false
}

// dispatchTaskPrefix handles the t-prefix key sequence for task operations.
func dispatchTaskPrefix(key string, v prefixDispatchView) (tea.Model, tea.Cmd) {
	task, project := v.getTaskRef()
	if task != nil {
		if handled, cmd := handleTaskPrefixMsgKey(key, task, project); handled {
			return v, cmd
		}
		if handled, cmd := handleTaskActionPrefixKey(key, task, project, nil, nil); handled {
			return v, cmd
		}
	}
	switch key {
	case "d":
		return taskOpsToggleDimmed(v)
	case "p":
		return taskOpsTogglePinned(v)
	}
	return v, nil
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
	// CanMoveSection returns true when section moves are allowed (e.g. Outliner
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
	case "Ft", "Fs", "Fd", "Fa", "Fas", "Far", "Fae", "Fac", "Fav", "Faw", "Fao", "Fap", "FS", "F.":
		return true
	}
	return false
}

// stageKeyForIndex returns the shortcut key for a stage at the given index.
// Indices 0-8 map to "1"-"9", indices 9-34 map to "a"-"z", beyond returns "".
func stageKeyForIndex(i int) string {
	if i >= 0 && i < 9 {
		return string(rune('1' + i))
	}
	if i >= 9 && i < 35 {
		return string(rune('a' + i - 9))
	}
	return ""
}

// stageIndexForKey returns the stage index for a shortcut key, or -1 if unrecognized.
// "1"-"9" map to 0-8, "a"-"z" map to 9-34.
func stageIndexForKey(key string) int {
	if len(key) != 1 {
		return -1
	}
	ch := key[0]
	if ch >= '1' && ch <= '9' {
		return int(ch - '1')
	}
	if ch >= 'a' && ch <= 'z' {
		return int(ch-'a') + 9
	}
	return -1
}

// StageForPrefixKey maps a key press to a stage name from the configured stages.
// Keys 1-9 map to stages[0..8], a-z map to stages[9..34], key 0 clears (empty string).
// Returns nil if the key is not recognized.
func StageForPrefixKey(key string, stages []string) *string {
	if key == "0" {
		s := ""
		return &s
	}
	idx := stageIndexForKey(key)
	if idx >= 0 && idx < len(stages) {
		return &stages[idx]
	}
	return nil
}
