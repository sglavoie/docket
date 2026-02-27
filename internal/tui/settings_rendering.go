package tui

import (
	"fmt"
	"strings"
)

func (m *SettingsModel) renderGeneralTab() string {
	var b strings.Builder
	const labelWidth = 20 // "Undo History Depth: " is the widest
	const indent = 22     // 2 (arrow prefix) + labelWidth

	// Editor input
	b.WriteString(renderFormLabelWithWidth("Editor: ", labelWidth, m.focusIndex == 0))
	b.WriteString(m.editorInput.View())
	b.WriteString("\n")
	if m.focusIndex == 0 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Command used to open tasks for editing", indent, m.focusIndex == 0))

	b.WriteString("\n")

	// Default context input
	b.WriteString(renderFormLabelWithWidth("Default Context: ", labelWidth, m.focusIndex == 1))
	b.WriteString(m.contextInput.View())
	b.WriteString("\n")

	if len(m.contexts) > 0 {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(MutedStyle.Render("Available: "))
		for i, ctx := range m.contexts {
			if i == m.contextCursor {
				b.WriteString(BoldStyle.Foreground(ColorPrimary).Render(fmt.Sprintf("[%s]", ctx)))
			} else {
				b.WriteString(MutedStyle.Render(ctx))
			}
			if i < len(m.contexts)-1 {
				b.WriteString(MutedStyle.Render(", "))
			}
		}
		if m.focusIndex == 1 {
			b.WriteString(HelpStyle.Render("  (←/→ to select, or type custom)"))
		}
		b.WriteString("\n")
	}

	if m.focusIndex == 1 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Default context for new projects", indent, m.focusIndex == 1))

	b.WriteString("\n")

	// Undo history depth input
	b.WriteString(renderFormLabelWithWidth("Undo History Depth: ", labelWidth, m.focusIndex == 2))
	b.WriteString(m.undoDepthInput.View())
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render("(0=disable, max 1000)"))
	b.WriteString("\n")

	if m.focusIndex == 2 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Number of undo steps to keep in history", indent, m.focusIndex == 2))

	b.WriteString("\n")

	// Default project view selector
	b.WriteString(renderFormLabelWithWidth("Default View: ", labelWidth, m.focusIndex == 3))
	b.WriteString(renderEnumSelector(defaultProjectViewOptions, m.defaultViewCursor, m.focusIndex == 3))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("View shown when opening a project", indent, m.focusIndex == 3))

	return b.String()
}

func (m *SettingsModel) renderReviewTab() string {
	var b strings.Builder
	const labelWidth = 20 // "Completed (hours): " padded
	const indent = 22

	b.WriteString(SubtitleStyle.Render("Review View Thresholds"))
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render("(0 = hide section)"))
	b.WriteString("\n\n")

	// Due Soon Days
	b.WriteString(renderFormLabelWithWidth("Due Soon (days): ", labelWidth, m.focusIndex == 4))
	b.WriteString(m.dueSoonInput.View())
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Tasks due within this many days", indent, m.focusIndex == 4))

	b.WriteString("\n")

	// Future Days
	b.WriteString(renderFormLabelWithWidth("Future (days): ", labelWidth, m.focusIndex == 5))
	b.WriteString(m.futureInput.View())
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Tasks due further out than this", indent, m.focusIndex == 5))

	b.WriteString("\n")

	// Old Days
	b.WriteString(renderFormLabelWithWidth("Old (days): ", labelWidth, m.focusIndex == 6))
	b.WriteString(m.oldDaysInput.View())
	b.WriteString("       ")
	b.WriteString(MutedStyle.Render("(based on last update)"))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Tasks not updated in this many days", indent, m.focusIndex == 6))

	b.WriteString("\n")

	// Completed Hours
	b.WriteString(renderFormLabelWithWidth("Completed (hours): ", labelWidth, m.focusIndex == 7))
	b.WriteString(m.completedHoursInput.View())
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Show recently completed within this window", indent, m.focusIndex == 7))

	if m.focusIndex >= 4 && m.focusIndex <= 7 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}

	return b.String()
}

func (m *SettingsModel) renderBehaviorTab() string {
	var b strings.Builder
	const labelWidth = 17 // "Show completed: " padded
	const indent = 19     // 2 + labelWidth

	b.WriteString(SubtitleStyle.Render("Recent Activity"))
	b.WriteString("\n\n")

	// Recent Activity Limit
	b.WriteString(renderFormLabelWithWidth("Task limit: ", labelWidth, m.focusIndex == 8))
	b.WriteString(m.recentActivityInput.View())
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render("(0 = no limit)"))
	b.WriteString("\n")

	if m.focusIndex == 8 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Maximum tasks shown in recent activity", indent, m.focusIndex == 8))

	b.WriteString("\n")

	// Recent Activity Show All Toggle
	b.WriteString(renderFormLabelWithWidth("Show completed: ", labelWidth, m.focusIndex == 9))
	b.WriteString(renderFormToggle("Yes", "No", m.recentActivityShowAll, m.focusIndex == 9))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Include completed tasks in recent activity view", indent, m.focusIndex == 9))

	b.WriteString("\n")
	b.WriteString(MutedStyle.Render("  ───────────────────────────────"))
	b.WriteString("\n\n")

	b.WriteString(SubtitleStyle.Render("Linked Task Creation"))
	b.WriteString("\n\n")

	// Linked Task Jump Toggle
	b.WriteString(renderFormLabelWithWidth("After creation: ", labelWidth, m.focusIndex == 10))
	b.WriteString(renderFormToggle("Jump to new", "Stay on source", m.linkedTaskJump, m.focusIndex == 10))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Where to navigate after creating a linked task", indent, m.focusIndex == 10))

	b.WriteString("\n")
	b.WriteString(MutedStyle.Render("  ───────────────────────────────"))
	b.WriteString("\n\n")

	b.WriteString(SubtitleStyle.Render("Pinned Tasks"))
	b.WriteString("\n\n")

	// Max Pinned Bar Items
	b.WriteString(renderFormLabelWithWidth("Bar limit: ", labelWidth, m.focusIndex == 11))
	b.WriteString(m.maxPinnedInput.View())
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render("(0=disable, 1-20)"))
	b.WriteString("\n")
	if m.focusIndex == 11 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Maximum pinned tasks displayed in the bar", indent, m.focusIndex == 11))

	return b.String()
}

func (m *SettingsModel) renderAttachmentsTab() string {
	var b strings.Builder
	const labelWidth = 20 // "Attachment Editor: " padded
	const indent = 22     // 2 + labelWidth

	b.WriteString(SubtitleStyle.Render("Editor Configuration"))
	b.WriteString("\n\n")

	// Attachment Editor
	b.WriteString(renderFormLabelWithWidth("Attachment Editor: ", labelWidth, m.focusIndex == 12))
	b.WriteString(m.attachmentEditorInput.View())
	b.WriteString("\n")
	if m.focusIndex == 12 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Editor for general attachments (empty = OS default)", indent, m.focusIndex == 12))

	b.WriteString("\n")

	// Code Editor
	b.WriteString(renderFormLabelWithWidth("Code Editor: ", labelWidth, m.focusIndex == 13))
	b.WriteString(m.codeEditorInput.View())
	b.WriteString("\n")
	if m.focusIndex == 13 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Editor for code references (empty = code)", indent, m.focusIndex == 13))

	b.WriteString("\n")
	b.WriteString(MutedStyle.Render("  ───────────────────────────────"))
	b.WriteString("\n\n")

	b.WriteString(SubtitleStyle.Render("Code References"))
	b.WriteString("\n\n")
	b.WriteString(MutedStyle.Render("  Code references use the file:line format (e.g., src/main.go:42)."))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render("  They open in your code editor at the specified line."))
	b.WriteString("\n")

	return b.String()
}

// renderColumnToggles renders 13 column visibility toggles.
// The Priority column (index 1) is rendered as a 3-way enum selector instead of a toggle.
func (m *SettingsModel) renderColumnToggles(b *strings.Builder, toggles [13]bool, startIdx, labelWidth int, priorityCursor int) {
	for i, col := range AllColumns {
		focusIdx := startIdx + i
		b.WriteString(renderFormLabelWithWidth(col.Label+": ", labelWidth, m.focusIndex == focusIdx))
		if col.ID == ColPriority {
			b.WriteString(renderEnumSelector(priorityModeOptions, priorityCursor, m.focusIndex == focusIdx))
		} else {
			b.WriteString(renderFormToggle("Visible", "Hidden", toggles[i], m.focusIndex == focusIdx))
		}
		b.WriteString("\n")
	}
}

func (m *SettingsModel) renderProjectTreeTab() string {
	var b strings.Builder
	const labelWidth = 18 // "Custom Section: " padded
	b.WriteString(SubtitleStyle.Render("Default Column Visibility"))
	b.WriteString("\n\n")
	m.renderColumnToggles(&b, m.treeColumnDefaults, 14, labelWidth, m.treePriorityMode)
	return b.String()
}

func (m *SettingsModel) renderKanbanTab() string {
	var b strings.Builder
	const labelWidth = 22 // "Max Swimlane Rows: " padded
	const indent = 24     // 2 + labelWidth

	b.WriteString(SubtitleStyle.Render("Kanban Board Display"))
	b.WriteString("\n\n")

	// Max Columns
	b.WriteString(renderFormLabelWithWidth("Max Columns: ", labelWidth, m.focusIndex == 27))
	b.WriteString(m.kanbanMaxColumnsInput.View())
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render("(1-20)"))
	b.WriteString("\n")
	if m.focusIndex == 27 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Maximum visible columns (auto-reduces for narrow terminals)", indent, m.focusIndex == 27))

	b.WriteString("\n")

	// Max Swimlane Rows
	b.WriteString(renderFormLabelWithWidth("Max Swimlane Rows: ", labelWidth, m.focusIndex == 28))
	b.WriteString(m.kanbanMaxSwimlaneInput.View())
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render("(1-20)"))
	b.WriteString("\n")
	if m.focusIndex == 28 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Maximum visible swimlane rows when swimlanes are active", indent, m.focusIndex == 28))

	b.WriteString("\n")
	b.WriteString(MutedStyle.Render("  ───────────────────────────────"))
	b.WriteString("\n\n")

	b.WriteString(SubtitleStyle.Render("Default Column Visibility"))
	b.WriteString("\n\n")
	m.renderColumnToggles(&b, m.kanbanColumnDefaults, 29, labelWidth, m.kanbanPriorityMode)

	return b.String()
}

func (m *SettingsModel) renderOutlinerTab() string {
	var b strings.Builder
	const labelWidth = 12 // "Someday: " padded
	const indent = 14     // 2 + labelWidth

	b.WriteString(SubtitleStyle.Render("Default Section State"))
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render("(All perspective)"))
	b.WriteString("\n\n")

	for i, label := range outlinerSectionLabels {
		focusIdx := 42 + i
		b.WriteString(renderFormLabelWithWidth(label+": ", labelWidth, m.focusIndex == focusIdx))
		b.WriteString(renderFormToggle("Collapsed", "Expanded", m.outlinerCollapsed[i], m.focusIndex == focusIdx))
		b.WriteString("\n")
		if i == 0 {
			b.WriteString(renderFormFieldDesc("Tasks with no explicit section (in-progress)", indent, m.focusIndex == focusIdx))
		}
	}

	b.WriteString("\n")
	b.WriteString(MutedStyle.Render("  ───────────────────────────────"))
	b.WriteString("\n\n")

	b.WriteString(SubtitleStyle.Render("Default Column Visibility"))
	b.WriteString("\n\n")
	m.renderColumnToggles(&b, m.outlinerColumnDefaults, 49, labelWidth, m.outlinerPriorityMode)

	return b.String()
}

// settingsContentHeight returns the number of lines available for tab content,
// accounting for the title, tab bar, blank line, and footer.
func (m *SettingsModel) settingsContentHeight() int {
	// Title (1) + tab bar with border (2) + blank line (1) + footer (1) + padding (1) = 6 lines of chrome
	h := m.height - 6
	if h < 3 {
		return 3
	}
	return h
}

// focusLineInContent returns the approximate line number within the rendered
// tab content where the currently focused field appears. This drives auto-scroll.
func (m *SettingsModel) focusLineInContent() int {
	tab := tabForField(m.focusIndex)
	localIdx := m.focusIndex - tabFieldStart[tab]

	switch tab {
	case 0: // General: each field ~4-5 lines, context can be taller
		return localIdx * 5
	case 1: // Review: subtitle+blank (3 lines) then ~4 lines per field
		return 3 + localIdx*4
	case 2: // Behavior: fields at varying heights
		switch localIdx {
		case 0:
			return 3 // recent activity limit
		case 1:
			return 8 // show completed
		case 2:
			return 14 // linked task jump
		case 3:
			return 20 // max pinned
		}
		return localIdx * 5
	case 3: // Attachments
		return 3 + localIdx*5
	case 4: // Project Tree: subtitle+blank (3) then 1 line per toggle
		return 3 + localIdx
	case 5: // Kanban: 2 text inputs (~10 lines), separator+subtitle (4), then toggles
		if localIdx <= 1 {
			return 3 + localIdx*5 // max columns, max swimlane
		}
		return 14 + (localIdx - 2) // column toggles start after separator
	case 6: // Outliner: subtitle+blank (2), 7 section toggles, blank+separator+blank+subtitle+blank (5), then column toggles
		if localIdx <= 6 {
			return 2 + localIdx // section toggles (indices 42-48)
		}
		// Column toggles start at line 14 (2 + 7 sections + 5 lines of separator/subtitle/blanks)
		// Note: Active's desc line only appears when Active is focused, so it doesn't affect column positions
		return 14 + (localIdx - 7) // column toggles (indices 49-61)
	}
	return 0
}

func (m *SettingsModel) View() string {
	// Tab content
	var content string
	switch m.tabs.Active {
	case 0:
		content = m.renderGeneralTab()
	case 1:
		content = m.renderReviewTab()
	case 2:
		content = m.renderBehaviorTab()
	case 3:
		content = m.renderAttachmentsTab()
	case 4:
		content = m.renderProjectTreeTab()
	case 5:
		content = m.renderKanbanTab()
	case 6:
		content = m.renderOutlinerTab()
	}

	// Split content into lines and apply scroll
	contentLines := strings.Split(content, "\n")
	visibleHeight := m.settingsContentHeight()

	// Auto-scroll to keep focused field visible.
	// Find the focused line by searching for the focus indicator (▸) in the rendered content.
	focusLine := -1
	for i, line := range contentLines {
		if strings.Contains(line, "▸") {
			focusLine = i
			break
		}
	}
	if focusLine < 0 {
		focusLine = m.focusLineInContent() // fallback for non-label fields
	}
	if focusLine < m.scrollOffset {
		m.scrollOffset = focusLine
	}
	if focusLine >= m.scrollOffset+visibleHeight {
		m.scrollOffset = focusLine - visibleHeight + 1
	}

	// Clamp scroll offset
	maxScroll := len(contentLines) - visibleHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scrollOffset > maxScroll {
		m.scrollOffset = maxScroll
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}

	// Extract visible slice
	end := m.scrollOffset + visibleHeight
	if end > len(contentLines) {
		end = len(contentLines)
	}
	visibleContent := strings.Join(contentLines[m.scrollOffset:end], "\n")

	// Build final output
	var out strings.Builder
	out.WriteString(renderFullscreenTitle("Settings", ""))
	out.WriteString("\n")

	// Tab bar
	out.WriteString(m.tabs.RenderBar(m.width))
	out.WriteString("\n\n")

	out.WriteString(visibleContent)
	out.WriteString("\n")

	// Footer with scroll indicator
	scrollHint := ""
	if len(contentLines) > visibleHeight {
		scrollHint = fmt.Sprintf(" (%d/%d)", m.scrollOffset+1, maxScroll+1)
	}
	left := HelpStyle.Render("[/]: tabs · Tab: fields · Space: toggle · Enter: save · Esc: cancel" + scrollHint)
	right := MutedStyle.Render("Settings")
	out.WriteString(renderStatusBarFooter(left, right, m.width))

	return out.String()
}
