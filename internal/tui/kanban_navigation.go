package tui

// findNextNonEmptyColumn searches for the next non-empty column in the given direction.
// direction should be -1 for left or +1 for right.
// Returns the index of the next non-empty column, or the current column index if
// no non-empty columns exist in the specified direction.
func (m *KanbanViewModel) findNextNonEmptyColumn(direction int) int {
	// Start searching from the next column in the given direction
	for i := m.ActiveColumn + direction; i >= 0 && i < len(KanbanColumnOrder); i += direction {
		section := KanbanColumnOrder[i]
		if len(m.Tasks[section]) > 0 {
			return i
		}
	}
	// No non-empty column found in direction, stay on current column
	return m.ActiveColumn
}
