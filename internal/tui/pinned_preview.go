package tui

// renderPinnedPreviewBackground renders a dimmed task outliner of the
// highlighted pinned task's project as background content behind the pinned
// tasks popup.
func (m *AppModel) renderPinnedPreviewBackground() string {
	if !m.showPinnedPopup || len(m.pinnedPopupItems) == 0 {
		return ""
	}

	entry := m.pinnedPopupItems[m.pinnedPopupCursor]
	proj := entry.Project
	taskID := entry.Task.ID

	if m.pinnedPreviewID != proj.ID || m.pinnedPreview == nil {
		m.pinnedPreview = NewOutlinerModel(proj, m.configManager, m.runtime)
		m.pinnedPreview.SetSize(m.width, m.height-2)
		m.pinnedPreviewID = proj.ID
		m.pinnedPreviewTaskID = ""
	}

	// Move cursor to the pinned task when the highlighted item changes. The
	// outliner consumes OpenAtTask on its next rebuild (triggered by View()).
	if m.pinnedPreviewTaskID != taskID {
		m.pinnedPreviewTaskID = taskID
		if task := proj.GetTaskByID(taskID); task != nil {
			m.pinnedPreview.OpenAtTask = task
		}
	}

	content := m.pinnedPreview.View().Content

	// Dim content so the preview visually recedes behind the overlay
	return DimContent(content)
}

// invalidatePinnedPreview clears the cached pinned preview outliner.
func (m *AppModel) invalidatePinnedPreview() {
	m.pinnedPreview = nil
	m.pinnedPreviewID = ""
	m.pinnedPreviewTaskID = ""
}
