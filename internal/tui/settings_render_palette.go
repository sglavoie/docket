package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
)

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

	b.WriteString(SubtitleStyle.Render("Color Preview"))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render(strings.Repeat("─", min(width-1, 25))))
	b.WriteString("\n\n")

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

	b.WriteString(MutedStyle.Render("Priority"))
	b.WriteString("\n")
	for i, icon := range []string{"◉", "●", "◆", "◇", "·"} {
		if i > 0 {
			b.WriteString("  ")
		}
		p := i + 1
		b.WriteString(GetPriorityStyle(p).Render(icon))
		b.WriteString(" ")
		b.WriteString(GetTitlePriorityStyle(p).Render(fmt.Sprintf("P%d", p)))
	}
	b.WriteString("\n\n")

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

	b.WriteString(MutedStyle.Render("Selection"))
	b.WriteString("\n")
	b.WriteString(SelectedStyle.Render("▸ Selected cursor row"))
	b.WriteString("\n\n")

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

	b.WriteString(lipgloss.NewStyle().Foreground(ColorBorder).Render(strings.Repeat("─", min(width-4, 20)) + " Border"))

	return b.String()
}

func (m *SettingsModel) renderPaletteTab() string {
	const minPreviewWidth = 100

	var formContent string
	switch m.subTab {
	case 1:
		formContent = m.renderPaletteBgCursorSection()
	case 2:
		formContent = m.renderPrioritiesSection()
	default:
		formContent = m.renderPaletteColorsSection()
	}

	if m.width < minPreviewWidth {
		return formContent
	}

	separatorWidth := 3
	leftPaneWidth := m.width * 3 / 5
	rightWidth := m.width - leftPaneWidth - separatorWidth

	separator := lipgloss.NewStyle().Foreground(ColorBorder).Render(" │ ")

	formLines := strings.Split(formContent, "\n")
	separatorLines := make([]string, len(formLines))
	for i := range separatorLines {
		separatorLines[i] = separator
	}
	separatorCol := strings.Join(separatorLines, "\n")

	previewContent := m.renderColorPreviewPane(rightWidth)
	leftPane := lipgloss.NewStyle().Width(leftPaneWidth).Render(formContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, separatorCol, previewContent)
}

func (m *SettingsModel) renderPaletteColorsSection() string {
	var b strings.Builder
	const labelWidth = 16
	const indent = 18

	b.WriteString(renderFormLabelWithWidth("Theme: ", labelWidth, m.focusIndex == 4))
	b.WriteString(MutedStyle.Render("Dark"))
	b.WriteString("\n")

	m.renderColorInputField(&b, "Normal Text: ", m.normalTextInput, 5, labelWidth, indent, "Foreground for normal text", "")
	m.renderColorInputField(&b, "Muted: ", m.mutedInput, 6, labelWidth, indent, "Subdued text and descriptions", "248")
	m.renderColorInputField(&b, "Primary: ", m.primaryInput, 7, labelWidth, indent, "Primary accent color", "39")
	m.renderColorInputField(&b, "Secondary: ", m.secondaryInput, 8, labelWidth, indent, "Secondary accent color", "33")
	m.renderColorInputField(&b, "Accent: ", m.accentInput, 9, labelWidth, indent, "Highlight and accent color", "#f2eb16")
	m.renderColorInputField(&b, "Success: ", m.successInput, 10, labelWidth, indent, "Success/completed status", "42")
	m.renderColorInputField(&b, "Warning: ", m.warningInput, 11, labelWidth, indent, "Warning indicators", "214")
	m.renderColorInputField(&b, "Error: ", m.errorColorInput, 12, labelWidth, indent, "Error indicators", "196")
	m.renderColorInputField(&b, "Border: ", m.borderInput, 13, labelWidth, indent, "Border and divider color", "244")

	return b.String()
}

func (m *SettingsModel) renderPaletteBgCursorSection() string {
	var b strings.Builder
	const labelWidth = 16
	const indent = 18

	m.renderColorInputField(&b, "Background: ", m.bgColorInput, 14, labelWidth, indent, "HEX background color for all TUI views", "")
	m.renderColorInputField(&b, "Cursor BG: ", m.cursorColorInput, 15, labelWidth, indent, "HEX background color for selection/cursor rows", DefaultSelectedBg)
	m.renderColorInputField(&b, "Cursor Text: ", m.cursorTextColorInput, 16, labelWidth, indent, "HEX foreground color for selection/cursor text", DefaultSelectedFg)

	return b.String()
}

func (m *SettingsModel) renderPrioritiesSection() string {
	var b strings.Builder
	const labelWidth = 16
	const indent = 18

	m.renderColorInputField(&b, "Priority 1: ", m.priority1TitleInput, 17, labelWidth, indent, "P1 Critical title color (bold+italic)", "#ffffff")
	m.renderColorInputField(&b, "Priority 2: ", m.priority2TitleInput, 18, labelWidth, indent, "P2 High title color (bold)", "#d0d0d0")
	m.renderColorInputField(&b, "Priority 3: ", m.priority3TitleInput, 19, labelWidth, indent, "P3 Normal title color", "#b8b8b8")
	m.renderColorInputField(&b, "Priority 4: ", m.priority4TitleInput, 20, labelWidth, indent, "P4 Low title color", "#9e9e9e")
	m.renderColorInputField(&b, "Priority 5: ", m.priority5TitleInput, 21, labelWidth, indent, "P5 Someday title color (faint)", "#6a6a6a")

	return b.String()
}
