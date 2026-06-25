package tui

import tea "charm.land/bubbletea/v2"

// refreshAndClamp re-applies filters and clamps the cursor after filter changes.
func (m *ProjectFocusViewModel) refreshAndClamp() {
	m.resetViewScrollOffset()
	m.clampCursor()
}

func (m *ProjectFocusViewModel) buildTagCmd() tea.Cmd {
	return buildTagCmdFromProject(m.project, m.filter.Tags, ViewProjectFocusView, "Filter by Tags", false)
}
