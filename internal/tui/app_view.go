package tui

import (
	"fmt"
	"runtime/debug"
	"strings"

	tea "charm.land/bubbletea/v2"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/pulse"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/theme"
)

func (m *AppModel) View() (result tea.View) { //nolint:gocyclo
	defer func() {
		result.AltScreen = true
		result.KeyboardEnhancements.ReportEventTypes = true
		if r := recover(); r != nil {
			stack := debug.Stack()
			logging.Warn("TUI panic in View: %v\n%s", r, stack)
			result = tea.NewView(fmt.Sprintf("internal error: %v\n\nPress ctrl+c to quit.", r))
			result.AltScreen = true
		}
	}()

	if m.quitting {
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
	// Exception: when a transient status message is active (clipboard feedback,
	// error, loading), replace the view's footer with the AppModel status bar.
	// Further exception: zen mode suppresses even transient status overlays.
	if currentView := m.activeView(); currentView != nil {
		if caps, ok := currentView.(route.ViewCapabilities); ok && caps.IsFullHeight() {
			inZen := false
			if zenView, ok := currentView.(route.ZenModeView); ok {
				inZen = zenView.IsZenMode()
			}
			if !inZen && (m.feedback.message != "" || m.feedback.err != nil) {
				content = replaceFooter(content, m.renderStatusBar())
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
	contentHeight := lipgloss.Height(content)
	targetContentHeight := m.viewNav.height - statusBarHeight
	if contentHeight > targetContentHeight {
		content = clipContentHeight(content, targetContentHeight)
		contentHeight = lipgloss.Height(content)
	}
	if contentHeight < targetContentHeight {
		content += strings.Repeat("\n", targetContentHeight-contentHeight)
	}

	content = m.applyOverlays(content, targetContentHeight)

	join := "\n"
	if strings.HasSuffix(content, "\n") {
		join = ""
	}
	resultView.SetContent(m.applyBackground(content + join + statusBar))
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

// replaceFooter swaps a full-height view's own footer with the AppModel status
// bar. Structured guidance may occupy one or two rows, so locate its divider
// instead of assuming the legacy fixed two-line shape.
func replaceFooter(content, newFooter string) string {
	lines := strings.Split(content, "\n")
	if len(lines) < 2 {
		return content
	}
	footerStart := -1
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
			footerStart = index
			break
		}
	}
	if footerStart < 0 {
		footerStart = len(lines) - 2
	}
	lines = lines[:footerStart]
	return strings.Join(lines, "\n") + "\n" + newFooter
}
