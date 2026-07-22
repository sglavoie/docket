package tui

// applyOverlays composites all popup/overlay layers onto the base content.
// h is the overlay positioning height (m.height for full-height views,
// targetContentHeight for views with an external status bar).
//
// Overlay visual contract:
//   - View-local prefix popups: no dim (user is mid-chord, context stays visible)
//   - Quick-switch popup: no dim (navigational, lightweight)
//   - Pickers/selection overlays (dimmedOverlayRenderers): dim
//   - Peek/preview: dim (replaces context with detail)
//   - Confirm dialogs: dim (destructive action requires focus)
func (m *AppModel) applyOverlays(content string, h int) string {
	if m.activeTaskWorkspace() && m.taskWorkspaceView != nil {
		if popup := m.taskWorkspaceView.renderViewMenu(m.width); popup != "" {
			content = OverlayPopupDimmed(content, popup, m.width, h)
		}
	}
	if planner, ok := m.activeView().(*FocusBoardModel); ok {
		if popup := planner.renderPlannerViewMenu(m.width); popup != "" {
			content = OverlayPopupDimmed(content, popup, m.width, h)
		}
	}
	// The canonical Ctrl+G destination switcher is a typed shell overlay.
	if popup := m.renderDestinationSwitcher(); popup != "" {
		content = OverlayPopupDimmed(content, popup, m.width, h)
	}

	// Overlay prefix popup if the current view provides one
	if popup := m.renderPrefixPopupOverlay(); popup != "" {
		content = OverlayPopup(content, popup, m.width, h)
	}

	// Overlay quick-switch popup if active
	if popup := m.renderQuickSwitchPopup(); popup != "" {
		content = OverlayPopup(content, popup, m.width, h)
	}

	// Replace base content with task preview when pinned popup is active
	if m.showPinnedPopup && len(m.pinnedPopupItems) > 0 {
		if preview := m.renderPinnedPreviewBackground(); preview != "" {
			content = preview
		}
	}

	// Overlay pinned tasks popup if active
	if popup := m.renderPinnedPopup(); popup != "" {
		content = OverlayPopupBottomLeft(content, popup, m.width, h)
	}

	// Overlay dimmed picker/note popups (all share the same nil-check + render pattern)
	for _, renderer := range m.dimmedOverlayRenderers() {
		if popup := renderer.render(m.width); popup != "" {
			content = OverlayPopupDimmed(content, popup, m.width, h)
		}
	}

	// Overlay peek task detail if active
	if popup := m.renderPeekOverlay(); popup != "" {
		content = OverlayPopupDimmed(content, popup, m.width, h)
	}

	// Overlay confirm dialog last so it renders on top of all other popups
	if popup := m.renderConfirmDialogOverlay(); popup != "" {
		content = OverlayPopupDimmed(content, popup, m.width, h)
	}

	return content
}

// widthRenderer is satisfied by all overlay pickers/popups that render given a width.
type widthRenderer interface {
	render(width int) string
}

// dimmedOverlayRenderers returns the non-nil overlay renderers that are composited
// with OverlayPopupDimmed. Order matters: later entries render on top.
func (m *AppModel) dimmedOverlayRenderers() []widthRenderer {
	var out []widthRenderer
	if m.globalProjectPicker != nil {
		out = append(out, m.globalProjectPicker)
	}
	if m.globalTaskPicker != nil {
		out = append(out, m.globalTaskPicker)
	}
	if m.statusQuickPicker != nil {
		out = append(out, m.statusQuickPicker)
	}
	if m.sectionQuickPicker != nil {
		out = append(out, m.sectionQuickPicker)
	}
	if m.linkTypeQuickPicker != nil {
		out = append(out, m.linkTypeQuickPicker)
	}
	if m.linkModeQuickPicker != nil {
		out = append(out, m.linkModeQuickPicker)
	}
	if m.priorityQuickPicker != nil {
		out = append(out, m.priorityQuickPicker)
	}
	if m.dueDateQuickPicker != nil {
		out = append(out, m.dueDateQuickPicker)
	}
	if m.sectionChangeQuickPicker != nil {
		out = append(out, m.sectionChangeQuickPicker)
	}
	if m.quickNoteOverlay != nil {
		out = append(out, m.quickNoteOverlay)
	}
	return out
}

// renderPrefixPopupOverlay checks if the current view has a pending prefix and
// renders the popup box for overlay compositing.
func (m *AppModel) renderPrefixPopupOverlay() string {
	view := m.activeView()
	if view == nil {
		return ""
	}
	provider, ok := view.(PrefixPopupProvider)
	if !ok {
		return ""
	}
	info := provider.GetPrefixPopupInfo()
	if info == nil {
		return ""
	}
	return RenderPrefixPopup(info.Prefix, info.Hints, m.width, m.height)
}

// renderConfirmDialogOverlay checks if an active confirm dialog should render
// as a floating overlay box.
func (m *AppModel) renderConfirmDialogOverlay() string {
	if m.projectArchiveConfirm.IsActive() {
		return m.projectArchiveConfirm.RenderOverlay(m.width)
	}
	if m.taskDeleteConfirm.IsActive() {
		return m.taskDeleteConfirm.RenderOverlay(m.width)
	}

	view := m.activeView()
	if view == nil {
		return ""
	}
	provider, ok := view.(ConfirmDialogProvider)
	if !ok {
		return ""
	}
	cd := provider.GetConfirmDialog()
	if cd == nil || !cd.IsActive() {
		return ""
	}
	return cd.RenderOverlay(m.width)
}
