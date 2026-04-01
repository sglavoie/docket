package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
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
				b.WriteString(BoldPrimaryStyle.Render(fmt.Sprintf("[%s]", ctx)))
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

// renderColorInputField renders a hex color input with swatch and validation error.
// defaultColor is a lipgloss color string (ANSI code or hex) shown as a swatch when the input is empty.
func (m *SettingsModel) renderColorInputField(b *strings.Builder, label string, input textinput.Model, focusIdx, labelWidth, indent int, desc string, defaultColor string) {
	b.WriteString(renderFormLabelWithWidth(label, labelWidth, m.focusIndex == focusIdx))
	b.WriteString(input.View())
	b.WriteString("  ")
	if colorVal := normalizeHexColor(input.Value()); colorVal != "" && hexColorRegex.MatchString(strings.TrimSpace(input.Value())) {
		b.WriteString(lipgloss.NewStyle().Background(lipgloss.Color(colorVal)).Render("  "))
		b.WriteString("  ")
	} else if defaultColor != "" {
		b.WriteString(lipgloss.NewStyle().Background(lipgloss.Color(defaultColor)).Render("  "))
		b.WriteString("  ")
	}
	b.WriteString(MutedStyle.Render("(hex, blank = default)"))
	b.WriteString("\n")
	if m.focusIndex == focusIdx && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	if desc != "" {
		b.WriteString(renderFormFieldDesc(desc, indent, m.focusIndex == focusIdx))
		b.WriteString("\n")
	}
}

func (m *SettingsModel) renderColorPreviewPane(width int) string {
	var b strings.Builder

	// Title
	b.WriteString(SubtitleStyle.Render("Color Preview"))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render(strings.Repeat("─", min(width-1, 25))))
	b.WriteString("\n\n")

	// Status
	b.WriteString(MutedStyle.Render("Status"))
	b.WriteString("\n")
	b.WriteString(GetStatusBadge("todo", "□ Todo"))
	b.WriteString("  ")
	b.WriteString(GetStatusBadge("in_progress", "▶ In Progress"))
	b.WriteString("  ")
	b.WriteString(GetStatusBadge("done", "✓ Done"))
	b.WriteString("\n")
	b.WriteString(GetStatusBadge("review", "◑ Review"))
	b.WriteString("  ")
	b.WriteString(GetStatusBadge("standby", "⏸ Standby"))
	b.WriteString("  ")
	b.WriteString(GetStatusBadge("blocked", "⊘ Blocked"))
	b.WriteString("\n")
	b.WriteString(GetStatusBadge("canceled", "✕ Canceled"))
	b.WriteString("\n\n")

	// Priority
	b.WriteString(MutedStyle.Render("Priority"))
	b.WriteString("\n")
	b.WriteString(GetPriorityStyle(1).Render("◉ P1"))
	b.WriteString("  ")
	b.WriteString(GetPriorityStyle(2).Render("● P2"))
	b.WriteString("  ")
	b.WriteString(GetPriorityStyle(3).Render("◆ P3"))
	b.WriteString("  ")
	b.WriteString(GetPriorityStyle(4).Render("◇ P4"))
	b.WriteString("  ")
	b.WriteString(GetPriorityStyle(5).Render("· P5"))
	b.WriteString("\n\n")

	// Types
	b.WriteString(MutedStyle.Render("Types"))
	b.WriteString("\n")
	b.WriteString(GetTypeBadge("epic", "Epic"))
	b.WriteString(" ")
	b.WriteString(GetTypeBadge("story", "Story"))
	b.WriteString(" ")
	b.WriteString(GetTypeBadge("task", "Task"))
	b.WriteString(" ")
	b.WriteString(GetTypeBadge("bug", "Bug"))
	b.WriteString(" ")
	b.WriteString(GetTypeBadge("idea", "Idea"))
	b.WriteString("\n\n")

	// Text
	b.WriteString(MutedStyle.Render("Text"))
	b.WriteString("\n")
	b.WriteString(NormalStyle.Render("Normal text sample"))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render("Muted description text"))
	b.WriteString("\n")
	b.WriteString(BoldPrimaryStyle.Render("Primary"))
	b.WriteString(MutedStyle.Render(" · "))
	b.WriteString(BoldSecondaryStyle.Render("Secondary"))
	b.WriteString(MutedStyle.Render(" · "))
	b.WriteString(ActiveStyle.Render("Accent"))
	b.WriteString("\n")
	b.WriteString(SuccessStyle.Render("✓ Success"))
	b.WriteString("  ")
	b.WriteString(WarningMessageStyle.Render("⚠ Warning"))
	b.WriteString("  ")
	b.WriteString(ErrorStatusStyle.Render("✕ Error"))
	b.WriteString("\n\n")

	// Selection
	b.WriteString(MutedStyle.Render("Selection"))
	b.WriteString("\n")
	b.WriteString(SelectedStyle.Render("▸ Selected cursor row"))
	b.WriteString("\n\n")

	// View Modes
	b.WriteString(MutedStyle.Render("View Modes"))
	b.WriteString("\n")
	b.WriteString(ViewModeActiveStyle.Render("Active"))
	b.WriteString(" ")
	b.WriteString(ViewModeNextStyle.Render("Next"))
	b.WriteString(" ")
	b.WriteString(ViewModeInboxStyle.Render("Inbox"))
	b.WriteString(" ")
	b.WriteString(ViewModeWaitingStyle.Render("Waiting"))
	b.WriteString("\n")
	b.WriteString(ViewModeSomedayStyle.Render("Someday"))
	b.WriteString(" ")
	b.WriteString(ViewModeReviewStyle.Render("Review"))
	b.WriteString(" ")
	b.WriteString(ViewModeStandbyStyle.Render("Standby"))
	b.WriteString("\n\n")

	// Border sample
	b.WriteString(lipgloss.NewStyle().Foreground(ColorBorder).Render(strings.Repeat("─", min(width-4, 20)) + " Border"))

	return b.String()
}

func (m *SettingsModel) renderPaletteTab() string {
	const minPreviewWidth = 100

	var formContent string
	switch m.subTab {
	case 1:
		formContent = m.renderPaletteBgCursorSection()
	default:
		formContent = m.renderPaletteColorsSection()
	}

	if m.width < minPreviewWidth {
		return formContent
	}

	// Horizontal split: left form | separator | right preview
	separatorWidth := 3 // space + │ + space
	leftPaneWidth := m.width*3/5
	rightWidth := m.width - leftPaneWidth - separatorWidth

	separator := lipgloss.NewStyle().
		Foreground(ColorBorder).
		Render(" │ ")

	// Pad separator to match content height
	formLines := strings.Split(formContent, "\n")
	separatorLines := make([]string, len(formLines))
	for i := range separatorLines {
		separatorLines[i] = separator
	}
	separatorCol := strings.Join(separatorLines, "\n")

	previewContent := m.renderColorPreviewPane(rightWidth)

	leftPane := lipgloss.NewStyle().Width(leftPaneWidth).Render(formContent)

	return lipgloss.JoinHorizontal(lipgloss.Top,
		leftPane,
		separatorCol,
		previewContent,
	)
}

func (m *SettingsModel) renderPaletteColorsSection() string {
	var b strings.Builder
	const labelWidth = 16
	const indent = 18

	m.renderColorInputField(&b, "Normal Text: ", m.normalTextInput, 4, labelWidth, indent, "Foreground for normal text", "")
	m.renderColorInputField(&b, "Muted: ", m.mutedInput, 5, labelWidth, indent, "Subdued text and descriptions", "248")
	m.renderColorInputField(&b, "Primary: ", m.primaryInput, 6, labelWidth, indent, "Primary accent color", "39")
	m.renderColorInputField(&b, "Secondary: ", m.secondaryInput, 7, labelWidth, indent, "Secondary accent color", "33")
	m.renderColorInputField(&b, "Accent: ", m.accentInput, 8, labelWidth, indent, "Highlight and accent color", "#f2eb16")
	m.renderColorInputField(&b, "Success: ", m.successInput, 9, labelWidth, indent, "Success/completed status", "42")
	m.renderColorInputField(&b, "Warning: ", m.warningInput, 10, labelWidth, indent, "Warning indicators", "214")
	m.renderColorInputField(&b, "Error: ", m.errorColorInput, 11, labelWidth, indent, "Error indicators", "196")
	m.renderColorInputField(&b, "Border: ", m.borderInput, 12, labelWidth, indent, "Border and divider color", "244")

	return b.String()
}

func (m *SettingsModel) renderPaletteBgCursorSection() string {
	var b strings.Builder
	const labelWidth = 16
	const indent = 18

	m.renderColorInputField(&b, "Background: ", m.bgColorInput, 13, labelWidth, indent, "HEX background color for all TUI views", "")
	m.renderColorInputField(&b, "Cursor BG: ", m.cursorColorInput, 14, labelWidth, indent, "HEX background color for selection/cursor rows", DefaultSelectedBg)
	m.renderColorInputField(&b, "Cursor Text: ", m.cursorTextColorInput, 15, labelWidth, indent, "HEX foreground color for selection/cursor text", DefaultSelectedFg)

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
	b.WriteString(renderFormLabelWithWidth("Due Soon (days): ", labelWidth, m.focusIndex == 16))
	b.WriteString(m.dueSoonInput.View())
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Tasks due within this many days", indent, m.focusIndex == 16))

	b.WriteString("\n")

	// Future Days
	b.WriteString(renderFormLabelWithWidth("Future (days): ", labelWidth, m.focusIndex == 17))
	b.WriteString(m.futureInput.View())
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Tasks due further out than this", indent, m.focusIndex == 17))

	b.WriteString("\n")

	// Old Days
	b.WriteString(renderFormLabelWithWidth("Old (days): ", labelWidth, m.focusIndex == 18))
	b.WriteString(m.oldDaysInput.View())
	b.WriteString("       ")
	b.WriteString(MutedStyle.Render("(based on last update)"))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Tasks not updated in this many days", indent, m.focusIndex == 18))

	b.WriteString("\n")

	// Completed Hours
	b.WriteString(renderFormLabelWithWidth("Completed (hours): ", labelWidth, m.focusIndex == 19))
	b.WriteString(m.completedHoursInput.View())
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Show recently completed within this window", indent, m.focusIndex == 19))

	if m.focusIndex >= 16 && m.focusIndex <= 19 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}

	return b.String()
}

func (m *SettingsModel) renderBehaviorTab() string {
	switch m.subTab {
	case 1:
		return m.renderBehaviorLinkedTasksSection()
	case 2:
		return m.renderBehaviorPinnedTasksSection()
	case 3:
		return m.renderBehaviorRecentTasksSection()
	case 4:
		return m.renderBehaviorPopupsSection()
	case 5:
		return m.renderBehaviorSectionLimitsSection()
	default:
		return m.renderBehaviorRecentActivitySection()
	}
}

func (m *SettingsModel) renderBehaviorRecentActivitySection() string {
	var b strings.Builder
	const labelWidth = 17
	const indent = 19

	b.WriteString(renderFormLabelWithWidth("Task limit: ", labelWidth, m.focusIndex == 20))
	b.WriteString(m.recentActivityInput.View())
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render("(0 = no limit)"))
	b.WriteString("\n")

	if m.focusIndex == 20 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Maximum tasks shown in recent activity", indent, m.focusIndex == 20))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Show completed: ", labelWidth, m.focusIndex == 21))
	b.WriteString(renderFormToggle("Yes", "No", m.recentActivityShowAll, m.focusIndex == 21))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Include completed tasks in recent activity view", indent, m.focusIndex == 21))

	return b.String()
}

func (m *SettingsModel) renderBehaviorLinkedTasksSection() string {
	var b strings.Builder
	const labelWidth = 17
	const indent = 19

	b.WriteString(renderFormLabelWithWidth("After creation: ", labelWidth, m.focusIndex == 22))
	b.WriteString(renderFormToggle("Jump to new", "Stay on source", m.linkedTaskJump, m.focusIndex == 22))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Where to navigate after creating a linked task", indent, m.focusIndex == 22))

	return b.String()
}

func (m *SettingsModel) renderBehaviorPinnedTasksSection() string {
	var b strings.Builder
	const labelWidth = 17
	const indent = 19

	b.WriteString(renderFormLabelWithWidth("Bar limit: ", labelWidth, m.focusIndex == 23))
	b.WriteString(m.maxPinnedInput.View())
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render("(0=disable, 1-20)"))
	b.WriteString("\n")
	if m.focusIndex == 23 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Maximum pinned tasks displayed in the bar", indent, m.focusIndex == 23))

	return b.String()
}

func (m *SettingsModel) renderBehaviorRecentTasksSection() string {
	var b strings.Builder
	const labelWidth = 17
	const indent = 19

	b.WriteString(renderFormLabelWithWidth("Task limit: ", labelWidth, m.focusIndex == 24))
	b.WriteString(m.maxRecentTaskVisitsInput.View())
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render("(1-50, default 15)"))
	b.WriteString("\n")
	if m.focusIndex == 24 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Maximum tasks shown in quick-switch overlay (Ctrl+R)", indent, m.focusIndex == 24))

	return b.String()
}

func (m *SettingsModel) renderBehaviorPopupsSection() string {
	var b strings.Builder
	const labelWidth = 17
	const indent = 19

	b.WriteString(renderFormLabelWithWidth("Auto-dismiss: ", labelWidth, m.focusIndex == 25))
	b.WriteString(m.popupTimeoutInput.View())
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render("(0=stay, 1-30 seconds)"))
	b.WriteString("\n")
	if m.focusIndex == 25 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Seconds before prefix popups auto-dismiss (0=disabled)", indent, m.focusIndex == 25))

	return b.String()
}

func (m *SettingsModel) renderBehaviorSectionLimitsSection() string {
	var b strings.Builder
	const labelWidth = 17
	const indent = 19

	type limitField struct {
		label    string
		focusIdx int
		input    *textinput.Model
	}
	fields := []limitField{
		{"Active: ", 26, &m.sectionLimitActiveInput},
		{"Next: ", 27, &m.sectionLimitNextInput},
		{"Inbox: ", 28, &m.sectionLimitInboxInput},
		{"Standby: ", 29, &m.sectionLimitStandbyInput},
		{"Waiting: ", 30, &m.sectionLimitWaitingInput},
		{"Someday: ", 31, &m.sectionLimitSomedayInput},
		{"Review: ", 32, &m.sectionLimitReviewInput},
	}
	for _, f := range fields {
		b.WriteString(renderFormLabelWithWidth(f.label, labelWidth, m.focusIndex == f.focusIdx))
		b.WriteString(f.input.View())
		b.WriteString("  ")
		b.WriteString(MutedStyle.Render("(0=disabled, max 999)"))
		b.WriteString("\n")
		if m.focusIndex == f.focusIdx && m.validationErr != "" {
			b.WriteString(strings.Repeat(" ", indent))
			b.WriteString(ValidationErrorStyle.Render(m.validationErr))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m *SettingsModel) renderAttachmentsTab() string {
	var b strings.Builder
	const labelWidth = 20 // "Attachment Editor: " padded
	const indent = 22     // 2 + labelWidth

	b.WriteString(SubtitleStyle.Render("Editor Configuration"))
	b.WriteString("\n\n")

	// Attachment Editor
	b.WriteString(renderFormLabelWithWidth("Attachment Editor: ", labelWidth, m.focusIndex == 33))
	b.WriteString(m.attachmentEditorInput.View())
	b.WriteString("\n")
	if m.focusIndex == 33 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Editor for general attachments (empty = OS default)", indent, m.focusIndex == 33))

	b.WriteString("\n")

	// Code Editor
	b.WriteString(renderFormLabelWithWidth("Code Editor: ", labelWidth, m.focusIndex == 34))
	b.WriteString(m.codeEditorInput.View())
	b.WriteString("\n")
	if m.focusIndex == 34 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Editor for code references (empty = code)", indent, m.focusIndex == 34))

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
	m.renderColumnToggles(&b, m.treeColumnDefaults, 35, labelWidth, m.treePriorityMode)
	return b.String()
}

func (m *SettingsModel) renderOutlinerTab() string {
	switch m.subTab {
	case 1:
		return m.renderOutlinerSectionStateSection()
	case 2:
		return m.renderOutlinerColumnVisibilitySection()
	default:
		return m.renderOutlinerSidebarSection()
	}
}

func (m *SettingsModel) renderOutlinerSidebarSection() string {
	var b strings.Builder
	const labelWidth = 22
	const indent = 24

	b.WriteString(renderFormLabelWithWidth("Cross-Project Sidebar: ", labelWidth, m.focusIndex == 48))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.crossOutlinerSidebar, m.focusIndex == 48))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Show sidebar by default in cross-project outliner", indent, m.focusIndex == 48))

	return b.String()
}

func (m *SettingsModel) renderOutlinerSectionStateSection() string {
	var b strings.Builder
	const labelWidth = 22
	const indent = 24

	for i, label := range outlinerSectionLabels {
		focusIdx := 49 + i
		b.WriteString(renderFormLabelWithWidth(label+": ", labelWidth, m.focusIndex == focusIdx))
		b.WriteString(renderFormToggle("Collapsed", "Expanded", m.outlinerCollapsed[i], m.focusIndex == focusIdx))
		b.WriteString("\n")
		if i == 0 {
			b.WriteString(renderFormFieldDesc("Tasks with no explicit section (in-progress)", indent, m.focusIndex == focusIdx))
		}
	}

	return b.String()
}

func (m *SettingsModel) renderOutlinerColumnVisibilitySection() string {
	var b strings.Builder
	const labelWidth = 22

	m.renderColumnToggles(&b, m.outlinerColumnDefaults, 56, labelWidth, m.outlinerPriorityMode)

	return b.String()
}

func (m *SettingsModel) renderNotificationsTab() string {
	switch m.subTab {
	case 1:
		return m.renderNotificationsTasksSection()
	case 2:
		return m.renderNotificationsProjectsSection()
	default:
		return m.renderNotificationsReviewsSection()
	}
}

func (m *SettingsModel) renderNotificationsReviewsSection() string {
	var b strings.Builder
	const labelWidth = 22
	const indent = 24

	b.WriteString(renderFormLabelWithWidth("Daily Review: ", labelWidth, m.focusIndex == 69))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.alertDailyReviewEnabled, m.focusIndex == 69))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Remind to do daily review", indent, m.focusIndex == 69))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Review Hour: ", labelWidth, m.focusIndex == 70))
	b.WriteString(m.alertDailyReviewHourInput.View())
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render("(0-23)"))
	b.WriteString("\n")
	if m.focusIndex == 70 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Hour of day to trigger daily review reminder", indent, m.focusIndex == 70))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Weekly Review: ", labelWidth, m.focusIndex == 71))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.alertWeeklyReviewEnabled, m.focusIndex == 71))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Remind to do weekly review", indent, m.focusIndex == 71))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Review Day: ", labelWidth, m.focusIndex == 72))
	b.WriteString(renderEnumSelector(weeklyReviewDayOptions, m.alertWeeklyReviewDay, m.focusIndex == 72))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Day of week for weekly review reminder", indent, m.focusIndex == 72))

	return b.String()
}

func (m *SettingsModel) renderNotificationsTasksSection() string {
	var b strings.Builder
	const labelWidth = 22
	const indent = 24

	b.WriteString(renderFormLabelWithWidth("Overdue: ", labelWidth, m.focusIndex == 73))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.alertOverdueEnabled, m.focusIndex == 73))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Alert for tasks past their due date", indent, m.focusIndex == 73))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Due Soon: ", labelWidth, m.focusIndex == 74))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.alertDueSoonEnabled, m.focusIndex == 74))
	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Due Soon (days): ", labelWidth, m.focusIndex == 75))
	b.WriteString(m.alertDueSoonDaysInput.View())
	b.WriteString("\n")
	if m.focusIndex == 75 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Alert when tasks are due within this many days", indent, m.focusIndex == 75))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Stale: ", labelWidth, m.focusIndex == 76))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.alertStaleEnabled, m.focusIndex == 76))
	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Stale (days): ", labelWidth, m.focusIndex == 77))
	b.WriteString(m.alertStaleDaysInput.View())
	b.WriteString("\n")
	if m.focusIndex == 77 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Days without updates before a task is stale", indent, m.focusIndex == 77))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Inbox Backlog: ", labelWidth, m.focusIndex == 78))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.alertInboxEnabled, m.focusIndex == 78))
	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Inbox Threshold: ", labelWidth, m.focusIndex == 79))
	b.WriteString(m.alertInboxThresholdInput.View())
	b.WriteString("\n")
	if m.focusIndex == 79 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Alert when inbox has more than this many tasks", indent, m.focusIndex == 79))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Blocked: ", labelWidth, m.focusIndex == 80))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.alertBlockedEnabled, m.focusIndex == 80))
	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Blocked (days): ", labelWidth, m.focusIndex == 81))
	b.WriteString(m.alertBlockedDaysInput.View())
	b.WriteString("\n")
	if m.focusIndex == 81 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Days a task must be blocked before alerting", indent, m.focusIndex == 81))

	return b.String()
}

func (m *SettingsModel) renderNotificationsProjectsSection() string {
	var b strings.Builder
	const labelWidth = 22
	const indent = 24

	b.WriteString(renderFormLabelWithWidth("Project Health: ", labelWidth, m.focusIndex == 82))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.alertProjectHealthEnabled, m.focusIndex == 82))
	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Stale (days): ", labelWidth, m.focusIndex == 83))
	b.WriteString(m.alertProjectStaleDaysInput.View())
	b.WriteString("\n")
	if m.focusIndex == 83 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Days without task updates to flag project health", indent, m.focusIndex == 83))

	return b.String()
}

func (m *SettingsModel) renderStagesTab() string {
	var b strings.Builder

	b.WriteString(SubtitleStyle.Render("Workflow Stages"))
	b.WriteString("\n\n")

	if len(m.stages) == 0 {
		b.WriteString(MutedStyle.Render("  No stages configured. Press 'a' to add one."))
		b.WriteString("\n")
	} else {
		for i, stage := range m.stages {
			if i == m.stageCursor {
				b.WriteString(BoldPrimaryStyle.Render("  ▸ "))
				b.WriteString(StageBadgeStyle.Render(stage))
			} else {
				b.WriteString(MutedStyle.Render("    "))
				b.WriteString(StageBadgeStyle.Render(stage))
			}
			b.WriteString("\n")
		}
	}

	if m.stageEditMode {
		b.WriteString("\n")
		if m.stageAddMode {
			b.WriteString(BoldStyle.Render("  New stage: "))
		} else {
			b.WriteString(BoldStyle.Render("  Edit stage: "))
		}
		b.WriteString(m.stageInput.View())
		b.WriteString("\n")
		if m.stageValidationErr != "" {
			b.WriteString("  ")
			b.WriteString(ValidationErrorStyle.Render(m.stageValidationErr))
			b.WriteString("\n")
		}
	}

	if !m.stageEditMode && m.hasUnsavedStages() {
		b.WriteString("\n")
		b.WriteString(WarningMessageStyle.Render("  Unsaved changes — press Enter to save"))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(MutedStyle.Render("  ───────────────────────────────"))
	b.WriteString("\n\n")
	b.WriteString(HelpStyle.Render("  a=add  e=edit  d=delete  J/K=reorder  Enter(save)/Esc(cancel)"))
	b.WriteString("\n")

	return b.String()
}

// settingsContentHeight returns the number of lines available for tab content,
// accounting for the title, tab bar, blank line, and footer.
func (m *SettingsModel) settingsContentHeight() int {
	// Title (1) + tab bar with border (2) + blank line (1) + footer (1) + padding (1)
	h := m.height - TabbedViewChrome
	// Subtract 1 extra line when sub-tab row is visible
	if subTabNamesForTab(m.tabs.Active) != nil {
		h--
	}
	if h < 3 {
		return 3
	}
	return h
}

// focusLineInContent returns the approximate line number within the rendered
// tab content where the currently focused field appears. This drives auto-scroll.
func (m *SettingsModel) focusLineInContent() int {
	tab := tabForField(m.focusIndex)
	localIdx := m.focusIndex - subTabFieldStart(tab, m.subTab)

	switch tab {
	case 0: // General: each field ~4-5 lines, context can be taller
		return localIdx * 5
	case 1: // Palette: ~3 lines per color input
		return localIdx * 3
	case 2: // Review: subtitle+blank (3 lines) then ~4 lines per field
		return 3 + localIdx*4
	case 3: // Behavior: ~4 lines per field
		return localIdx * 4
	case 4: // Attachments
		return 3 + localIdx*5
	case 5: // Project Tree: subtitle+blank (3) then 1 line per toggle
		return 3 + localIdx
	case 6: // Outliner
		switch m.subTab {
		case 0: // Sidebar: toggle+desc
			return 0
		case 1: // Section State: 1 line per toggle (+desc on first)
			if localIdx == 0 {
				return 0
			}
			return 1 + localIdx
		case 2: // Column Visibility: 1 line per toggle
			return localIdx
		}
	case 7: // Notifications
		switch m.subTab {
		case 0: // Reviews: ~4 lines per field
			return localIdx * 4
		case 1: // Tasks: varying heights
			switch localIdx {
			case 0:
				return 0 // overdue toggle
			case 1:
				return 4 // due soon toggle
			case 2:
				return 6 // due soon days
			case 3:
				return 10 // stale toggle
			case 4:
				return 12 // stale days
			case 5:
				return 16 // inbox toggle
			case 6:
				return 18 // inbox threshold
			case 7:
				return 22 // blocked toggle
			case 8:
				return 24 // blocked days
			}
			return localIdx * 3
		case 2: // Projects: 2 fields
			return localIdx * 2
		}
	case 8: // Stages: subtitle+blank (3) then cursor is at list position
		return 3
	}
	return 0
}

func (m *SettingsModel) View() tea.View {
	// Tab content
	var content string
	switch m.tabs.Active {
	case 0:
		content = m.renderGeneralTab()
	case 1:
		content = m.renderPaletteTab()
	case 2:
		content = m.renderReviewTab()
	case 3:
		content = m.renderBehaviorTab()
	case 4:
		content = m.renderAttachmentsTab()
	case 5:
		content = m.renderProjectTreeTab()
	case 6:
		content = m.renderOutlinerTab()
	case 7:
		content = m.renderNotificationsTab()
	case 8:
		content = m.renderStagesTab()
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
	maxScroll := max(len(contentLines)-visibleHeight, 0)
	if m.scrollOffset > maxScroll {
		m.scrollOffset = maxScroll
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}

	// Extract visible slice
	end := min(m.scrollOffset+visibleHeight, len(contentLines))
	visibleContent := strings.Join(contentLines[m.scrollOffset:end], "\n")

	// Build final output
	var out strings.Builder
	out.WriteString(renderFullscreenTitle("Settings", ""))
	out.WriteString("\n")

	// Tab bar (with optional sub-tab row)
	subNames := subTabNamesForTab(m.tabs.Active)
	if subNames != nil {
		out.WriteString(renderTabBarWithOptionalSubTabs(settingsTabNames, m.tabs.Active, subNames, m.subTabRowFocused, m.width))
		out.WriteString("\n")
		out.WriteString(renderSubTabBarWithFocus(subNames, m.subTab, m.subTabRowFocused, m.width))
		out.WriteString("\n\n")
	} else {
		out.WriteString(m.tabs.RenderBar(m.width))
		out.WriteString("\n\n")
	}

	out.WriteString(visibleContent)
	out.WriteString("\n")

	// Footer with scroll indicator
	scrollHint := ""
	if len(contentLines) > visibleHeight {
		scrollHint = fmt.Sprintf(" (%d/%d)", m.scrollOffset+1, maxScroll+1)
	}
	helpPrefix := ""
	if subTabNamesForTab(m.tabs.Active) != nil {
		helpPrefix = "(/) sub-tabs · "
	}
	left := HelpStyle.Render(helpPrefix + "[/]: tabs · Tab: fields · Space: toggle · Enter: save · Esc: cancel" + scrollHint)
	right := MutedStyle.Render("Settings")
	out.WriteString(renderStatusBarFooter(left, right, m.width))

	return tea.NewView(out.String())
}
