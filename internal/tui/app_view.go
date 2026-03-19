package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *AppModel) View() string {
	if m.quitting {
		return ""
	}

	if m.tooNarrow {
		msg := fmt.Sprintf("Terminal too narrow (min %d columns)", minTerminalWidth)
		style := lipgloss.NewStyle().
			Width(m.width).
			Height(m.height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(ColorMuted)
		return m.applyBackground(style.Render(msg))
	}

	content := m.views.View(m.currentView())
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
	if view := m.views.Get(m.currentView()); view != nil {
		if caps, ok := view.(ViewCapabilities); ok && caps.IsFullHeight() {
			// Still overlay the semicolon prefix popup on full-height views
			if popup := m.renderSemicolonPrefixPopup(); popup != "" {
				content = OverlayPopup(content, popup, m.width, m.height)
			}
			return m.applyBackground(content)
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
		// Overlay prefix hints on top of quick-switch popup when prefix is pending
		if m.quickSwitchPendingPrefix != "" {
			prefixPopup := RenderPrefixPopup(m.quickSwitchPendingPrefix, PrefixHints{",": StatusPrefixHints, "!": PriorityPrefixHints}, m.width, targetContentHeight)
			content = OverlayPopup(content, prefixPopup, m.width, targetContentHeight)
		}
	}

	// Overlay project cycle popup if active (bottom-left to avoid covering sidebar)
	if popup := m.renderProjectCyclePopup(); popup != "" {
		content = OverlayPopupBottomLeft(content, popup, m.width, targetContentHeight)
	}

	// Replace base content with task preview when pinned popup is active
	if m.showPinnedPopup && len(m.pinnedPopupItems) > 0 {
		if preview := m.renderPinnedPreviewBackground(); preview != "" {
			content = preview
		}
	}

	// Overlay pinned tasks popup if active
	if popup := m.renderPinnedPopup(); popup != "" {
		content = OverlayPopupBottomLeft(content, popup, m.width, targetContentHeight)
		// Overlay prefix hints on top of pinned popup when prefix is pending
		if m.pinnedPendingPrefix != "" {
			prefixPopup := RenderPrefixPopup(m.pinnedPendingPrefix, PrefixHints{",": StatusPrefixHints, "!": PriorityPrefixHints}, m.width, targetContentHeight)
			content = OverlayPopupBottomLeft(content, prefixPopup, m.width, targetContentHeight)
		}
	}

	// Overlay confirm dialog last so it renders on top of all other popups
	if popup := m.renderConfirmDialogOverlay(); popup != "" {
		content = OverlayPopup(content, popup, m.width, targetContentHeight)
	}

	return m.applyBackground(content + "\n" + statusBar)
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

// applyBackground fills every line of content with the configured TUI
// background color so no terminal default leaks through. Inner lipgloss
// styles emit SGR resets (\x1b[0m) that clear the background mid-line,
// so we re-inject the background escape after every reset and pad each
// line to the full terminal width.
func (m *AppModel) applyBackground(content string) string {
	bg := m.config.TUIBackgroundColor
	if bg == "" {
		return content
	}
	bgEsc := hexToBgEsc(bg)
	if bgEsc == "" {
		return content
	}

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		// Re-apply background after every SGR reset within the line
		patched := bgEsc + strings.ReplaceAll(line, "\x1b[0m", "\x1b[0m"+bgEsc)
		// Pad to full terminal width
		pad := m.width - lipgloss.Width(line)
		if pad > 0 {
			patched += strings.Repeat(" ", pad)
		}
		lines[i] = patched + "\x1b[0m"
	}
	// Fill remaining vertical space with background-colored empty lines
	emptyLine := bgEsc + strings.Repeat(" ", m.width) + "\x1b[0m"
	for len(lines) < m.height {
		lines = append(lines, emptyLine)
	}
	return strings.Join(lines, "\n")
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

func (m *AppModel) renderStatusBar() string {
	// Top border separator for visual distinction
	separator := renderHorizontalDivider(m.width)

	// Helper to render left-aligned message with spacer
	renderLeftAlignedMsg := func(msg string) string {
		gap := max(m.width-lipgloss.Width(msg), 0)
		spacer := ThemeSpacer(gap)
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
		var msg string
		if m.statusIsInfo {
			msg = InfoStatusStyle.Render("· " + m.statusMessage)
		} else {
			msg = SuccessStyle.Render("✓ " + m.statusMessage)
		}
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
		left = StatusBarStyle.Render("Dashboard") + ": Run " + InfoStatusStyle.Render("tutor") + " command to start. Dismiss with " + InfoStatusStyle.Render("|")
	} else {
		left = StatusBarStyle.Render(status)
	}
	right := HelpStyle.Render(help)

	gap := max(m.width-lipgloss.Width(left)-lipgloss.Width(right), 1)

	spacer := ThemeSpacer(gap)

	return separator + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, left, spacer, right)
}
