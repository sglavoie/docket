package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// View renders the section manager view.
func (m *SectionManagerModel) View() tea.View {
	var b strings.Builder

	b.WriteString(renderFullscreenTitle("Section Manager", ""))
	b.WriteString("\n\n")

	b.WriteString(m.renderSectionList())

	if m.colorPickMode {
		b.WriteString(m.renderColorPicker())
	}

	if m.errorMsg != "" {
		b.WriteString("\n")
		b.WriteString(TagManagerErrorStyle.Render(m.errorMsg))
		b.WriteString("\n")
	}

	b.WriteString(HelpStyle.Render(renderWrappingHelpLine(m.footerHelp(), m.width)))

	return tea.NewView(b.String())
}

func (m *SectionManagerModel) renderSectionList() string {
	if len(m.sections) == 0 {
		return MutedStyle.Render("No sections in project") + "\n"
	}

	var b strings.Builder
	for i, section := range m.sections {
		cursor := "  "
		style := TagManagerItemStyle
		if i == m.cursor {
			cursor = "> "
			style = TagManagerItemSelectedStyle
		}

		sectionColor := m.getProject().GetSectionColor(section.Name)
		colorStyle := GetSectionColorStyle(sectionColor)
		colorSwatch := colorStyle.Render("█") + " "

		label := section.Name
		if label == "" {
			label = "Active"
		}
		line := fmt.Sprintf("%s%s%s ", cursor, colorSwatch, label)
		b.WriteString(style.Render(line))
		b.WriteString(TagManagerCountStyle.Render(fmt.Sprintf("(%d)", section.TaskCount)))
		b.WriteString("\n")
	}

	return b.String()
}

func (m *SectionManagerModel) renderColorPicker() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render("Select color:"))
	b.WriteString("\n")

	for i, pc := range SectionColorPalette {
		style := lipgloss.NewStyle().Foreground(GetSectionColorValue(pc.Name))
		swatch := " █ "
		if i == m.colorCursor {
			swatch = "[█]"
			style = style.Bold(true)
		}
		b.WriteString(style.Render(swatch))
	}

	b.WriteString("\n")
	b.WriteString(MutedStyle.Render(SectionColorPalette[m.colorCursor].Name))
	b.WriteString("\n")
	return b.String()
}

func (m *SectionManagerModel) footerHelp() string {
	if m.colorPickMode {
		return "h/l nav · c cycle · x clear · enter confirm · esc cancel"
	}
	if len(m.sections) > 0 {
		return "j/k nav · enter filter · c color · esc close"
	}
	return "esc close"
}

func (m *SectionManagerModel) HasTextInput() bool { return false }

func (m *SectionManagerModel) ShouldCaptureEsc() bool {
	return m.colorPickMode
}

func (m *SectionManagerModel) IsFullHeight() bool { return false }

func (m *SectionManagerModel) StatusBarLabel() string { return "Section Manager" }
