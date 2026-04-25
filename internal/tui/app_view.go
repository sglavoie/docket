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

	resultView := m.views.View(m.currentView())
	content := resultView.Content
	if content == "" {
		content = "Unknown view"
	}

	// Replace base content with task preview when cycle overlay is active
	if m.showProjectCycle && len(m.projectCycleItems) > 0 {
		if preview := m.renderCyclePreviewBackground(); preview != "" {
			content = preview
		}
	}

	// Overlay alert indicator on the first row of main views
	if m.shouldShowAlertIndicator() {
		indicator := renderAlertIndicator(m.alertCount())
		content = overlayAlertIndicator(content, indicator, m.width)
	}

	// Full-height views render their own footer and skip the external status bar
	if currentView := m.views.Get(m.currentView()); currentView != nil {
		if caps, ok := currentView.(ViewCapabilities); ok && caps.IsFullHeight() {
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
	if contentHeight < targetContentHeight {
		content += strings.Repeat("\n", targetContentHeight-contentHeight)
	}

	content = m.applyOverlays(content, targetContentHeight)

	resultView.SetContent(m.applyBackground(content + "\n" + statusBar))
	return resultView
}
