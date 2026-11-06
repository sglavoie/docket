package tui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/theme"
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
	if m.commandPalette != nil {
		out = append(out, m.commandPalette)
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

// Switching views eases the new body in under a still title and footer: a
// motion session only, so the harness and Reduce Motion never see it. The body
// starts at viewFadeFloor rather than blank, so quick tabbing never flashes an
// empty screen, and the text and layout of every frame are the settled ones.
const (
	viewFadeFrame  = 32 * time.Millisecond
	viewFadeFrames = 5 // about a sixth of a second
	viewFadeFloor  = 0.3
	// viewFadeTitleRows is the title row every surface draws first; it names
	// where the user now is, so it changes at once rather than fading.
	viewFadeTitleRows = 1
)

// viewFade is the view switch being drawn: the frames it has left, counted
// rather than timed so a frame the terminal drew late is still drawn, and the
// generation that voids a superseded switch's ticks.
type viewFade struct {
	remaining int
	seq       int
}

// viewFadeTickMsg draws the next frame of the switch Seq names.
type viewFadeTickMsg struct{ Seq int }

// screenForMotion is what a view switch compares across one Update: the
// presentation on screen and, in the Outliner, its mode. A bounded panel names
// its host, which stays drawn, so opening or closing one is not a switch.
type screenForMotion struct {
	view presentation.PresentationID
	mode chrome.OutlinerModeID
}

func (m *AppModel) screenForMotion() screenForMotion {
	view := m.currentPresentationView()
	if frame, ok := m.hostedPanelFrame(); ok && frame.layout.HostDrawn {
		view = frame.host
	}
	return screenForMotion{view: view, mode: m.outlinerModeForMotion()}
}

// markViewFade starts the fade when an Update changed the view or the mode,
// whichever key or seam changed it. Like the pulse, the launch's first frame
// is an arrival rather than a switch, so it draws nothing.
func (m *AppModel) markViewFade(before screenForMotion, launchArmed bool) tea.Cmd {
	if !m.lifecycle.motion || launchArmed || m.lifecycle.launchFocusArmed() || m.screenForMotion() == before {
		return nil
	}
	m.viewFade.seq++
	m.viewFade.remaining = viewFadeFrames
	return viewFadeTick(m.viewFade.seq)
}

func (m *AppModel) advanceViewFade(msg viewFadeTickMsg) tea.Cmd {
	if msg.Seq != m.viewFade.seq || m.viewFade.remaining <= 0 {
		return nil
	}
	m.viewFade.remaining--
	if m.viewFade.remaining == 0 {
		return nil
	}
	return viewFadeTick(msg.Seq)
}

// stopViewFade drops a fade in progress, as turning motion off does, and the
// panel rise with it: both are an arrival a keystroke has already moved past.
func (m *AppModel) stopViewFade() {
	m.viewFade.seq++
	m.viewFade.remaining = 0
	m.panelRise.seq++
	m.panelRise.remaining = 0
}

// A bounded panel such as capture rises into its rows rather than appearing
// whole: its bottom row, where the caret is, is drawn at once, and the rows
// above it ease in from the background bottom first, so the panel reads as
// coming up from the terminal's floor. Like the view fade it is paint alone —
// every frame has the settled text and layout — and a motion session only.
const (
	panelRiseFrame  = 28 * time.Millisecond
	panelRiseFrames = 5
)

// panelRise is the panel arrival being drawn; see viewFade, which it mirrors.
type panelRise struct {
	remaining int
	seq       int
}

// panelRiseTickMsg draws the next frame of the rise Seq names.
type panelRiseTickMsg struct{ Seq int }

// markPanelRise starts the rise when an Update opened a bounded panel that was
// not open before it. Moving between two panels, or changing one's size, is
// not an arrival.
func (m *AppModel) markPanelRise(panelBefore, launchArmed bool) tea.Cmd {
	if !m.lifecycle.motion || launchArmed || m.lifecycle.launchFocusArmed() || panelBefore {
		return nil
	}
	if _, open := m.hostedPanelLayout(); !open {
		return nil
	}
	m.panelRise.seq++
	m.panelRise.remaining = panelRiseFrames
	return panelRiseTick(m.panelRise.seq)
}

func (m *AppModel) advancePanelRise(msg panelRiseTickMsg) tea.Cmd {
	if msg.Seq != m.panelRise.seq || m.panelRise.remaining <= 0 {
		return nil
	}
	m.panelRise.remaining--
	if m.panelRise.remaining == 0 {
		return nil
	}
	return panelRiseTick(msg.Seq)
}

func panelRiseTick(seq int) tea.Cmd {
	return status.TickCmd(panelRiseFrame, func(time.Time) tea.Msg { return panelRiseTickMsg{Seq: seq} })
}

// riseShare is how far the rise has come, eased out, from 0 on the panel's own
// frame to 1 once its last tick has run.
func (m *AppModel) riseShare() float64 {
	if !m.lifecycle.motion || m.panelRise.remaining <= 0 {
		return 1
	}
	t := float64(panelRiseFrames-m.panelRise.remaining) / panelRiseFrames
	return 1 - (1-t)*(1-t)
}

// risePanel draws the panel rows [top, bottom) of frame partway in: the bottom
// row whole, and each row above it revealed once the rising edge has passed
// it, over a soft band so the edge reads as a slope rather than a cut.
func (m *AppModel) risePanel(frame string, top, bottom int) string {
	share := m.riseShare()
	if share >= 1 || top < 0 || theme.ThemeBgColor == "" || m.lifecycle.colorDisabled {
		return frame
	}
	rows := bottom - top
	if rows <= 1 {
		return frame
	}
	lines := strings.Split(frame, "\n")
	bottom = min(bottom, len(lines))
	edge := share * float64(rows) * 1.5
	band := max(float64(rows)/2, 1)
	for y := top; y < bottom-1; y++ {
		distance := float64(bottom - 1 - y)
		reveal := min(max((edge-distance)/band, 0), 1)
		if reveal >= 1 {
			continue
		}
		lines[y] = fadeLines(lines[y], 0, 1, viewFadeFloor*0.3+(1-viewFadeFloor*0.3)*reveal)
	}
	return strings.Join(lines, "\n")
}

func viewFadeTick(seq int) tea.Cmd {
	return status.TickCmd(viewFadeFrame, func(time.Time) tea.Msg { return viewFadeTickMsg{Seq: seq} })
}

// viewFadeReveal is how far the new view has faded in, from viewFadeFloor on
// the switch's own frame to 1 once its last tick has run.
func (m *AppModel) viewFadeReveal() float64 {
	if !m.lifecycle.motion || m.viewFade.remaining <= 0 {
		return 1
	}
	t := float64(viewFadeFrames-m.viewFade.remaining) / viewFadeFrames
	eased := 1 - (1-t)*(1-t)*(1-t)
	return viewFadeFloor + (1-viewFadeFloor)*eased
}

// fadeViewBody draws the body of a backgrounded frame, the rows between the
// title and footerTop, partway between the background and their own colors.
// Every other row is returned byte for byte.
func (m *AppModel) fadeViewBody(frame string, footerTop int) string {
	reveal := m.viewFadeReveal()
	if reveal >= 1 || theme.ThemeBgColor == "" || m.lifecycle.colorDisabled {
		return frame
	}
	return fadeLines(frame, viewFadeTitleRows, footerTop, reveal)
}

// fadeLines blends every cell of lines [from, to) toward the theme background
// by 1-reveal. Cells are read back from the rendered line, so nested styles,
// the shell's background fill, and wide characters all keep their geometry.
func fadeLines(frame string, from, to int, reveal float64) string {
	background := lipgloss.Color(theme.ThemeBgColor)
	foreground := lipgloss.Color(theme.ColorNormalText)
	lines := strings.Split(frame, "\n")
	for y := max(from, 0); y < min(to, len(lines)); y++ {
		width := lipgloss.Width(lines[y])
		if width == 0 {
			continue
		}
		canvas := lipgloss.NewCanvas(width, 1).Compose(lipgloss.NewLayer(lines[y]))
		for x := range width {
			cell := canvas.CellAt(x, 0)
			if cell == nil || cell.Width == 0 {
				continue
			}
			fg, bg := cell.Style.Fg, cell.Style.Bg
			if fg == nil {
				fg = foreground
			}
			if bg == nil {
				bg = background
			}
			cell.Style.Fg = blendColor(background, fg, reveal)
			cell.Style.Bg = blendColor(background, bg, reveal)
		}
		lines[y] = canvas.Render()
	}
	return strings.Join(lines, "\n")
}
