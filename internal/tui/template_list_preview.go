package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/template"
)

// renderPreviewPane renders the template preview with details.
func (m *TaskTemplatesModel) renderPreviewPane(width, height int) string {
	if m.cursor >= len(m.filteredTemplates) {
		return ""
	}

	t := m.filteredTemplates[m.cursor]
	isDefault := m.isDefaultTemplate(t.ID)
	lines := append([]string{previewHeader(isDefault), ""}, m.renderTemplateDetails(t, isDefault, width-2)...)
	lines = m.slicePreviewLines(lines)
	if len(lines) > height {
		lines = lines[:height]
	}

	return lipgloss.NewStyle().Width(width).Render(strings.Join(lines, "\n"))
}

// renderFullscreenPreview renders the preview as a fullscreen view for narrow screens.
func (m *TaskTemplatesModel) renderFullscreenPreview() string {
	if m.cursor >= len(m.filteredTemplates) {
		return ""
	}

	t := m.filteredTemplates[m.cursor]
	isDefault := m.isDefaultTemplate(t.ID)

	var b strings.Builder
	b.WriteString(renderFullscreenTitle("Template Preview", ""))
	b.WriteString("\n\n")

	lines := m.renderTemplateDetails(t, isDefault, m.Width()-4)
	lines = m.slicePreviewLines(lines)

	availableHeight := max(m.Height()-6, 5)
	if len(lines) > availableHeight {
		lines = lines[:availableHeight]
	}

	b.WriteString(strings.Join(lines, "\n"))
	b.WriteString("\n")
	b.WriteString(renderStatusBarFooter(HelpStyle.Render("j/k: scroll · Ctrl+D/U: page · Enter/Esc: close · e: edit · Space: toggle · d: delete"), "", m.Width()))

	return b.String()
}

func (m *TaskTemplatesModel) renderTemplateDetails(t template.TaskTemplate, isDefault bool, wrapWidth int) []string {
	var lines []string

	nameLine := BoldPrimaryStyle.Render(t.Name)
	if isDefault {
		nameLine += " " + MutedStyle.Render("[Default]")
	}
	lines = append(lines, nameLine, "")

	if t.Enabled {
		lines = append(lines, DoneStyle.Render("● Enabled"))
	} else {
		lines = append(lines, MutedStyle.Render("○ Disabled"))
	}
	lines = append(lines, "")

	if t.Category != "" {
		lines = append(lines, MutedStyle.Render("Category: ")+t.Category)
	}
	lines = append(lines, MutedStyle.Render("Type: ")+string(t.Type))
	if t.Priority > 0 {
		lines = append(lines, MutedStyle.Render("Priority: ")+fmt.Sprintf("%d", t.Priority))
	}
	if t.Section != "" {
		lines = append(lines, MutedStyle.Render("Section: ")+t.Section)
	}
	if len(t.Tags) > 0 {
		lines = append(lines, MutedStyle.Render("Tags: ")+strings.Join(t.Tags, ", "))
	}
	if varNames := template.VariableNames(&t); len(varNames) > 0 {
		lines = append(lines, MutedStyle.Render("Variables: ")+strings.Join(varNames, ", "))
	}
	if t.Description != "" {
		lines = append(lines, "", MutedStyle.Render("Description:"))
		for _, line := range wrapText(t.Description, wrapWidth) {
			lines = append(lines, "  "+line)
		}
	}
	if len(t.Checklist) > 0 {
		lines = append(lines, "", MutedStyle.Render("Checklist:"))
		for _, item := range t.Checklist {
			lines = append(lines, "  ☐ "+item)
		}
	}
	if len(t.Children) > 0 {
		lines = append(lines, "", MutedStyle.Render("Subtasks:"))
		for _, child := range t.Children {
			marker := "  ○ "
			if child.Optional {
				marker = "  ◌ "
			}
			line := marker + MutedStyle.Render(fmt.Sprintf("[%s] ", child.GetChildType())) + child.Title
			if child.Optional {
				line += MutedStyle.Render(" (optional)")
			}
			lines = append(lines, line)
		}
	}
	if t.UseCount > 0 {
		lines = append(lines, "", MutedStyle.Render(fmt.Sprintf("Used %d times", t.UseCount)))
	}

	return lines
}

func (m *TaskTemplatesModel) slicePreviewLines(lines []string) []string {
	if m.previewScroll > 0 && m.previewScroll < len(lines) {
		return lines[m.previewScroll:]
	}
	if m.previewScroll >= len(lines) && len(lines) > 0 {
		m.previewScroll = len(lines) - 1
		return lines[m.previewScroll:]
	}
	return lines
}

func previewHeader(isDefault bool) string {
	header := BoldStyle.Render("Preview")
	if isDefault {
		header += " " + MutedStyle.Render("[Default]")
	}
	return header
}
