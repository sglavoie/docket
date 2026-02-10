package tui

// findNextNonEmptyColumn searches for the next non-empty column in the given direction.
// direction should be -1 for left or +1 for right.
// Returns the index of the next non-empty column, or the current column index if
// no non-empty columns exist in the specified direction.
func (m *KanbanViewModel) findNextNonEmptyColumn(direction int) int {
	// Start searching from the next column in the given direction
	for i := m.ActiveColumn + direction; i >= 0 && i < len(m.columnDefs); i += direction {
		key := m.columnDefs[i].Key
		if m.visibleNodeCount(key) > 0 {
			return i
		}
	}
	// No non-empty column found in direction, stay on current column
	return m.ActiveColumn
}

// currentVisibleNodes returns visible nodes for the active column (or active cell in swimlane mode).
func (m *KanbanViewModel) currentVisibleNodes() []*KanbanNode {
	if m.swimlaneEnabled() {
		return m.swimlaneCurrentVisibleNodes()
	}
	return m.visibleNodesForSection(m.activeColumnKey())
}

// currentVisibleNodeCount returns the count of visible nodes in the active column (or active cell in swimlane mode).
func (m *KanbanViewModel) currentVisibleNodeCount() int {
	if m.swimlaneEnabled() {
		return len(m.swimlaneCurrentVisibleNodes())
	}
	return m.visibleNodeCount(m.activeColumnKey())
}

// CurrentColumnState returns the ColumnState for the currently active column (or active cell in swimlane mode).
func (m *KanbanViewModel) CurrentColumnState() *ColumnState {
	if m.swimlaneEnabled() {
		return m.swimlaneCurrentColumnState()
	}
	return m.Columns[m.activeColumnKey()]
}
