package tui

import (
	"fmt"
	"image/color"
	"runtime/debug"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/help"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/tui/pulse"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/taskform"
	"github.com/sglavoie/tt/internal/tui/theme"
)

func (m *AppModel) View() (result tea.View) { //nolint:gocyclo
	defer func() {
		result.AltScreen = true
		// Keep Bubble Tea's default key disambiguation, but do not request
		// releases: we do not use them, and the pinned decoder mistakes CSI ~
		// releases (including F10) for another press.
		// Focus reports are what let a long-lived session notice a workspace
		// changed elsewhere while it sat in the background.
		result.ReportFocus = true
		// A view naming its own title keeps it; otherwise the title is the
		// pulse, which tt.app reads back for its Dock badge.
		if !m.lifecycle.quitting && result.WindowTitle == "" {
			result.WindowTitle = pulse.WindowTitle(m.workspacePulse)
		}
		result.BackgroundColor = m.terminalBackground()
		if r := recover(); r != nil {
			stack := debug.Stack()
			logging.Warn("TUI panic in View: %v\n%s", r, stack)
			result = tea.NewView(fmt.Sprintf("internal error: %v\n\nPress ctrl+c to quit.", r))
			result.AltScreen = true
		}
	}()

	if m.lifecycle.quitting {
		return tea.NewView("")
	}

	if m.viewNav.tooSmall() {
		msg := fmt.Sprintf("Terminal too small (need %d×%d, have %d×%d)",
			chrome.MinTerminalWidth, chrome.MinTerminalHeight, m.viewNav.width, m.viewNav.height)
		style := lipgloss.NewStyle().
			Width(m.viewNav.width).
			Height(m.viewNav.height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(theme.ColorMuted)
		return tea.NewView(m.applyBackground(style.Render(msg)))
	}

	if current := m.activeView(); current != nil {
		policy, classified := route.ChromePolicyForPresentation(m.currentPresentationView())
		if injector, ok := current.(pulse.WorkspacePulseAware); classified && policy.WorkspacePulseEligible() && ok {
			injector.SetWorkspacePulse(m.workspacePulse)
		}
		if injector, ok := current.(pulse.CompletionActivityAware); classified && policy.WorkspacePulseEligible() && ok {
			injector.SetCompletionActivity(m.completionActivity)
		}
	}
	resultView := tea.NewView("")
	if current := m.activeView(); current != nil {
		resultView = current.View()
	}
	content := resultView.Content
	if content == "" {
		content = "Unknown view"
	}

	// Full-height views render their own footer and skip the external status bar.
	// The shell still owns feedback, so it is given a row of its own directly
	// above that footer rather than handed the footer's place: a message about the
	// last keystroke may not be the reason the keys for the next one go unnamed.
	// Zen mode suppresses even that row.
	if currentView := m.activeView(); currentView != nil {
		if caps, ok := currentView.(route.ViewCapabilities); ok && caps.IsFullHeight() {
			inZen := false
			if zenView, ok := currentView.(route.ZenModeView); ok {
				inZen = zenView.IsZenMode()
			}
			if !inZen {
				content = composeSurfaceFooterFeedback(content, m.footerFeedbackLine())
			}
			content = m.applyOverlays(content, m.viewNav.height)
			resultView.SetContent(m.applyBackground(content))
			return resultView
		}
	}

	// Add status bar
	statusBar := m.renderStatusBar()
	statusBarHeight := lipgloss.Height(statusBar)

	// Pad content so the status bar is always at the bottom of the terminal
	targetContentHeight := m.viewNav.height - statusBarHeight
	content = m.composeHostedPanelContent(content, targetContentHeight)
	contentHeight := lipgloss.Height(content)
	if contentHeight > targetContentHeight {
		content = clipContentHeight(content, targetContentHeight)
		contentHeight = lipgloss.Height(content)
	}
	if contentHeight < targetContentHeight {
		content += strings.Repeat("\n", targetContentHeight-contentHeight)
	}

	content = m.applyOverlays(content, targetContentHeight)

	// One separator, always. A trailing newline is the last content row rather
	// than a spare one — lipgloss.Height counts it, and so did the padding above —
	// so joining without it hands the footer that row and leaves the frame a row
	// short of the terminal, with the footer floating above a blank bottom line.
	resultView.SetContent(m.applyBackground(content + "\n" + statusBar))
	return resultView
}

// clipContentHeight prevents a renderer from pushing the shell-owned status bar
// beyond the terminal boundary. Renderers are sized against the usual two-row
// footer, but contextual footers may be taller; the shell remains the final
// authority for the composed frame height.
func clipContentHeight(content string, maxRows int) string {
	if maxRows < 1 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if len(lines) <= maxRows {
		return content
	}
	return strings.Join(lines[:maxRows], "\n")
}

// composeSurfaceFooterFeedback puts the shell's feedback line into a view that
// draws its own footer. The footer is left exactly where it was and says exactly
// what it said: the feedback is inserted immediately above its divider and paid
// for out of the body rows directly above it, so the frame keeps its height and
// the surface keeps every control it published.
//
// This is the surface-owned half of the same composition the shell does for
// itself. Both spend feedback out of the content area rather than out of the
// controls, and neither reports an act by removing the way to undo it.
//
// A view whose footer cannot be located, or whose body has no row to spend,
// keeps its frame untouched: the message is transient and the controls are not,
// so a frame that cannot afford both keeps the one that persists.
func composeSurfaceFooterFeedback(content, feedback string) string {
	if feedback == "" {
		return content
	}
	lines := strings.Split(content, "\n")
	footerStart := surfaceFooterDividerIndex(lines)
	if footerStart < 1 {
		return content
	}
	rows := strings.Split(feedback, "\n")
	if len(rows) > footerStart {
		rows = rows[:footerStart]
	}
	composed := make([]string, 0, len(lines))
	composed = append(composed, lines[:footerStart-len(rows)]...)
	composed = append(composed, rows...)
	composed = append(composed, lines[footerStart:]...)
	return strings.Join(composed, "\n")
}

// surfaceFooterDividerIndex locates the divider a view's own footer opens with,
// scanning up from the bottom. Structured guidance may occupy one or two rows,
// so the divider is found rather than the shape assumed. It returns -1 when the
// content has no footer to find, which is the caller's signal to leave the frame
// alone rather than to guess at a row.
func surfaceFooterDividerIndex(lines []string) int {
	if len(lines) < 2 {
		return -1
	}
	for index := len(lines) - 2; index >= 0; index-- {
		plain := strings.TrimSpace(ansi.Strip(lines[index]))
		if plain == "" {
			continue
		}
		divider := true
		for _, cell := range plain {
			if cell != '─' {
				divider = false
				break
			}
		}
		if divider {
			return index
		}
	}
	return -1
}

// applyBackground fills every line of content with the configured TUI
// background color so no terminal default leaks through. Inner lipgloss
// styles emit SGR resets (\x1b[m or \x1b[0m) that clear the background
// mid-line, so we re-inject the background escape after every reset and
// pad each line to the full terminal width.
//
// When a NormalText palette color is configured, the default foreground is
// also injected alongside the background so unstyled text (e.g. plain
// numeric counts in section headers) renders with the theme's body color
// instead of the terminal's default foreground.
//
// lipgloss v2 emits \x1b[m (empty-param reset) rather than \x1b[0m, so
// both forms must be handled.
func (m *AppModel) applyBackground(content string) string {
	bg := theme.ThemeBgColor
	if bg == "" {
		return content
	}
	bgEsc := hexToBgEsc(bg)
	if bgEsc == "" {
		return content
	}
	fillEsc := bgEsc + hexToFgEsc(theme.ColorNormalText)

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		// Re-apply background (and normal foreground, when set) after every
		// SGR reset within the line. lipgloss v2 emits \x1b[m (empty-param);
		// handle \x1b[0m too.
		patched := fillEsc + strings.ReplaceAll(line, "\x1b[m", "\x1b[m"+fillEsc)
		patched = strings.ReplaceAll(patched, "\x1b[0m", "\x1b[0m"+fillEsc)
		// Pad to full terminal width
		pad := m.viewNav.width - lipgloss.Width(line)
		if pad > 0 {
			patched += strings.Repeat(" ", pad)
		}
		lines[i] = patched + "\x1b[m"
	}
	// Fill remaining vertical space with background-colored empty lines
	emptyLine := bgEsc + strings.Repeat(" ", m.viewNav.width) + "\x1b[0m"
	for len(lines) < m.viewNav.height {
		lines = append(lines, emptyLine)
	}
	return strings.Join(lines, "\n")
}

// terminalBackground is the theme background announced to the terminal as its
// default (OSC 11), so the terminal's padding — and a host window that follows
// it, as tt.app's does — matches the cells applyBackground paints. Bubble Tea
// restores the terminal's own color when the program exits, and nil, on the
// way out or under --no-color, restores it at once.
func (m *AppModel) terminalBackground() color.Color {
	if m.lifecycle.quitting || m.lifecycle.colorDisabled || theme.ThemeBgColor == "" {
		return nil
	}
	return theme.ColorBackground
}

// hexToBgEsc converts a #rrggbb hex color to an ANSI true-color background
// escape sequence.
func hexToBgEsc(hex string) string {
	if len(hex) != 7 || hex[0] != '#' {
		return ""
	}
	r, err1 := strconv.ParseInt(hex[1:3], 16, 64)
	g, err2 := strconv.ParseInt(hex[3:5], 16, 64)
	b, err3 := strconv.ParseInt(hex[5:7], 16, 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return ""
	}
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b)
}

// hexToFgEsc converts a #rrggbb hex color to an ANSI true-color foreground
// escape sequence. Returns an empty string for unset or malformed input.
func hexToFgEsc(hex string) string {
	if len(hex) != 7 || hex[0] != '#' {
		return ""
	}
	r, err1 := strconv.ParseInt(hex[1:3], 16, 64)
	g, err2 := strconv.ParseInt(hex[3:5], 16, 64)
	b, err3 := strconv.ParseInt(hex[5:7], 16, 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return ""
	}
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r, g, b)
}

// handleWindowSize updates dimensions for all views.
func (m *AppModel) handleWindowSize(msg tea.WindowSizeMsg) {
	// All views (including full-height ones) are sized to height-2, matching the
	// open path: showView's full-height SetSize is immediately clobbered by
	// ViewRegistry.Register re-applying the registry's stored height-2. Views
	// self-pad their render to the terminal height, so the height-2 budget is
	// what every render is tuned against. Previously this loop re-applied the
	// full msg.Height to full-height views, diverging from the open path and
	// overflowing views (such as Notes) that size against their budget
	// rather than self-padding. Regression coverage: the *-resize-minimum-layout
	// scenarios under docs/tui-testing/scenarios/ (run by TestExampleScenariosPass).
	m.viewNav.resize(msg.Width, msg.Height)
	// The hub and the composer are registry-external renderers, so the
	// controller's registry sweep does not reach them.
	viewWidth, viewHeight := m.viewNav.viewSize()
	if m.manageHub != nil {
		m.manageHub.SetSize(viewWidth, viewHeight)
	}
	if m.taskComposer != nil {
		m.taskComposer.SetSize(viewWidth, viewHeight)
	}
}

// delegateToCurrentView forwards messages to the active view model.
func (m *AppModel) delegateToCurrentView(msg tea.Msg) tea.Cmd { //nolint:gocyclo
	if m.activeTaskComposer() {
		updated, cmd := m.taskComposer.Update(msg)
		if composer, ok := updated.(*taskform.TaskComposerModel); ok {
			m.taskComposer = composer
			m.viewNav.views.Register(presentation.ViewTaskComposer, composer)
		}
		return cmd
	}
	if m.activeManageHub() {
		updated, cmd := m.manageHub.Update(msg)
		if hub, ok := updated.(*projecthub.ManageHubModel); ok {
			m.manageHub = hub
		}
		return cmd
	}
	currentView := m.currentView()
	cmd := m.viewNav.views.Update(currentView, msg)

	// Keep typed pointers in sync for views that need direct access
	switch currentView {
	case presentation.ViewHelp:
		if v := m.viewNav.views.Get(presentation.ViewHelp); v != nil {
			m.helpView = v.(*help.HelpViewModel)
		}
	case presentation.ViewCrossProjectOutliner:
		if v, ok := m.viewNav.views.Get(presentation.ViewCrossProjectOutliner).(*outliner.OutlinerModel); ok {
			m.crossOutliner = v
		}
	default:
	}

	return cmd
}

// getCurrentCursorTaskAndProject returns the task and project under the cursor in
// the current primary view, or (nil, nil) when no task is selected. The project
// pointer enables overlays that need project context.
func (m *AppModel) getCurrentCursorTaskAndProject() (*models.Task, *models.Project) {
	if hub, _, ok := m.manageHubPrimary(); ok && m.activeManageHub() {
		return hub.GetTaskRef()
	}
	type taskRefProvider interface {
		GetTaskRef() (*models.Task, *models.Project)
	}
	if v, ok := m.currentViewAsModel().(taskRefProvider); ok {
		return v.GetTaskRef()
	}
	return nil, nil
}

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

// The rest of this file is the shell's side of a bounded bottom panel. It sits
// with the composition rather than in a file of its own because handing a
// presentation fewer rows and then drawing it inside them is one act, not two:
// the same arithmetic decides what the host is sized to and what the frame puts
// above the panel, and splitting it would leave the root shell carrying the
// same concern twice.

// hostedPanelHostRecord names the presentation the shell shrank for a bounded
// bottom panel and the budget it handed it. The record exists for the release:
// once the panel closes, only one view is owed its whole content area back, and
// a sweep over the registry would reset the scroll window of every other surface
// to say so.
type hostedPanelHostRecord struct {
	view   presentation.PresentationID
	height int
	active bool
}

// sizedView is satisfied by every list surface through its shared base. The
// shell reads the host's own size rather than trusting what it last handed it,
// so a re-registration that resets the host to the full budget is corrected on
// the next update instead of silently clipping the host's cursor row away.
type sizedView interface {
	Width() int
	Height() int
}

// hostedPanelFrame is the active view's panel layout together with the
// presentation it is drawn over: the entry directly beneath the panel's own
// overlay in the derived path, so a panel is hosted by whatever raised it rather
// than by a hardcoded surface.
type hostedPanelFrame struct {
	layout chrome.HostedPanelLayout
	host   presentation.PresentationID
	view   route.View
}

// hostedPanelLayout is what the active view says about its own presentation,
// read before any host is resolved. The panel sizes itself from this alone, so
// the shell has to honor it even when there is nothing to draw underneath.
func (m *AppModel) hostedPanelLayout() (chrome.HostedPanelLayout, bool) {
	active := m.activeView()
	if active == nil {
		return chrome.HostedPanelLayout{}, false
	}
	panel, ok := active.(route.HostedPanelView)
	if !ok {
		return chrome.HostedPanelLayout{}, false
	}
	layout := panel.HostedPanelLayout(m.viewNav.viewSize())
	return layout, layout.Active()
}

func (m *AppModel) hostedPanelFrame() (hostedPanelFrame, bool) {
	layout, ok := m.hostedPanelLayout()
	if !ok {
		return hostedPanelFrame{}, false
	}
	hostID, host, ok := m.hostedPanelHostView()
	if !ok {
		return hostedPanelFrame{}, false
	}
	return hostedPanelFrame{layout: layout, host: hostID, view: host}, true
}

// hostedPanelHostView resolves the renderer one step below the top of the
// derived presentation path. The hub is registry-external, so it answers for
// itself before the registry is consulted.
func (m *AppModel) hostedPanelHostView() (presentation.PresentationID, route.View, bool) {
	path := m.presentationPath()
	if len(path) < 2 {
		return 0, nil, false
	}
	hostID := path[len(path)-2]
	if view := m.presentationViewFor(hostID); view != nil {
		return hostID, view, true
	}
	return 0, nil, false
}

// presentationViewFor resolves a renderer by its presentation identity. It is
// how both budget-handing seams find the view they last spoke to: the registry
// is keyed that way and the hub, being registry-external, answers for itself
// first.
func (m *AppModel) presentationViewFor(view presentation.PresentationID) route.View {
	if m.manageHub != nil {
		if hub, ok := m.manageHub.PresentationView(); ok && hub == view {
			return m.manageHub
		}
	}
	if m.viewNav.views == nil {
		return nil
	}
	return m.viewNav.views.Get(view)
}

// composeHostedPanelContent stacks the panel under the presentation it was
// opened from. The host is re-rendered inside the rows the panel left it rather
// than covered by it, which is the whole reason the shell resizes it: a panel
// drawn on top of a full-height host would hide the row the host's cursor is on.
func (m *AppModel) composeHostedPanelContent(panel string, contentHeight int) string {
	layout, ok := m.hostedPanelLayout()
	if !ok {
		return panel
	}
	hostRows := contentHeight - lipgloss.Height(panel)
	if hostRows < 1 {
		return panel
	}
	frame, hosted := m.hostedPanelFrame()
	if !hosted || !layout.HostDrawn {
		// The panel asked for its share of the area whatever the shell found
		// above it, so the rows it declined are blank rather than its own: a
		// bottom panel that floated to the top row would read as a full view.
		return chrome.ComposeHostedPanel("", panel, hostRows)
	}
	return chrome.ComposeHostedPanel(frame.view.View().Content, panel, hostRows)
}

// syncHostedPanelHost hands the presentation under a bottom panel the budget the
// panel left it, and hands it back when the panel closes. It runs once per
// update rather than once per frame because a view resized on every frame is a
// view that never keeps a scroll window between two of them.
func (m *AppModel) syncHostedPanelHost() {
	frame, ok := m.hostedPanelFrame()
	if !ok {
		m.releaseHostedPanelHost()
		return
	}
	if m.hostedPanelHost.active && m.hostedPanelHost.view != frame.host {
		m.releaseHostedPanelHost()
	}
	width, _ := m.viewNav.viewSize()
	applied := m.hostedPanelHost
	m.hostedPanelHost = hostedPanelHostRecord{view: frame.host, height: frame.layout.HostRows, active: true}
	if sized, ok := frame.view.(sizedView); ok {
		if sized.Width() == width && sized.Height() == frame.layout.HostRows {
			return
		}
	} else if applied.active && applied.view == frame.host && applied.height == frame.layout.HostRows {
		// A host that cannot report its own size is taken at the record's word:
		// it still holds the budget this shell last handed it.
		return
	}
	applyHostedBudget(frame.view, width, frame.layout.HostRows)
}

func (m *AppModel) releaseHostedPanelHost() {
	if !m.hostedPanelHost.active {
		return
	}
	released := m.hostedPanelHost.view
	m.hostedPanelHost = hostedPanelHostRecord{}
	if view := m.presentationViewFor(released); view != nil {
		width, height := m.viewNav.viewSize()
		releaseHostedBudget(view, width, height)
	}
}

// applyHostedBudget and releaseHostedBudget are the two halves of the loan a
// bounded panel takes out of its host's content area. They exist because the
// shell has one act to perform and the host has two ways of answering it: a
// surface that keeps a scroll window says so through route.HostedBudgetView and
// is told which of the two this is, and every other surface is resized the way
// it always was.
func applyHostedBudget(view route.View, width, height int) {
	if budgeted, ok := view.(route.HostedBudgetView); ok {
		budgeted.SetHostedBudget(width, height)
		return
	}
	view.SetSize(width, height)
}

func releaseHostedBudget(view route.View, width, height int) {
	if budgeted, ok := view.(route.HostedBudgetView); ok {
		budgeted.ReleaseHostedBudget(width, height)
		return
	}
	view.SetSize(width, height)
}

// shellFrameRecipient names the presentation the shell last handed a footer
// budget and what it handed it. It exists for the same reason the hosted-panel
// record does: exactly one view is drawing under this footer, and a sweep over
// the registry would tell every other surface to reserve rows it is not paying
// for.
type shellFrameRecipient struct {
	view    presentation.PresentationID
	metrics chrome.ShellFrameMetrics
	active  bool
}

// syncShellFrameMetrics measures the footer this frame will draw and hands the
// active view what it costs beyond the two rows every view is sized against.
//
// It runs once per update rather than once per frame for the reason
// syncHostedPanelHost does: a budget delivered mid-render would make the paint
// depend on the order the frame was assembled in, and the measurement it carries
// is of a footer that has not been composed yet.
//
// Only the active view is told, and the one that stops being active is handed
// the zero value. A host drawn under a bounded panel is not the view paying for
// the footer — the panel above it is — so telling both would reserve the same
// rows twice and leave a band of blank content above the divider.
func (m *AppModel) syncShellFrameMetrics() {
	view := m.currentPresentationView()
	if m.shellFrame.active && m.shellFrame.view != view {
		m.releaseShellFrameMetrics()
	}
	aware, ok := m.activeView().(route.ShellFrameAware)
	if !ok {
		return
	}
	metrics := m.shellFrameMetrics()
	if m.shellFrame.active && m.shellFrame.view == view && m.shellFrame.metrics == metrics {
		return
	}
	m.shellFrame = shellFrameRecipient{view: view, metrics: metrics, active: true}
	aware.SetShellFrameMetrics(metrics)
}

func (m *AppModel) releaseShellFrameMetrics() {
	if !m.shellFrame.active {
		return
	}
	released := m.shellFrame.view
	m.shellFrame = shellFrameRecipient{}
	if aware, ok := m.presentationViewFor(released).(route.ShellFrameAware); ok {
		aware.SetShellFrameMetrics(chrome.ShellFrameMetrics{})
	}
}
