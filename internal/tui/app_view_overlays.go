package tui

// applyOverlays composites all popup/overlay layers onto the base content.
// h is the overlay positioning height (m.height for full-height views,
// targetContentHeight for views with an external status bar).
//
// Overlay visual contract:
//   - Pickers/selection overlays (dimmedOverlayRenderers): dim
//   - Confirm dialogs: dim (destructive action requires focus)
func (m *AppModel) applyOverlays(content string, h int) string {
	// Overlay dimmed picker/note popups (all share the same nil-check + render pattern)
	for _, renderer := range m.dimmedOverlayRenderers() {
		if popup := renderOverlayWithin(renderer, m.width, h); popup != "" {
			content = overlayPopupBelowPersistentTitle(content, popup, m.width, h)
		}
	}

	// Overlay confirm dialog last so it renders on top of all other popups
	if popup := m.renderConfirmDialogOverlay(); popup != "" {
		content = overlayPopupBelowPersistentTitle(content, popup, m.width, h)
	}

	return content
}

// overlayPopupBelowPersistentTitle reserves the host's first row before
// fitting a transient popup. Full-screen hosts keep their workspace pulse
// visible while the popup owns only its local interaction chrome.
func overlayPopupBelowPersistentTitle(base, popup string, width, height int) string {
	popup = fitOverlayPopupHeight(popup, max(height-1, 1))
	return OverlayPopupDimmed(base, popup, width, height)
}

// widthRenderer is satisfied by all overlay pickers/popups that render given a width.
type widthRenderer interface {
	render(width int) string
}

// viewportRenderer lets an overlay trade scrollable rows for required inline
// content before generic popup fitting would need to remove interior rows.
type viewportRenderer interface {
	renderWithin(width, height int) string
}

func renderOverlayWithin(renderer widthRenderer, width, height int) string {
	if renderer, ok := renderer.(viewportRenderer); ok {
		return renderer.renderWithin(width, height)
	}
	return renderer.render(width)
}

// dimmedOverlayRenderers returns the non-nil overlay renderers that are composited
// with OverlayPopupDimmed. Order matters: later entries render on top.
func (m *AppModel) dimmedOverlayRenderers() []widthRenderer {
	var out []widthRenderer
	if m.contextActionSheet != nil {
		out = append(out, m.contextActionSheet)
	}
	if m.operationValuePicker != nil {
		out = append(out, m.operationValuePicker)
	}
	if m.operationTextPrompt != nil {
		out = append(out, m.operationTextPrompt)
	}
	return out
}

// renderConfirmDialogOverlay checks if an active confirm dialog should render
// as a floating overlay box.
func (m *AppModel) renderConfirmDialogOverlay() string {
	if m.projectArchiveConfirm.IsActive() {
		return m.projectArchiveConfirm.RenderOverlay(m.width)
	}
	if m.projectParkConfirm.IsActive() {
		return m.projectParkConfirm.RenderOverlay(m.width)
	}
	if m.taskDeleteConfirm.IsActive() {
		return m.taskDeleteConfirm.RenderOverlay(m.width)
	}
	if m.outlineDeleteConfirm.IsActive() {
		return m.outlineDeleteConfirm.RenderOverlay(m.width)
	}
	if m.operationConfirm.IsActive() {
		return m.operationConfirm.RenderOverlay(m.width)
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
