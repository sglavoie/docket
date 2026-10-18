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
	// A hosted panel draws no title, so the pulse on screen is its host's, and
	// the host is not the active view: it is handed the current one here or it
	// keeps the count it held when the panel opened.
	if frame, ok := m.hostedPanelFrame(); ok && frame.layout.HostDrawn {
		policy, classified := route.ChromePolicyForPresentation(frame.host)
		if injector, ok := frame.view.(pulse.WorkspacePulseAware); classified && policy.WorkspacePulseEligible() && ok {
			injector.SetWorkspacePulse(m.workspacePulse)
		}
		if injector, ok := frame.view.(pulse.CompletionActivityAware); classified && policy.WorkspacePulseEligible() && ok {
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
				content = fitSurfaceFooterHeight(content, m.viewNav.height)
				content = composeSurfaceFooterFeedback(content, m.footerFeedbackLine())
			}
			content = m.applyOverlays(content, m.viewNav.height, noPanelDivider)
			resultView.SetContent(m.applyBackground(content))
			return resultView
		}
	}

	// Add status bar
	statusBar := m.renderStatusBar()
	statusBarHeight := lipgloss.Height(statusBar)

	// Pad content so the status bar is always at the bottom of the terminal
	targetContentHeight := m.viewNav.height - statusBarHeight
	content, panelDivider := m.composeHostedPanelContent(content, targetContentHeight)
	contentHeight := lipgloss.Height(content)
	if contentHeight > targetContentHeight {
		content = clipContentHeight(content, targetContentHeight)
		contentHeight = lipgloss.Height(content)
	}
	if contentHeight < targetContentHeight {
		content += strings.Repeat("\n", targetContentHeight-contentHeight)
	}

	if layout, ok := m.hostedPanelLayout(); ok && layout.FooterAbove {
		// Capture keeps its editor at the terminal's floor. Insert the footer
		// before the panel so neither changing controls nor transient feedback
		// can lift the draft. Overlay coordinates use this final arrangement.
		lines := strings.Split(content, "\n")
		above := strings.Join(lines[:panelDivider], "\n")
		panel := strings.Join(lines[panelDivider:], "\n")
		content = statusBar + "\n" + panel
		if panelDivider > 0 {
			content = above + "\n" + content
		}
		panelDivider += statusBarHeight
		content = m.applyOverlays(content, m.viewNav.height, panelDivider)
		resultView.SetContent(m.applyBackground(content))
		return resultView
	}

	content = m.applyOverlays(content, targetContentHeight, panelDivider)

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

// fitSurfaceFooterHeight anchors a surface-owned footer to the terminal's last
// row. Registered views receive the ordinary content budget (height minus the
// shell footer); return those reserved rows above their own footer, preserving
// the body's scroll position and every control. Oversized bodies are clipped
// before the footer rather than clipping the controls themselves.
func fitSurfaceFooterHeight(content string, height int) string {
	lines := strings.Split(content, "\n")
	start := surfaceFooterDividerIndex(lines)
	if start < 0 || height < len(lines)-start {
		return content
	}
	foot := lines[start:]
	bodyHeight := height - len(foot)
	body := append([]string(nil), lines[:min(start, bodyHeight)]...)
	for len(body) < bodyHeight {
		body = append(body, "")
	}
	return strings.Join(append(body, foot...), "\n")
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
