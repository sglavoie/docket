package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// applyOverlays composites all popup/overlay layers onto the base content.
// h is the overlay positioning height (m.height for full-height views,
// targetContentHeight for views with an external status bar).
//
// Overlay visual contract:
//   - View-local prefix popups: no dim (user is mid-chord, context stays visible)
//   - Global prefix popups (";"): dim (full-screen mode change)
//   - Quick-switch popup: no dim (navigational, lightweight)
//   - Project cycle picker: dim (selection overlay)
//   - Pickers/selection overlays (dimmedOverlayRenderers): dim
//   - Peek/preview: dim (replaces context with detail)
//   - Confirm dialogs: dim (destructive action requires focus)
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

	// Overlay project cycle picker popup if active
	if m.showProjectCyclePicker {
		popup := RenderPrefixPopupWithCategory("`", "project", m.buildProjectCyclePickerHints(), m.width, h)
		content = OverlayPopupDimmed(content, popup, m.width, h)
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

	// Overlay dimmed picker/note popups (all share the same nil-check + render pattern)
	for _, renderer := range m.dimmedOverlayRenderers() {
		if popup := renderer.render(m.width); popup != "" {
			content = OverlayPopupDimmed(content, popup, m.width, h)
		}
	}

	// Overlay peek task detail if active
	if popup := m.renderPeekOverlay(); popup != "" {
		content = OverlayPopupDimmed(content, popup, m.width, h)
	}

	// Overlay confirm dialog last so it renders on top of all other popups
	if popup := m.renderConfirmDialogOverlay(); popup != "" {
		content = OverlayPopupDimmed(content, popup, m.width, h)
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
		dateStr := task.DueDate.Format("Mon Jan 2, 2006")
		dateStr = AppendTimeIfHasTime(task, dateStr, "15:04")
		sb.WriteString(MutedStyle.Render("Due: " + dateStr))
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

	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	return renderTitledBox(sb.String(), innerWidth, title, footerText, borderStyle)
}

// widthRenderer is satisfied by all overlay pickers/popups that render given a width.
type widthRenderer interface {
	render(width int) string
}

// dimmedOverlayRenderers returns the non-nil overlay renderers that are composited
// with OverlayPopupDimmed. Order matters: later entries render on top.
func (m *AppModel) dimmedOverlayRenderers() []widthRenderer {
	var out []widthRenderer
	if m.globalProjectPicker != nil {
		out = append(out, m.globalProjectPicker)
	}
	if m.globalTaskPicker != nil {
		out = append(out, m.globalTaskPicker)
	}
	if m.statusQuickPicker != nil {
		out = append(out, m.statusQuickPicker)
	}
	if m.sectionQuickPicker != nil {
		out = append(out, m.sectionQuickPicker)
	}
	if m.linkTypeQuickPicker != nil {
		out = append(out, m.linkTypeQuickPicker)
	}
	if m.linkModeQuickPicker != nil {
		out = append(out, m.linkModeQuickPicker)
	}
	if m.priorityQuickPicker != nil {
		out = append(out, m.priorityQuickPicker)
	}
	if m.dueDateQuickPicker != nil {
		out = append(out, m.dueDateQuickPicker)
	}
	if m.sectionChangeQuickPicker != nil {
		out = append(out, m.sectionChangeQuickPicker)
	}
	if m.quickNoteOverlay != nil {
		out = append(out, m.quickNoteOverlay)
	}
	return out
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
	hints := PrefixHints{";": m.buildSemicolonHints()}
	return RenderPrefixPopup(";", hints, m.width, m.height)
}

// buildSemicolonHints returns a copy of SemicolonPrefixHints with context-dependent disabling.
// Shortcuts pointing to empty/unavailable data are dimmed so users understand at a glance
// which destinations are currently active.
func (m *AppModel) buildSemicolonHints() []PrefixHint {
	hints := make([]PrefixHint, len(SemicolonPrefixHints))
	copy(hints, SemicolonPrefixHints)

	hasAlerts := m.alertCount() > 0
	hasWorkflowRules := m.config != nil && len(m.config.WorkflowRules) > 0

	for i := range hints {
		switch hints[i].Key {
		case "n":
			hints[i].Disabled = !hasAlerts
		case "W":
			hints[i].Disabled = !hasWorkflowRules
		}
	}
	return hints
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
