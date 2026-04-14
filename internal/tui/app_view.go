package tui

import (
	"fmt"
	"runtime/debug"
	"strconv"
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

// applyOverlays composites all popup/overlay layers onto the base content.
// h is the overlay positioning height (m.height for full-height views,
// targetContentHeight for views with an external status bar).
func (m *AppModel) applyOverlays(content string, h int) string {
	// Overlay prefix popup if the current view provides one
	if popup := m.renderPrefixPopupOverlay(); popup != "" {
		content = OverlayPopup(content, popup, m.width, h)
	}

	// Overlay semicolon prefix popup if active (global, not view-specific)
	if popup := m.renderSemicolonPrefixPopup(); popup != "" {
		content = OverlayPopupDimmed(content, popup, m.width, h)
	}

	// Overlay quick-switch popup if active
	if popup := m.renderQuickSwitchPopup(); popup != "" {
		content = OverlayPopup(content, popup, m.width, h)
		// Overlay prefix hints on top of quick-switch popup when prefix is pending
		if m.quickSwitchPendingPrefix != "" {
			var qsPriority int
			if m.quickSwitchCursor >= 0 && m.quickSwitchCursor < m.quickSwitchFilteredLen() {
				realIdx := m.quickSwitchFiltered[m.quickSwitchCursor]
				qsPriority = m.recentTaskVisits[realIdx].Task.Priority
			}
			prefixPopup := RenderPrefixPopup(m.quickSwitchPendingPrefix, PrefixHints{",": StatusPrefixHints, "!": BuildPriorityPrefixHints(qsPriority)}, m.width, h)
			content = OverlayPopup(content, prefixPopup, m.width, h)
		}
	}

	// Overlay project cycle popup if active (bottom-left to avoid covering sidebar)
	if popup := m.renderProjectCyclePopup(); popup != "" {
		content = OverlayPopupBottomLeftDimmed(content, popup, m.width, h)
	}

	// Replace base content with task preview when pinned popup is active
	if m.showPinnedPopup && len(m.pinnedPopupItems) > 0 {
		if preview := m.renderPinnedPreviewBackground(); preview != "" {
			content = preview
		}
	}

	// Overlay pinned tasks popup if active
	if popup := m.renderPinnedPopup(); popup != "" {
		content = OverlayPopupBottomLeft(content, popup, m.width, h)
		// Overlay prefix hints on top of pinned popup when prefix is pending
		if m.pinnedPendingPrefix != "" {
			var pinnedPriority int
			if m.pinnedPopupCursor >= 0 && m.pinnedPopupCursor < m.pinnedPopupFilteredLen() {
				realIdx := m.pinnedPopupFiltered[m.pinnedPopupCursor]
				pinnedPriority = m.pinnedPopupItems[realIdx].Task.Priority
			}
			prefixPopup := RenderPrefixPopup(m.pinnedPendingPrefix, PrefixHints{",": StatusPrefixHints, "!": BuildPriorityPrefixHints(pinnedPriority)}, m.width, h)
			content = OverlayPopupBottomLeft(content, prefixPopup, m.width, h)
		}
	}

	// Overlay global project picker if active
	if popup := m.renderGlobalProjectPicker(); popup != "" {
		content = OverlayPopupDimmed(content, popup, m.width, h)
	}

	// Overlay global task picker if active
	if popup := m.renderGlobalTaskPicker(); popup != "" {
		content = OverlayPopupDimmed(content, popup, m.width, h)
	}

	// Overlay status quick picker if active
	if popup := m.renderStatusQuickPicker(); popup != "" {
		content = OverlayPopupDimmed(content, popup, m.width, h)
	}

	// Overlay section quick picker if active (step 4 of global task move flow)
	if popup := m.renderSectionQuickPicker(); popup != "" {
		content = OverlayPopupDimmed(content, popup, m.width, h)
	}

	// Overlay link type quick picker if active (step 3 of global ;c linked task flow)
	if popup := m.renderLinkTypeQuickPicker(); popup != "" {
		content = OverlayPopupDimmed(content, popup, m.width, h)
	}

	// Overlay link mode quick picker if active (step 4 of global ;c linked task flow)
	if popup := m.renderLinkModeQuickPicker(); popup != "" {
		content = OverlayPopupDimmed(content, popup, m.width, h)
	}

	// Overlay priority quick picker if active (step 3 of global ;! priority update flow)
	if popup := m.renderPriorityQuickPicker(); popup != "" {
		content = OverlayPopupDimmed(content, popup, m.width, h)
	}

	// Overlay due date quick picker if active (step 3 of global ;td due date flow)
	if popup := m.renderDueDateQuickPicker(); popup != "" {
		content = OverlayPopupDimmed(content, popup, m.width, h)
	}

	// Overlay section change quick picker if active (step 3 of global ;ts section change flow)
	if popup := m.renderSectionChangeQuickPicker(); popup != "" {
		content = OverlayPopupDimmed(content, popup, m.width, h)
	}

	// Overlay quick note popup if active
	if popup := m.renderQuickNoteOverlay(); popup != "" {
		content = OverlayPopupDimmed(content, popup, m.width, h)
	}

	// Overlay peek task detail if active
	if popup := m.renderPeekOverlay(); popup != "" {
		content = OverlayPopupDimmed(content, popup, m.width, h)
	}

	// Overlay confirm dialog last so it renders on top of all other popups
	if popup := m.renderConfirmDialogOverlay(); popup != "" {
		content = OverlayPopup(content, popup, m.width, h)
	}

	return content
}

// renderPeekOverlay renders the peek task detail overlay when peekActive is true.
func (m *AppModel) renderPeekOverlay() string {
	if !m.peekActive || m.peekTask == nil {
		return ""
	}
	task := m.peekTask

	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("Task Detail")
	footerText := HelpStyle.Render("Esc: dismiss")

	maxWidth := min(max(m.width*3/5, 40), m.width-4)
	titleMinInner := lipgloss.Width(title) + 6 - 2
	footerMinInner := lipgloss.Width(footerText) + 6 - 2
	innerWidth := max(maxWidth, max(titleMinInner, footerMinInner))

	var sb strings.Builder

	// Task title
	taskTitle := task.Title
	if len(taskTitle) > innerWidth-2 {
		taskTitle = taskTitle[:innerWidth-5] + "…"
	}
	sb.WriteString(lipgloss.NewStyle().Bold(true).Render(taskTitle))
	sb.WriteString("\n")

	// Type + Status
	typeStatus := string(task.Type) + " · " + string(task.Status)
	sb.WriteString(MutedStyle.Render(typeStatus))
	sb.WriteString("\n")

	// Priority
	if task.Priority > 0 {
		priorityNames := map[int]string{1: "Critical", 2: "High", 3: "Normal", 4: "Low", 5: "Someday"}
		if name, ok := priorityNames[task.Priority]; ok {
			sb.WriteString(MutedStyle.Render("Priority: " + name))
			sb.WriteString("\n")
		}
	}

	// Due date
	if task.DueDate != nil {
		sb.WriteString(MutedStyle.Render("Due: " + task.DueDate.Format("Mon Jan 2, 2006")))
		sb.WriteString("\n")
	}

	// Tags
	if len(task.Tags) > 0 {
		sb.WriteString(MutedStyle.Render("Tags: " + strings.Join(task.Tags, ", ")))
		sb.WriteString("\n")
	}

	// Count lines used so far for metadata (including border/padding overhead).
	// Border top + bottom = 2, padding top + bottom = 0, footer = 1.
	metadataLines := strings.Count(sb.String(), "\n")
	overhead := 3 // top border + bottom border + footer line
	maxTotalLines := max(m.height*3/5, 10)
	remainingLines := max(maxTotalLines-metadataLines-overhead, 2)

	// Description (fill remaining space)
	if task.Description != "" {
		sb.WriteString("\n")
		remainingLines-- // account for the blank separator
		lines := strings.Split(strings.TrimSpace(task.Description), "\n")
		if len(lines) > remainingLines {
			lines = lines[:remainingLines]
			lines[len(lines)-1] = "…"
		}
		for _, line := range lines {
			if len(line) > innerWidth-2 {
				line = line[:innerWidth-5] + "…"
			}
			sb.WriteString(MutedStyle.Render(line))
			sb.WriteString("\n")
		}
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(0, 1).
		Width(innerWidth)

	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	rendered := boxStyle.Render(sb.String())

	renderedLines := strings.Split(rendered, "\n")
	if len(renderedLines) < 2 {
		return rendered
	}

	actualWidth := lipgloss.Width(renderedLines[0])
	renderedLines[0] = renderTitledTopBorder(title, actualWidth, borderStyle)
	renderedLines[len(renderedLines)-1] = renderTitledBottomBorder(footerText, actualWidth, borderStyle)

	return strings.Join(renderedLines, "\n")
}

// renderGlobalProjectPicker renders the global project picker overlay when active.
func (m *AppModel) renderGlobalProjectPicker() string {
	if m.globalProjectPicker == nil {
		return ""
	}
	return m.globalProjectPicker.render(m.width)
}

// renderGlobalTaskPicker renders the global task picker overlay when active.
func (m *AppModel) renderGlobalTaskPicker() string {
	if m.globalTaskPicker == nil {
		return ""
	}
	return m.globalTaskPicker.render(m.width)
}

// renderStatusQuickPicker renders the status quick picker overlay when active.
func (m *AppModel) renderStatusQuickPicker() string {
	if m.statusQuickPicker == nil {
		return ""
	}
	return m.statusQuickPicker.render(m.width)
}

func (m *AppModel) renderSectionQuickPicker() string {
	if m.sectionQuickPicker == nil {
		return ""
	}
	return m.sectionQuickPicker.render(m.width)
}

func (m *AppModel) renderLinkTypeQuickPicker() string {
	if m.linkTypeQuickPicker == nil {
		return ""
	}
	return m.linkTypeQuickPicker.render(m.width)
}

func (m *AppModel) renderLinkModeQuickPicker() string {
	if m.linkModeQuickPicker == nil {
		return ""
	}
	return m.linkModeQuickPicker.render(m.width)
}

func (m *AppModel) renderPriorityQuickPicker() string {
	if m.priorityQuickPicker == nil {
		return ""
	}
	return m.priorityQuickPicker.render(m.width)
}

func (m *AppModel) renderDueDateQuickPicker() string {
	if m.dueDateQuickPicker == nil {
		return ""
	}
	return m.dueDateQuickPicker.render(m.width)
}

func (m *AppModel) renderSectionChangeQuickPicker() string {
	if m.sectionChangeQuickPicker == nil {
		return ""
	}
	return m.sectionChangeQuickPicker.render(m.width)
}

// renderQuickNoteOverlay renders the quick note overlay popup when active.
func (m *AppModel) renderQuickNoteOverlay() string {
	if m.quickNoteOverlay == nil {
		return ""
	}
	return m.quickNoteOverlay.render(m.width)
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

// renderSemicolonPrefixPopup renders the ";" or ";t" prefix popup.
func (m *AppModel) renderSemicolonPrefixPopup() string {
	if m.pendingSemicolonTPrefix {
		hints := PrefixHints{";t": SemicolonTPrefixHints}
		return RenderPrefixPopup(";t", hints, m.width, m.height)
	}
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
	// Top border separator for visual distinction; tinted with current view accent.
	separator := renderHorizontalDivider(m.width, WithDividerColor(m.currentAccentColor()))

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

	status := m.buildBreadcrumb()

	// Check if view supports help shortcut hint — only show if view implements HelpProvider
	currentView := m.currentView()
	showHelp := false
	if view := m.views.Get(currentView); view != nil {
		if _, ok := view.(HelpProvider); ok {
			showHelp = true
		}
	}

	help := ""
	if showHelp && m.isNotesFullscreen() {
		showHelp = false
	}
	if showHelp {
		help = "Press : for commands, ? for help"
	}

	accent := m.currentAccentColor()
	accentBarStyle := lipgloss.NewStyle().
		Foreground(accent).
		Background(ColorBackground).
		Padding(0, 1)

	var left string
	if m.currentView() == ViewDashboard && !m.config.GetTutorDismissed() {
		left = accentBarStyle.Render("Dashboard") + ": Run " + InfoStatusStyle.Render("tutor") + " command to start. Dismiss with " + InfoStatusStyle.Render("|")
	} else {
		left = accentBarStyle.Render(status)
	}

	// Add filter pill if current view has active filters
	if view := m.views.Get(currentView); view != nil {
		if fp, ok := view.(ActiveFilterProvider); ok {
			if count := fp.ActiveFilterCount(); count > 0 {
				filterLabel := "filters"
				if count == 1 {
					filterLabel = "filter"
				}
				left += " " + FilterBadgeStyle.Render(fmt.Sprintf("%d %s", count, filterLabel))
			}
		}
	}

	right := HelpStyle.Render(help)

	gap := max(m.width-lipgloss.Width(left)-lipgloss.Width(right), 1)

	spacer := ThemeSpacer(gap)

	return separator + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, left, spacer, right)
}

// buildBreadcrumb constructs a compact "Dashboard > project > view" label from the view stack.
// Each segment uses the view's StatusBarLabel() when available, with fallbacks for common views.
// Project names are truncated to keep the breadcrumb short.
func (m *AppModel) buildBreadcrumb() string {
	const maxProjectLen = 16
	const sep = " > "

	// Build a deduplicated view stack, keeping the LAST occurrence of each
	// view type (e.g. for stacked ViewProjectTree the deepest project wins).
	lastIdx := make(map[ViewType]int)
	for i, vt := range m.viewStack {
		lastIdx[vt] = i
	}

	var parts []string
	for i, vt := range m.viewStack {
		if lastIdx[vt] != i {
			continue // skip earlier duplicates
		}
		label := m.breadcrumbLabel(vt, maxProjectLen)
		if label != "" {
			parts = append(parts, label)
		}
	}

	if len(parts) == 0 {
		return ""
	}

	crumb := strings.Join(parts, sep)
	// Truncate total breadcrumb if it would crowd out the help text.
	maxLen := max(m.width/2, 20)
	if len(crumb) > maxLen {
		crumb = crumb[:maxLen-1] + "…"
	}
	return crumb
}

// breadcrumbLabel returns a short human-readable label for a single view type.
func (m *AppModel) breadcrumbLabel(vt ViewType, maxProjectLen int) string {
	// ViewDashboard has no StatusBarLabel — use a fixed string.
	if vt == ViewDashboard {
		return "Dashboard"
	}

	// Use the registered view's StatusBarLabel when available.
	if view := m.views.Get(vt); view != nil {
		if labeler, ok := view.(StatusBarLabeler); ok {
			raw := labeler.StatusBarLabel()
			// Strip "Project: " prefix from project tree label for compactness.
			label := strings.TrimPrefix(raw, "Project: ")
			// Also strip cycle-position suffix (starts with " [") for compactness.
			if idx := strings.Index(label, " ["); idx > 0 {
				label = label[:idx]
			}
			if len(label) > maxProjectLen {
				label = label[:maxProjectLen-1] + "…"
			}
			return label
		}
	}

	// Fallback labels for views that don't implement StatusBarLabeler.
	switch vt {
	case ViewProjectTree:
		if m.currentProject != nil {
			name := m.currentProject.Name
			if len(name) > maxProjectLen {
				name = name[:maxProjectLen-1] + "…"
			}
			return name
		}
		return "Project"
	case ViewFilter:
		return "Filter"
	case ViewDailyReview:
		return "Daily Review"
	case ViewWeeklyReview:
		return "Weekly Review"
	default:
	}
	return ""
}
