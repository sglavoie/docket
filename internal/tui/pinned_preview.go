package tui

// renderPinnedPreviewBackground renders a dimmed task tree of the highlighted
// pinned task's project as background content behind the pinned tasks popup.
func (m *AppModel) renderPinnedPreviewBackground() string {
	if !m.showPinnedPopup || len(m.pinnedPopupItems) == 0 {
		return ""
	}

	entry := m.pinnedPopupItems[m.pinnedPopupCursor]
	proj := entry.Project
	taskID := entry.Task.ID

	if m.pinnedPreviewID != proj.ID {
		m.pinnedPreview = NewProjectTreeModel(proj, m.configManager, m.runtime)
		m.pinnedPreview.SetSize(m.width, m.height-2)
		m.pinnedPreviewID = proj.ID
		m.pinnedPreviewTaskID = ""
	}

	// Move cursor to the pinned task when the highlighted item changes
	if m.pinnedPreviewTaskID != taskID {
		m.pinnedPreviewTaskID = taskID
		m.focusPinnedPreviewOnTask(taskID)
	}

	content := m.pinnedPreview.View().Content

	// Dim content so the preview visually recedes behind the overlay
	return DimContent(content)
}

// focusPinnedPreviewOnTask positions the preview cursor on the given task.
// If the task isn't visible in the current view mode, switches to All mode.
func (m *AppModel) focusPinnedPreviewOnTask(taskID string) {
	if m.pinnedPreview == nil {
		return
	}
	// Try finding in current view mode first
	if m.setCursorOnTask(taskID) {
		return
	}
	// Task not visible — switch to All and retry
	m.pinnedPreview.viewMode = ViewModeAll
	m.pinnedPreview.updateVisibility()
	m.setCursorOnTask(taskID)
}

// setCursorOnTask sets the preview cursor to the node matching taskID.
// Returns true if found.
func (m *AppModel) setCursorOnTask(taskID string) bool {
	visible := m.pinnedPreview.visibleNodes()
	for i, node := range visible {
		if node.Task != nil && node.Task.ID == taskID {
			m.pinnedPreview.cursor = i
			return true
		}
	}
	return false
}

// invalidatePinnedPreview clears the cached pinned preview tree.
func (m *AppModel) invalidatePinnedPreview() {
	m.pinnedPreview = nil
	m.pinnedPreviewID = ""
	m.pinnedPreviewTaskID = ""
}
