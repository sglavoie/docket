package tui

// renderCyclePreviewBackground renders a dimmed task tree of the highlighted
// project as background content behind the cycle overlay popup.
func (m *AppModel) renderCyclePreviewBackground() string {
	if !m.showProjectCycle || len(m.projectCycleItems) == 0 {
		return ""
	}
	if m.projectCycleCursor < 0 || m.projectCycleCursor >= len(m.projectCycleItems) {
		return ""
	}

	proj := m.projectCycleItems[m.projectCycleCursor].Project
	if proj == nil {
		return ""
	}
	if m.projectCyclePreviewID != proj.ID {
		m.projectCyclePreview = NewProjectTreeModel(proj, m.config, m.configManager)
		m.projectCyclePreview.SetSize(m.width, m.height-2)
		m.projectCyclePreviewID = proj.ID
	}

	content := m.projectCyclePreview.View().Content

	// Dim content so the preview visually recedes behind the overlay
	return DimContent(content)
}

// invalidateCyclePreview clears the cached preview tree.
func (m *AppModel) invalidateCyclePreview() {
	m.projectCyclePreview = nil
	m.projectCyclePreviewID = ""
}
