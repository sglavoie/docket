package tui

import "fmt"

// MultiSelectState provides reusable multi-selection state for views that
// support selecting multiple tasks. Embed this in view models to share the
// common fields and basic selection operations.
type MultiSelectState struct {
	multiSelectMode bool
	selectedTasks   map[string]bool
}

// ToggleSelection toggles the selection state of a task by ID.
func (s *MultiSelectState) ToggleSelection(id string) {
	if s.selectedTasks[id] {
		delete(s.selectedTasks, id)
	} else {
		if s.selectedTasks == nil {
			s.selectedTasks = make(map[string]bool)
		}
		s.selectedTasks[id] = true
	}
}

// ClearSelection resets all selections without changing multiSelectMode.
func (s *MultiSelectState) ClearSelection() {
	s.selectedTasks = make(map[string]bool)
}

// IsSelected returns true if the given task ID is selected.
func (s *MultiSelectState) IsSelected(id string) bool {
	return s.selectedTasks[id]
}

// SelectedCount returns the number of currently selected items.
func (s *MultiSelectState) SelectedCount() int {
	return len(s.selectedTasks)
}

// IsMultiSelectActive reports whether multi-select mode is on. Used by the
// global key dispatcher to keep space bound to toggle-select instead of peek.
func (s *MultiSelectState) IsMultiSelectActive() bool {
	return s.multiSelectMode
}

// ExitMultiSelect deactivates multi-select mode and clears all selections.
func (s *MultiSelectState) ExitMultiSelect() {
	s.multiSelectMode = false
	s.selectedTasks = make(map[string]bool)
}

// ToggleMode enters or exits multi-select mode. On entry, statusMsg is set to
// enterMsg. On exit, selections are cleared and statusMsg is set to empty.
func (s *MultiSelectState) ToggleMode(statusMsg *string, enterMsg string) {
	s.multiSelectMode = !s.multiSelectMode
	if !s.multiSelectMode {
		s.ClearSelection()
		*statusMsg = ""
	} else {
		*statusMsg = enterMsg
	}
}

// ToggleAndUpdateStatus toggles one task's selection, then updates the status
// message. noSelMsg is shown when no tasks are selected; withSelFmt (containing
// one %d verb) is used when at least one task is selected.
func (s *MultiSelectState) ToggleAndUpdateStatus(id string, statusMsg *string, noSelMsg, withSelFmt string) {
	s.ToggleSelection(id)
	s.UpdateStatus(statusMsg, noSelMsg, withSelFmt)
}

// UpdateStatus sets the status message based on current selection count.
// noSelMsg is used when count is 0; withSelFmt (with one %d) when count > 0.
func (s *MultiSelectState) UpdateStatus(statusMsg *string, noSelMsg, withSelFmt string) {
	count := s.SelectedCount()
	if count == 0 {
		*statusMsg = noSelMsg
	} else {
		*statusMsg = fmt.Sprintf(withSelFmt, count)
	}
}

// Checkbox returns "[x] " if id is selected, "[ ] " if not, or "" when not in
// multi-select mode. Use this to prepend a selection indicator to task lines.
func (s *MultiSelectState) Checkbox(id string) string {
	if !s.multiSelectMode {
		return ""
	}
	if s.selectedTasks[id] {
		return "[x] "
	}
	return "[ ] "
}

// FinishBulk exits multi-select mode and sets the status message to msg.
func (s *MultiSelectState) FinishBulk(statusMsg *string, msg string) {
	s.ExitMultiSelect()
	*statusMsg = msg
}
