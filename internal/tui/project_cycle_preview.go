package tui

// renderCyclePreviewBackground renders a dimmed task tree of the highlighted
// project as background content behind the cycle overlay popup.
func (m *AppModel) renderCyclePreviewBackground(visibleRows ...int) string {
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

	previewRows := m.height - 2
	if len(visibleRows) > 0 {
		previewRows = visibleRows[0]
	}
	previewRows = max(previewRows, 1)

	if m.projectCyclePreviewID != proj.ID || m.projectCyclePreview == nil {
		m.projectCyclePreview = NewProjectTreeModel(proj, m.configManager, m.runtime)
		m.projectCyclePreviewID = proj.ID
	}
	m.projectCyclePreview.SetSize(m.width, previewRows)
	m.projectCyclePreview.fillTreeCardToViewHeight = true

	content := m.projectCyclePreview.View().Content

	// Dim content so the preview visually recedes behind the overlay
	return DimContent(content)
}

// invalidateCyclePreview clears the cached preview tree.
func (m *AppModel) invalidateCyclePreview() {
	m.projectCyclePreview = nil
	m.projectCyclePreviewID = ""
}
