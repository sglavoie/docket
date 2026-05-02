package tui

// clampCursor ensures cursor is within valid range.
func (m *ProjectFocusViewModel) clampCursor() {
	m.ClampCursorTo(len(m.filteredTasks()))
}
