package tui

import "github.com/sglavoie/tt/internal/tui/chrome"

// applyOverlays composites all popup/overlay layers onto the base content.
// h is the overlay positioning height (m.viewNav.height for full-height views,
// targetContentHeight for views with an external status bar).
//
// Overlay visual contract:
//   - Pickers/selection overlays (dimmedOverlayRenderers): dim
//   - Confirm dialogs: dim (destructive action requires focus)
func (m *AppModel) applyOverlays(content string, h int) string {
	// Overlay dimmed picker/note popups (all share the same nil-check + render pattern)
	for _, renderer := range m.dimmedOverlayRenderers() {
		if popup := renderOverlayWithin(renderer, m.viewNav.width, h); popup != "" {
			content = overlayPopupBelowPersistentTitle(content, popup, m.viewNav.width, h)
		}
	}

	// Overlay confirm dialog last so it renders on top of all other popups
	if popup := m.renderConfirmDialogOverlay(); popup != "" {
		content = overlayPopupBelowPersistentTitle(content, popup, m.viewNav.width, h)
	}

	return content
}

// overlayPopupBelowPersistentTitle reserves the host's first row before
// fitting a transient popup. Full-screen hosts keep their workspace pulse
// visible while the popup owns only its local interaction chrome.
func overlayPopupBelowPersistentTitle(base, popup string, width, height int) string {
	popup = chrome.FitOverlayPopupHeight(popup, max(height-1, 1))
	return chrome.OverlayPopupDimmed(base, popup, width, height)
}

// widthRenderer is satisfied by all overlay pickers/popups that render given a
// width. Exported for the same reason as keyInterceptor.HandleKey.
type widthRenderer interface {
	Render(width int) string
}

// viewportRenderer lets an overlay trade scrollable rows for required inline
// content before generic popup fitting would need to remove interior rows.
// It is selected by type assertion rather than by the compiler, so an
// unexported method here would fail silently rather than loudly once an overlay
// moved into a sub-package.
type viewportRenderer interface {
	RenderWithin(width, height int) string
}

func renderOverlayWithin(renderer widthRenderer, width, height int) string {
	if renderer, ok := renderer.(viewportRenderer); ok {
		return renderer.RenderWithin(width, height)
	}
	return renderer.Render(width)
}

// dimmedOverlayRenderers returns the non-nil overlay renderers that are composited
// with OverlayPopupDimmed. Order matters: later entries render on top.
func (m *AppModel) dimmedOverlayRenderers() []widthRenderer {
	var out []widthRenderer
	if m.lenses.mounted() {
		// First, so the sheet, prompt, and confirmation a lens action opens all
		// paint above the picker that raised them.
		out = append(out, m.lenses.picker)
	}
	if m.operations.actionSheet != nil {
		out = append(out, m.operations.actionSheet)
	}
	if m.operations.valuePicker != nil {
		out = append(out, m.operations.valuePicker)
	}
	if m.operations.textPrompt != nil {
		out = append(out, m.operations.textPrompt)
	}
	if m.conflictBrowser != nil {
		out = append(out, m.conflictBrowser)
	}
	return out
}

// renderConfirmDialogOverlay checks if an active confirm dialog should render
// as a floating overlay box.
func (m *AppModel) renderConfirmDialogOverlay() string {
	if overlay, armed := m.project.confirmOverlay(m.viewNav.width); armed {
		return overlay
	}
	if overlay, armed := m.taskMutation.confirmOverlay(m.viewNav.width); armed {
		return overlay
	}
	if overlay, armed := m.operations.confirmOverlay(m.viewNav.width); armed {
		return overlay
	}

	view := m.activeView()
	if view == nil {
		return ""
	}
	provider, ok := view.(chrome.ConfirmDialogProvider)
	if !ok {
		return ""
	}
	cd := provider.GetConfirmDialog()
	if cd == nil || !cd.IsActive() {
		return ""
	}
	return cd.RenderOverlay(m.viewNav.width)
}
