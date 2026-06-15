package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/template"
)

func (m *TaskTemplatesModel) View() tea.View {
	var b strings.Builder

	if m.expandedView && !m.isWideScreen() && len(m.filteredTemplates) > 0 && m.cursor < len(m.filteredTemplates) {
		return tea.NewView(m.renderFullscreenPreview())
	}

	b.WriteString(renderFullscreenTitle("Task Templates", fmt.Sprintf("%d templates", len(m.filteredTemplates)), m.AccentColor()))
	b.WriteString("\n\n")

	if m.searchActive || m.searchQuery != "" {
		b.WriteString("/")
		b.WriteString(m.SearchInputView())
		if m.searchQuery != "" && !m.searchActive {
			b.WriteString(MutedStyle.Render(fmt.Sprintf(" (%d matches)", len(m.filteredTemplates))))
		}
		b.WriteString("\n\n")
	}

	availableHeight := max(m.Height()-TabbedViewChrome, 1)
	if m.searchActive || m.searchQuery != "" {
		availableHeight -= 2
	}
	if availableHeight < 5 {
		availableHeight = 5
	}

	listContent := m.renderTemplatesList(availableHeight)
	if m.isWideScreen() {
		listWidth := m.Width() * 45 / 100
		previewWidth := m.Width() - listWidth - SeparatorWidth

		listPane := lipgloss.NewStyle().
			Width(listWidth).
			Height(availableHeight).
			Render(listContent)

		var previewPane string
		if len(m.filteredTemplates) > 0 && m.cursor < len(m.filteredTemplates) {
			previewPane = m.renderPreviewPane(previewWidth, availableHeight)
		} else {
			previewPane = lipgloss.NewStyle().
				Width(previewWidth).
				Foreground(ColorMuted).
				Render("No template selected")
		}
		previewPane = lipgloss.NewStyle().
			Width(previewWidth).
			Height(availableHeight).
			Render(previewPane)

		separator := SeparatorStyle.Render(strings.Repeat("┃\n", availableHeight))
		mainContent := lipgloss.JoinHorizontal(lipgloss.Top, listPane, " ", separator, " ", previewPane)
		b.WriteString(mainContent)
	} else {
		b.WriteString(listContent)
	}

	b.WriteString("\n")
	b.WriteString(renderStatusBarFooter(HelpStyle.Render(m.renderHelpText()), "", m.Width(), m.AccentColor()))

	return tea.NewView(b.String())
}

// renderTemplatesList renders the list of templates.
func (m *TaskTemplatesModel) renderTemplatesList(paneHeight int) string {
	var b strings.Builder

	if len(m.filteredTemplates) == 0 {
		if m.searchQuery != "" {
			b.WriteString(MutedStyle.Render("No templates match your search. Press Esc to clear search."))
		} else {
			b.WriteString(MutedStyle.Render("No task templates configured. Use 'tt template init' to add defaults."))
		}
		return b.String()
	}

	maxVisible := max(paneHeight, 3)
	startIdx := 0
	if m.cursor >= maxVisible {
		startIdx = m.cursor - maxVisible + 1
	}
	endIdx := min(startIdx+maxVisible, len(m.filteredTemplates))
	scrollbar := RenderScrollbar(len(m.filteredTemplates), maxVisible, startIdx)
	listWidth := m.listContentWidth()
	if scrollbar != "" {
		listWidth = max(listWidth-1, 1)
	}
	maxNameLen := m.listNameWidth(listWidth)

	var rowBuilder strings.Builder
	for i := startIdx; i < endIdx; i++ {
		t := m.filteredTemplates[i]
		isDefault := m.isDefaultTemplate(t.ID)

		status := MutedStyle.Render("○")
		if t.Enabled {
			status = DoneStyle.Render("●")
		}

		defaultMarker := ""
		effectiveMaxLen := maxNameLen
		if isDefault {
			defaultMarker = MutedStyle.Render("[D]") + " "
			effectiveMaxLen -= 4
		}

		name := truncateToVisualWidth(t.Name, max(effectiveMaxLen, 1))
		if i == m.cursor {
			m.renderSelectedTemplateRow(&rowBuilder, status, defaultMarker, name, t)
			continue
		}

		fmt.Fprintf(&rowBuilder, "  %s %s%s\n", status, defaultMarker, name)
	}

	b.WriteString(attachScrollbarAtWidth(rowBuilder.String(), scrollbar, listWidth))

	return b.String()
}

func (m *TaskTemplatesModel) listContentWidth() int {
	if m.isWideScreen() {
		return m.Width() * 45 / 100
	}
	return m.Width()
}

func (m *TaskTemplatesModel) listNameWidth(contentWidth int) int {
	if m.isWideScreen() {
		if contentWidth-7 < 15 {
			return 15
		}
		return contentWidth - 7
	}
	if contentWidth-9 < 15 {
		return 15
	}
	return contentWidth - 9
}

func (m *TaskTemplatesModel) renderSelectedTemplateRow(b *strings.Builder, status, defaultMarker, name string, t template.TaskTemplate) {
	b.WriteString(SelectedItemTextStyle.Render(fmt.Sprintf("%s %s %s%s", IconCursorRight, status, defaultMarker, name)))
	b.WriteString("\n")

	if m.isWideScreen() {
		return
	}

	summary := truncateToVisualWidth(t.Summary(), max(m.Width()-8, 1))
	fmt.Fprintf(b, "    %s\n", MutedStyle.Render(summary))
}

// renderHelpText renders the help text for the footer, wrapping if necessary.
func (m *TaskTemplatesModel) renderHelpText() string {
	var parts []string

	if len(m.filteredTemplates) == 0 {
		if m.searchQuery == "" {
			parts = []string{"/: search", "n: new template", "Esc: close"}
		} else {
			parts = []string{"Esc: clear search", "n: new template", "Esc: close"}
		}
	} else if m.isWideScreen() {
		parts = []string{"/: search", "j/k: nav", "Ctrl+D/U: scroll", "Space: toggle", "n: new", "e: edit", "d: delete", "Esc: close"}
	} else {
		parts = []string{"/: search", "j/k: nav", "Enter: preview", "Space: toggle", "n: new", "e: edit", "d: delete", "Esc: close"}
	}

	return renderWrappingHelpLine(strings.Join(parts, " · "), m.Width()-2)
}

// GetHelpItems returns help items for this view.
func (m *TaskTemplatesModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewTaskTemplates)
}

// HasTextInput returns true when search is active.
func (m *TaskTemplatesModel) HasTextInput() bool {
	return m.searchActive
}

// ShouldCaptureEsc returns true when expanded view, delete confirmation, or search is active.
func (m *TaskTemplatesModel) ShouldCaptureEsc() bool {
	return m.expandedView || m.DefaultShouldCaptureEsc() || m.SearchShouldCaptureEsc() || m.searchQuery != ""
}
