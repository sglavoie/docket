package tui

import (
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/presentation"
)

// applyOverlays composites all popup/overlay layers onto the base content.
// h is the overlay positioning height (m.viewNav.height for full-height views,
// targetContentHeight for views with an external status bar), and panelDivider
// the row a bounded panel opens with, or noPanelDivider.
//
// Overlay visual contract:
//   - Pickers/selection overlays (dimmedOverlayRenderers): dim
//   - Confirm dialogs: dim (destructive action requires focus)
func (m *AppModel) applyOverlays(content string, h, panelDivider int) string {
	// Where each popup lands is recorded as it is composited, so a click is read
	// against the frame on screen. A popup this pass does not place has none.
	m.pointer.overlays = m.pointer.overlays[:0]
	// Overlay dimmed picker/note popups (all share the same nil-check + render pattern)
	for _, renderer := range m.dimmedOverlayRenderers() {
		if anchored, ok := m.overlayAnchoredToOutlinerRow(renderer, content, h); ok {
			content = anchored
			continue
		}
		if popup := renderOverlayWithin(renderer, m.viewNav.width, h); popup != "" {
			var placement chrome.OverlayPlacement
			content, placement = overlayPopupPlacedBelowPersistentTitle(content, popup, m.viewNav.width, h)
			m.pointer.overlays = append(m.pointer.overlays, placedOverlay{renderer: renderer, placement: placement})
		}
	}

	// Overlay confirm dialog last so it renders on top of all other popups. It
	// stands beside a bounded panel rather than on it: the panel's divider is
	// what says where the panel starts, so the dialog's halo never takes it.
	if popup := m.renderConfirmDialogOverlay(); popup != "" {
		content = chrome.OverlayPopupDimmedKeepingRow(content, popup, m.viewNav.width, h, 1, panelDivider)
	}

	return content
}

// overlayAnchoredToOutlinerRow composites the task action sheet under the
// Outliner row that raised it, or above that row when the sheet does not fit
// below, never covering the Outliner's header rows. It reads the row the last
// render drew the cursor on, so it answers false — and the sheet takes the
// ordinary bottom-centered placement — for any other overlay, any other sheet,
// another presentation, or a cursor row the render did not draw.
func (m *AppModel) overlayAnchoredToOutlinerRow(renderer widthRenderer, content string, h int) (string, bool) {
	sheet := m.operations.ActionSheet
	if sheet == nil || sheet != m.operations.RowSheet || renderer != widthRenderer(sheet) {
		return "", false
	}
	if m.currentPresentationView() != presentation.ViewCrossProjectOutliner || m.crossOutliner == nil {
		return "", false
	}
	row, ok := m.crossOutliner.CursorScreenRow()
	if !ok || row >= h {
		return "", false
	}
	top := min(m.crossOutliner.HostHeaderRows(), row)
	popup := renderOverlayWithin(sheet, m.viewNav.width, max(h-top, 1))
	if popup == "" {
		return content, true
	}
	composed, placement := chrome.OverlayPopupDimmedAnchoredPlaced(content, popup, m.viewNav.width, h, row, top)
	m.pointer.overlays = append(m.pointer.overlays, placedOverlay{renderer: renderer, placement: placement})
	return composed, true
}

// overlayPopupPlacedBelowPersistentTitle reserves the host's first row before
// fitting a transient popup. Full-screen hosts keep their workspace pulse
// visible while the popup owns only its local interaction chrome. It reports
// where the popup landed, and whether either fitting kept every row of it.
func overlayPopupPlacedBelowPersistentTitle(base, popup string, width, height int) (string, chrome.OverlayPlacement) {
	fitted := chrome.FitOverlayPopupHeight(popup, max(height-1, 1))
	composed, placement := chrome.OverlayPopupDimmedPlaced(base, fitted, width, height)
	placement.Whole = placement.Whole && fitted == popup
	return composed, placement
}

// placedOverlay is one popup as the last frame composited it.
type placedOverlay struct {
	renderer  widthRenderer
	placement chrome.OverlayPlacement
}

// overlayPlacement reports where the last frame drew renderer, if it did.
func (m *AppModel) overlayPlacement(renderer widthRenderer) (chrome.OverlayPlacement, bool) {
	for _, placed := range m.pointer.overlays {
		if placed.renderer == renderer {
			return placed.placement, true
		}
	}
	return chrome.OverlayPlacement{}, false
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
	if m.lenses.Mounted() {
		// First, so the sheet, prompt, and confirmation a lens action opens all
		// paint above the picker that raised them.
		out = append(out, m.lenses.Picker())
	}
	if m.operations.ActionSheet != nil {
		out = append(out, m.operations.ActionSheet)
	}
	if m.operations.CheckInEditor != nil {
		out = append(out, m.operations.CheckInEditor)
	}
	if m.operations.ValuePicker != nil {
		out = append(out, m.operations.ValuePicker)
	}
	if m.operations.TextPrompt != nil {
		out = append(out, m.operations.TextPrompt)
	}
	if m.conflictBrowser != nil {
		out = append(out, m.conflictBrowser)
	}
	return out
}

// renderConfirmDialogOverlay checks if an active confirm dialog should render
// as a floating overlay box.
func (m *AppModel) renderConfirmDialogOverlay() string {
	if overlay, armed := m.project.ConfirmOverlay(m.viewNav.width); armed {
		return overlay
	}
	if overlay, armed := m.taskMutation.confirmOverlay(m.viewNav.width); armed {
		return overlay
	}
	if overlay, armed := m.operations.ConfirmOverlay(m.viewNav.width); armed {
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
