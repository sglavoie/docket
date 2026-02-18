package tui

import (
	"github.com/sglavoie/tt/internal/models"
)

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

// moveCursorUp moves the cursor up within the current column.
func (m *KanbanViewModel) moveCursorUp() {
	cs := m.CurrentColumnState()
	if cs.Cursor > 0 {
		cs.Cursor--
	}
}

// moveCursorDown moves the cursor down within the current column.
func (m *KanbanViewModel) moveCursorDown() {
	cs := m.CurrentColumnState()
	visibleCount := m.currentVisibleNodeCount()
	if cs.Cursor < visibleCount-1 {
		cs.Cursor++
	}
}

// positionCursorOnTask positions the cursor on the specified task in the given section.
// If the task is not found, the cursor is positioned at the first task (or 0 if empty).
func (m *KanbanViewModel) positionCursorOnTask(task *models.Task, section string) {
	visibleNodes := m.visibleNodesForSection(section)
	cs := m.Columns[section]

	// Find the task among visible nodes
	for i, node := range visibleNodes {
		if node.Task.ID == task.ID {
			cs.Cursor = i
			return
		}
	}

	// Task not found (possibly collapsed), validate cursor to be in bounds
	cs.Validate(len(visibleNodes))
}

// moveColumnLeft moves focus to the previous column on the left.
// When all columns fit on screen, empty columns are skipped (except in swimlane mode).
// When horizontal scrolling is active, moves one column at a time.
func (m *KanbanViewModel) moveColumnLeft() {
	if len(m.columnDefs) > m.maxVisibleColumns() || m.swimlaneEnabled() {
		if m.ActiveColumn > 0 {
			m.ActiveColumn--
		}
	} else {
		m.ActiveColumn = m.findNextNonEmptyColumn(-1)
	}
	m.ensureActiveColumnVisible()
}

// moveColumnRight moves focus to the next column on the right.
// When all columns fit on screen, empty columns are skipped (except in swimlane mode).
// When horizontal scrolling is active, moves one column at a time.
func (m *KanbanViewModel) moveColumnRight() {
	if len(m.columnDefs) > m.maxVisibleColumns() || m.swimlaneEnabled() {
		if m.ActiveColumn < len(m.columnDefs)-1 {
			m.ActiveColumn++
		}
	} else {
		m.ActiveColumn = m.findNextNonEmptyColumn(+1)
	}
	m.ensureActiveColumnVisible()
}

// customSectionTabs returns ["All"] + sorted custom sections, or nil if no custom sections exist.
func (m *KanbanViewModel) customSectionTabs() []string {
	return CustomSectionTabs(m.Project)
}

// activeCustomSectionFilter returns the custom section name being filtered, or "" for "All".
func (m *KanbanViewModel) activeCustomSectionFilter() string {
	return ActiveCustomSectionFilter(m.Project, m.customSectionTab)
}
