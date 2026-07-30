package tui

import (
	"fmt"
	"runtime/debug"
	"strings"

	tea "charm.land/bubbletea/v2"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/logging"
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

	if m.tooNarrow || m.tooShort {
		msg := fmt.Sprintf("Terminal too small (need %d×%d, have %d×%d)",
			minTerminalWidth, minTerminalHeight, m.width, m.height)
		style := lipgloss.NewStyle().
			Width(m.width).
			Height(m.height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(ColorMuted)
		return tea.NewView(m.applyBackground(style.Render(msg)))
	}

	if current := m.activeView(); current != nil {
		if injector, ok := current.(openLoopsAware); ok {
			injector.setOpenLoops(m.openLoops)
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
		if caps, ok := currentView.(ViewCapabilities); ok && caps.IsFullHeight() {
			inZen := false
			if zenView, ok := currentView.(ZenModeView); ok {
				inZen = zenView.IsZenMode()
			}
			if !inZen && (m.statusMessage != "" || m.err != nil || m.isLoading || m.todayClearedBanner != "") {
				content = replaceFooter(content, m.renderStatusBar())
			}
			content = m.applyOverlays(content, m.height)
			resultView.SetContent(m.applyBackground(content))
			return resultView
		}
	}

	// Add status bar
	statusBar := m.renderStatusBar()
	statusBarHeight := lipgloss.Height(statusBar)

	// Pad content so the status bar is always at the bottom of the terminal
	contentHeight := lipgloss.Height(content)
	targetContentHeight := m.height - statusBarHeight
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

// replaceFooter swaps the last 2 lines of content (a full-height view's own
// footer: separator + help text) with newFooter (the AppModel status bar).
func replaceFooter(content, newFooter string) string {
	lines := strings.Split(content, "\n")
	if len(lines) < 2 {
		return content
	}
	lines = lines[:len(lines)-2]
	return strings.Join(lines, "\n") + "\n" + newFooter
}
