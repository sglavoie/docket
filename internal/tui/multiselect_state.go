package tui

import "fmt"

// MultiSelectState provides reusable multi-selection state for views that
// support selecting multiple tasks. Embed this in view models to share the
// common fields and basic selection operations.
type MultiSelectState struct {
	multiSelectMode        bool
	selectedTaskIdentities map[TaskIdentity]bool
}

// ClearSelection resets all selections without changing multiSelectMode.
func (s *MultiSelectState) ClearSelection() {
	s.selectedTaskIdentities = make(map[TaskIdentity]bool)
}

// SelectedCount returns the number of currently selected items.
func (s *MultiSelectState) SelectedCount() int {
	return len(s.selectedTaskIdentities)
}

// IsMultiSelectActive reports whether multi-select mode is on. Used by the
// global key dispatcher to keep space bound to toggle-select instead of peek.
func (s *MultiSelectState) IsMultiSelectActive() bool {
	return s.multiSelectMode
}

// ExitMultiSelect deactivates multi-select mode and clears all selections.
func (s *MultiSelectState) ExitMultiSelect() {
	s.multiSelectMode = false
	s.ClearSelection()
}

func (s *MultiSelectState) ToggleTaskIdentity(id TaskIdentity) {
	if !id.Valid() {
		return
	}
	if s.selectedTaskIdentities[id] {
		delete(s.selectedTaskIdentities, id)
		return
	}
	if s.selectedTaskIdentities == nil {
		s.selectedTaskIdentities = make(map[TaskIdentity]bool)
	}
	s.selectedTaskIdentities[id] = true
}

func (s *MultiSelectState) SelectTaskIdentity(id TaskIdentity) {
	if !id.Valid() {
		return
	}
	if s.selectedTaskIdentities == nil {
		s.selectedTaskIdentities = make(map[TaskIdentity]bool)
	}
	s.selectedTaskIdentities[id] = true
}

func (s *MultiSelectState) DeselectTaskIdentity(id TaskIdentity) {
	delete(s.selectedTaskIdentities, id)
}

func (s *MultiSelectState) IsTaskIdentitySelected(id TaskIdentity) bool {
	return id.Valid() && s.selectedTaskIdentities[id]
}

// CheckboxTaskIdentity renders an identity-safe multi-select marker for
// cross-project task lists.
func (s *MultiSelectState) CheckboxTaskIdentity(id TaskIdentity) string {
	if !s.multiSelectMode {
		return ""
	}
	if s.IsTaskIdentitySelected(id) {
		return "[x] "
	}
	return "[ ] "
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

// FinishBulk exits multi-select mode and sets the status message to msg.
func (s *MultiSelectState) FinishBulk(statusMsg *string, msg string) {
	s.ExitMultiSelect()
	*statusMsg = msg
}
