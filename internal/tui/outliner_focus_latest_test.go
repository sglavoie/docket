package tui

import "github.com/sglavoie/docket/internal/tui/outliner"

// cursorTaskID returns the task ID under the cursor, or "" when the cursor does
// not name a task row.
func cursorTaskID(m *outliner.OutlinerModel) string {
	if m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return ""
	}
	row := m.Rows[m.Cursor]
	if !row.IsTaskRow() || row.Task == nil {
		return ""
	}
	return row.Task.ID
}
