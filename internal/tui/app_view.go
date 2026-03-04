package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *AppModel) View() string {
	if m.quitting {
		return ""
	}

	content := m.views.View(m.currentView())
	if content == "" {
		content = "Unknown view"
	}

	// Overlay alert indicator on the first row of main views
	if m.shouldShowAlertIndicator() {
		indicator := renderAlertIndicator(m.alertCount())
		content = overlayAlertIndicator(content, indicator, m.width)
	}

	// Full-height views render their own footer and skip the external status bar
	if view := m.views.Get(m.currentView()); view != nil {
		if caps, ok := view.(ViewCapabilities); ok && caps.IsFullHeight() {
			// Still overlay the semicolon prefix popup on full-height views
			if popup := m.renderSemicolonPrefixPopup(); popup != "" {
				content = OverlayPopup(content, popup, m.width, m.height)
			}
			return content
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

	// Overlay prefix popup if the current view provides one
	if popup := m.renderPrefixPopupOverlay(); popup != "" {
		content = OverlayPopup(content, popup, m.width, targetContentHeight)
	}

	// Overlay semicolon prefix popup if active (global, not view-specific)
	if popup := m.renderSemicolonPrefixPopup(); popup != "" {
		content = OverlayPopup(content, popup, m.width, targetContentHeight)
	}

	// Overlay quick-switch popup if active
	if popup := m.renderQuickSwitchPopup(); popup != "" {
		content = OverlayPopup(content, popup, m.width, targetContentHeight)
	}

	// Overlay project cycle popup if active
	if popup := m.renderProjectCyclePopup(); popup != "" {
		content = OverlayPopup(content, popup, m.width, targetContentHeight)
	}

	// Overlay pinned tasks popup if active
	if popup := m.renderPinnedPopup(); popup != "" {
		content = OverlayPopup(content, popup, m.width, targetContentHeight)
	}

	// Overlay confirm dialog last so it renders on top of all other popups
	if popup := m.renderConfirmDialogOverlay(); popup != "" {
		content = OverlayPopup(content, popup, m.width, targetContentHeight)
	}

	return content + "\n" + statusBar
}

// renderPrefixPopupOverlay checks if the current view has a pending prefix and
// renders the popup box for overlay compositing.
func (m *AppModel) renderPrefixPopupOverlay() string {
	view := m.views.Get(m.currentView())
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

// renderSemicolonPrefixPopup renders the ";" prefix popup when pendingSemicolonPrefix is true.
func (m *AppModel) renderSemicolonPrefixPopup() string {
	if !m.pendingSemicolonPrefix {
		return ""
	}
	hints := PrefixHints{";": SemicolonPrefixHints}
	return RenderPrefixPopup(";", hints, m.width, m.height)
}

// renderConfirmDialogOverlay checks if the current view has an active confirm
// dialog and renders it as a floating overlay box.
func (m *AppModel) renderConfirmDialogOverlay() string {
	view := m.views.Get(m.currentView())
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

func (m *AppModel) renderStatusBar() string {
	// Top border separator for visual distinction
	separator := renderHorizontalDivider(m.width)

	// Helper to render left-aligned message with spacer
	renderLeftAlignedMsg := func(msg string) string {
		gap := m.width - lipgloss.Width(msg)
		if gap < 0 {
			gap = 0
		}
		spacer := lipgloss.NewStyle().Width(gap).Render("")
		return lipgloss.JoinHorizontal(lipgloss.Top, msg, spacer)
	}

	// Show loading spinner if active
	if m.isLoading {
		spinnerView := m.spinner.View()
		msg := LoadingStyle.Render(spinnerView + " " + m.loadingMessage)
		return separator + "\n" + renderLeftAlignedMsg(msg)
	}

	// Show error message if present
	if m.err != nil {
		msg := ErrorStatusStyle.Render("✗ " + m.err.Error())
		return separator + "\n" + renderLeftAlignedMsg(msg)
	}

	// Show temporary status message if present
	if m.statusMessage != "" {
		msg := SuccessStyle.Render("✓ " + m.statusMessage)
		return separator + "\n" + renderLeftAlignedMsg(msg)
	}

	// Show pinned tasks bar if any tasks are pinned
	if pinnedBar := m.renderPinnedTasksBar(m.width); pinnedBar != "" {
		return separator + "\n" + pinnedBar
	}

	var status string
	if view := m.views.Get(m.currentView()); view != nil {
		if labeler, ok := view.(StatusBarLabeler); ok {
			status = labeler.StatusBarLabel()
		}
	}

	// Check if view supports help shortcut hint
	currentView := m.currentView()
	showHelp := true
	if view := m.views.Get(currentView); view != nil {
		if hs, ok := view.(HelpShortcutSupporter); ok {
			showHelp = hs.SupportsHelpShortcut()
		}
	}

	help := ""
	if showHelp && m.isNotesFullscreen() {
		showHelp = false
	}
	if showHelp {
		help = "Press : for commands, ? for help"
	}

	var left string
	if m.currentView() == ViewDashboard && !m.config.GetTutorDismissed() {
		blue := lipgloss.NewStyle().Foreground(ColorPrimary)
		left = StatusBarStyle.Render("Dashboard") + ": Run " + blue.Render("tutor") + " command to start. Dismiss with " + blue.Render("|")
	} else {
		left = StatusBarStyle.Render(status)
	}
	right := HelpStyle.Render(help)

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}

	spacer := lipgloss.NewStyle().Width(gap).Render("")

	return separator + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, left, spacer, right)
}
