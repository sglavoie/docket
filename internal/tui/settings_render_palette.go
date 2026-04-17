package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
)

// renderColorInputField renders a hex color input with swatch and validation error.
// defaultColor is a lipgloss color string (ANSI code or hex) shown as a swatch when the input is empty.
func (m *SettingsModel) renderColorInputField(b *strings.Builder, label string, input textinput.Model, focusIdx, labelWidth, indent int, desc string, defaultColor string) {
	b.WriteString(renderFormLabelWithWidth(label, labelWidth, m.focusIndex == focusIdx))
	input.SetStyles(themedTextInputStyles())
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

// renderPresetNameLabel formats the active preset name (or "(built-in)" label)
// in black on light themes and white on dark themes for contrast against the bg.
func (m *SettingsModel) renderPresetNameLabel() string {
	name := m.activePresetName
	if name == "" {
		if m.activeTheme == "light" {
			name = "Light (built-in)"
		} else {
			name = "Dark (built-in)"
		}
	}
	fg := lipgloss.Color("#ffffff")
	if m.activeTheme == "light" {
		fg = lipgloss.Color("#000000")
	}
	return lipgloss.NewStyle().Foreground(fg).Render(name)
}

func (m *SettingsModel) renderColorPreviewPane(width int) string {
	var b strings.Builder

	b.WriteString(SubtitleStyle.MarginBottom(0).Render("Color Preview"))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render(strings.Repeat("─", min(width-1, 25))))
	b.WriteString("\n")
	b.WriteString(m.renderPresetNameLabel())
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

	// When a preset overlay is active, show it instead of the form.
	var formContent string
	if m.themePresetMode != presetModeNone {
		formContent = m.renderPresetOverlay()
		// Inline error from openPresetLoad (shown before entering load mode).
	} else {
		switch m.subTab {
		case 1:
			formContent = m.renderPaletteBgCursorSection()
		case 2:
			formContent = m.renderPrioritiesSection()
		default:
			formContent = m.renderPaletteColorsSection()
		}
	}

	// Show inline error (e.g. "no saved presets found") even outside overlay.
	if m.themePresetMode == presetModeNone && m.themePresetErr != "" {
		formContent += "\n" + ValidationErrorStyle.Render(m.themePresetErr)
	}
	// Show transient flash (e.g. after preset deletion).
	if m.themePresetFlash != "" {
		formContent += "\n" + MutedStyle.Render(m.themePresetFlash)
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

// paletteSwatchDefault returns the default swatch color for a palette field
// from the canonical config.DefaultTheme for the active theme variant.
func paletteSwatchDefault(field, active string) string {
	d := config.DefaultTheme(active)
	p := d.Palette
	switch field {
	case "normal_text":
		return p.NormalText
	case "muted":
		return p.Muted
	case "dimmed":
		return p.Dimmed
	case "dimmed_bg":
		return p.DimmedBg
	case "dimmed_fg":
		return p.DimmedFg
	case "primary":
		return p.Primary
	case "secondary":
		return p.Secondary
	case "accent":
		return p.Accent
	case "success":
		return p.Success
	case "warning":
		return p.Warning
	case "error":
		return p.Error
	case "border":
		return p.Border
	case "background":
		return d.BackgroundColor
	case "cursor_bg":
		return d.CursorColor
	case "cursor_fg":
		return d.CursorTextColor
	case "priority1":
		return p.Priority1Title
	case "priority2":
		return p.Priority2Title
	case "priority3":
		return p.Priority3Title
	case "priority4":
		return p.Priority4Title
	case "priority5":
		return p.Priority5Title
	}
	return ""
}

func (m *SettingsModel) renderPaletteColorsSection() string {
	var b strings.Builder
	const labelWidth = 16
	const indent = 18

	b.WriteString(renderFormLabelWithWidth("Theme: ", labelWidth, m.focusIndex == 4))
	b.WriteString(renderFormToggle("Dark", "Light", m.activeTheme == "dark", m.focusIndex == 4))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Active palette — edits below apply to this theme", indent, m.focusIndex == 4))
	b.WriteString("\n")

	m.renderColorInputField(&b, "Normal Text: ", m.normalTextInput, 5, labelWidth, indent, "Foreground for normal text", paletteSwatchDefault("normal_text", m.activeTheme))
	m.renderColorInputField(&b, "Muted: ", m.mutedInput, 6, labelWidth, indent, "Subdued text and descriptions", paletteSwatchDefault("muted", m.activeTheme))
	m.renderColorInputField(&b, "Primary: ", m.primaryInput, 7, labelWidth, indent, "Primary accent color", paletteSwatchDefault("primary", m.activeTheme))
	m.renderColorInputField(&b, "Secondary: ", m.secondaryInput, 8, labelWidth, indent, "Secondary accent color", paletteSwatchDefault("secondary", m.activeTheme))
	m.renderColorInputField(&b, "Accent: ", m.accentInput, 9, labelWidth, indent, "Highlight and accent color", paletteSwatchDefault("accent", m.activeTheme))
	m.renderColorInputField(&b, "Success: ", m.successInput, 10, labelWidth, indent, "Success/completed status", paletteSwatchDefault("success", m.activeTheme))
	m.renderColorInputField(&b, "Warning: ", m.warningInput, 11, labelWidth, indent, "Warning indicators", paletteSwatchDefault("warning", m.activeTheme))
	m.renderColorInputField(&b, "Error: ", m.errorColorInput, 12, labelWidth, indent, "Error indicators", paletteSwatchDefault("error", m.activeTheme))
	m.renderColorInputField(&b, "Border: ", m.borderInput, 13, labelWidth, indent, "Border and divider color", paletteSwatchDefault("border", m.activeTheme))

	return b.String()
}

func (m *SettingsModel) renderPaletteBgCursorSection() string {
	var b strings.Builder
	const labelWidth = 16
	const indent = 18

	m.renderColorInputField(&b, "Background: ", m.bgColorInput, 14, labelWidth, indent, "HEX background color for all TUI views", paletteSwatchDefault("background", m.activeTheme))
	m.renderColorInputField(&b, "Cursor BG: ", m.cursorColorInput, 15, labelWidth, indent, "HEX background color for selection/cursor rows", paletteSwatchDefault("cursor_bg", m.activeTheme))
	m.renderColorInputField(&b, "Cursor Text: ", m.cursorTextColorInput, 16, labelWidth, indent, "HEX foreground color for selection/cursor text", paletteSwatchDefault("cursor_fg", m.activeTheme))

	return b.String()
}

func (m *SettingsModel) renderPrioritiesSection() string {
	var b strings.Builder
	const labelWidth = 16
	const indent = 18

	m.renderColorInputField(&b, "Priority 1: ", m.priority1TitleInput, 17, labelWidth, indent, "P1 Critical title color (bold+italic)", paletteSwatchDefault("priority1", m.activeTheme))
	m.renderColorInputField(&b, "Priority 2: ", m.priority2TitleInput, 18, labelWidth, indent, "P2 High title color (bold)", paletteSwatchDefault("priority2", m.activeTheme))
	m.renderColorInputField(&b, "Priority 3: ", m.priority3TitleInput, 19, labelWidth, indent, "P3 Normal title color", paletteSwatchDefault("priority3", m.activeTheme))
	m.renderColorInputField(&b, "Priority 4: ", m.priority4TitleInput, 20, labelWidth, indent, "P4 Low title color", paletteSwatchDefault("priority4", m.activeTheme))
	m.renderColorInputField(&b, "Priority 5: ", m.priority5TitleInput, 21, labelWidth, indent, "P5 Someday title color (faint)", paletteSwatchDefault("priority5", m.activeTheme))

	return b.String()
}
