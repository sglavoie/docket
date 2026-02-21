package tui

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

// ExitMultiSelect deactivates multi-select mode and clears all selections.
func (s *MultiSelectState) ExitMultiSelect() {
	s.multiSelectMode = false
	s.selectedTasks = make(map[string]bool)
}
