package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/template"
)

func (m *TemplatePickerModel) View() tea.View {
	totalWidth := m.Width() - 4

	// Get selected template for preview
	var selectedTemplate *template.TaskTemplate
	if m.cursor < len(m.filteredTemplates) {
		t := m.filteredTemplates[m.cursor]
		selectedTemplate = &t
	}

	var content string
	var helpText string

	// Calculate content height (total height minus header, search, help footer)
	// Header: 2 lines, Search: 2 lines, Help: 2 lines = 6 lines overhead
	contentHeight := max(m.Height()-10, 5)

	if totalWidth >= minWidthForSplitView {
		// Wide mode: split view
		listWidth := (totalWidth * 40) / 100
		previewWidth := totalWidth - listWidth - 1 // -1 for separator

		leftPanel := m.renderListPanel(listWidth)
		rightPanel := m.renderPreviewPanel(selectedTemplate, previewWidth, contentHeight)
		content = renderOverlaySplitPane(leftPanel, rightPanel, listWidth, previewWidth, contentHeight)
		helpText = "\u2191/\u2193: select \u00b7 Enter: use \u00b7 Esc: close \u00b7 Ctrl+U/D: scroll"
	} else {
		// Narrow mode: single pane with TAB toggle
		if m.showingPreview {
			content = m.renderPreviewPanel(selectedTemplate, totalWidth, contentHeight)
			helpText = "Tab: back to list \u00b7 Ctrl+U/D: scroll \u00b7 Esc: close"
		} else {
			content = m.renderListPanel(totalWidth)
			helpText = "\u2191/\u2193: select \u00b7 Enter: use \u00b7 Tab: preview \u00b7 Esc: close"
		}
	}

	// Build final view
	var b strings.Builder

	b.WriteString(BoldStyle.Render("Select a Template"))
	b.WriteString("\n\n")

	// Search input
	b.WriteString("/")
	b.WriteString(m.SearchInputView())
	b.WriteString("\n\n")

	// Main content
	b.WriteString(content)
	b.WriteString("\n\n")

	// Help footer
	b.WriteString(HelpStyle.Render(helpText))

	// Wrap in a box
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorSubtleBorder).
		Padding(1, 2).
		Width(m.Width() - 4)

	return tea.NewView(boxStyle.Render(b.String()))
}

// HasTextInput returns true when search is active.
func (m *TemplatePickerModel) HasTextInput() bool {
	return m.DefaultHasTextInput()
}

// ShouldCaptureEsc returns true to handle escape internally.
func (m *TemplatePickerModel) ShouldCaptureEsc() bool {
	return true
}

func (m *TemplatePickerModel) StatusBarLabel() string { return "Choose Template" }
