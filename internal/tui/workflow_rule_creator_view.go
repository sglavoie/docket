package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"charm.land/lipgloss/v2"
)

// View renders the workflow rule creator interface.
func (m *WorkflowRuleCreatorModel) View() tea.View {
	// Render picker overlays if active
	if m.showingFieldPicker && m.fieldPicker != nil {
		return m.fieldPicker.View()
	}
	if m.showingActionPicker && m.actionPicker != nil {
		return m.actionPicker.View()
	}

	// Narrow screen with preview toggle: show fullscreen preview
	if !m.isWideScreen() && m.previewShown {
		return tea.NewView(m.renderFullscreenPreview())
	}

	title := "Edit Workflow Rule"
	if m.editingRule == nil {
		title = "Create Workflow Rule"
	}

	// Tab bar
	tabs := []string{"Triggers", "Actions", "Scope"}
	tabBar := RenderTabBar(tabs, m.activeTab, m.width, 1)

	availableHeight := max(m.height-7, 1)

	// Editor content with scrolling
	editorContent := m.renderEditorPane()
	editorContent = m.applyEditorScroll(editorContent, availableHeight)

	var mainContent string
	if m.isWideScreen() {
		// Split layout: editor left, preview right
		editorWidth := m.width * 55 / 100
		previewWidth := m.width - editorWidth - SeparatorWidth

		// Calculate pane height
		paneHeight := max(availableHeight, 5)

		editorPane := lipgloss.NewStyle().
			Width(editorWidth).
			Height(paneHeight).
			Render(editorContent)

		previewPane := lipgloss.NewStyle().
			Width(previewWidth).
			Height(paneHeight).
			Render(m.renderPreviewPane(previewWidth))

		// Full-height separator
		separator := SeparatorStyle.Render(strings.Repeat("┃\n", paneHeight))

		mainContent = lipgloss.JoinHorizontal(lipgloss.Top, editorPane, " ", separator, " ", previewPane)
	} else {
		// Narrow: fullscreen editor only
		mainContent = editorContent
	}

	// Help text with wrapping support
	helpText := m.renderHelpText()

	var out strings.Builder
	out.WriteString(renderFullscreenTitle(title, ""))
	out.WriteString("\n")
	out.WriteString(tabBar)
	out.WriteString("\n\n")
	out.WriteString(mainContent)
	out.WriteString("\n")
	out.WriteString(renderStatusBarFooter(helpText, "", m.width))

	return tea.NewView(out.String())
}

// renderHelpText renders the help text, wrapping if necessary.
func (m *WorkflowRuleCreatorModel) renderHelpText() string {
	var parts []string
	if m.isWideScreen() {
		parts = []string{
			"[ ]: tabs",
			"Ctrl+F/T: pickers",
			"Tab: nav",
			"↑↓: items",
			"+/-: add/rem",
			"c: condition",
			"g: group",
			"Ctrl+S: save",
			"Ctrl+R: test",
			"Esc: cancel",
		}
	} else {
		parts = []string{
			"[ ]: tabs",
			"Ctrl+P: preview",
			"↑↓: items",
			"+/-: add/rem",
			"c: condition",
			"g: group",
			"Ctrl+S: save",
			"Ctrl+R: test",
			"Esc: cancel",
		}
	}

	// Try single line first
	singleLine := strings.Join(parts, " · ")
	if lipgloss.Width(singleLine) <= m.width-2 {
		return HelpStyle.Render(singleLine)
	}

	// Split into two lines
	mid := len(parts) / 2
	line1 := strings.Join(parts[:mid], " · ")
	line2 := strings.Join(parts[mid:], " · ")
	return HelpStyle.Render(line1) + "\n" + HelpStyle.Render(line2)
}

// applyEditorScroll applies vertical scrolling to the editor content.
func (m *WorkflowRuleCreatorModel) applyEditorScroll(content string, availableHeight int) string {
	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	if totalLines <= availableHeight {
		return content
	}

	// Ensure scroll offset is valid
	maxOffset := totalLines - availableHeight
	if m.editorScrollOffset > maxOffset {
		m.editorScrollOffset = maxOffset
	}
	if m.editorScrollOffset < 0 {
		m.editorScrollOffset = 0
	}

	// Extract visible lines
	endLine := min(m.editorScrollOffset+availableHeight, totalLines)
	visibleLines := lines[m.editorScrollOffset:endLine]

	return strings.Join(visibleLines, "\n")
}
