package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/workflow"
)

func (m *WorkflowRulesModel) View() tea.View {
	var b strings.Builder

	if m.expandedView && !m.isWideScreen() && len(m.filteredRules) > 0 && m.cursor < len(m.filteredRules) {
		return tea.NewView(m.renderFullscreenPreview())
	}

	b.WriteString(renderFullscreenTitle("Workflow Rules", fmt.Sprintf("%d rules", len(m.filteredRules)), m.AccentColor()))
	b.WriteString("\n\n")

	if m.searchActive || m.searchQuery != "" {
		b.WriteString("/")
		b.WriteString(m.SearchInputView())
		if m.searchQuery != "" && !m.searchActive {
			b.WriteString(MutedStyle.Render(fmt.Sprintf(" (%d matches)", len(m.filteredRules))))
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

	listContent := m.renderRulesList(availableHeight)
	if m.isWideScreen() {
		listWidth := m.Width() * 45 / 100
		previewWidth := m.Width() - listWidth - SeparatorWidth

		listPane := lipgloss.NewStyle().
			Width(listWidth).
			Height(availableHeight).
			Render(listContent)

		var previewPane string
		if len(m.filteredRules) > 0 && m.cursor < len(m.filteredRules) {
			previewPane = m.renderPreviewPane(previewWidth)
		} else {
			previewPane = lipgloss.NewStyle().
				Width(previewWidth).
				Foreground(ColorMuted).
				Render("No rule selected")
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

// renderRulesList renders the list of rules.
func (m *WorkflowRulesModel) renderRulesList(paneHeight int) string {
	var b strings.Builder

	if len(m.filteredRules) == 0 {
		if m.searchQuery != "" {
			b.WriteString(MutedStyle.Render("No rules match your search."))
			b.WriteString("\n\n")
			b.WriteString(MutedStyle.Render("Press Esc to clear search."))
		} else {
			b.WriteString(MutedStyle.Render("No workflow rules configured."))
			b.WriteString("\n\n")
			for i, line := range wrapText("Use 'tt workflow add' to create rules.", m.listContentWidth()) {
				if i > 0 {
					b.WriteString("\n")
				}
				b.WriteString(MutedStyle.Render(line))
			}
		}
		return b.String()
	}

	maxVisible := max(paneHeight, 3)
	startIdx := 0
	if m.cursor >= maxVisible {
		startIdx = m.cursor - maxVisible + 1
	}
	endIdx := min(startIdx+maxVisible, len(m.filteredRules))

	scrollbar := RenderScrollbar(len(m.filteredRules), maxVisible, startIdx)
	listWidth := m.listContentWidth()
	if scrollbar != "" {
		listWidth = max(listWidth-1, 1)
	}
	maxNameLen := m.listNameWidth(listWidth)

	var rowBuilder strings.Builder
	for i := startIdx; i < endIdx; i++ {
		rule := m.filteredRules[i]
		isDefault := m.isDefaultRule(rule.ID)

		status := MutedStyle.Render("○")
		if rule.Enabled {
			status = DoneStyle.Render("●")
		}

		defaultMarker := ""
		effectiveMaxLen := maxNameLen
		if rule.IsProjectLevel() {
			defaultMarker += MutedStyle.Render("[P]") + " "
			effectiveMaxLen -= 4
		}
		if isDefault {
			defaultMarker += MutedStyle.Render("[D]") + " "
			effectiveMaxLen -= 4
		}

		name := truncateToVisualWidth(rule.Name, max(effectiveMaxLen, 1))
		if i == m.cursor {
			m.renderSelectedRuleRow(&rowBuilder, status, defaultMarker, name, rule)
			continue
		}

		nameHighlighted := name
		if m.searchQuery != "" {
			nameHighlighted = HighlightSubstring(name, m.searchQuery, NormalStyle)
		}
		fmt.Fprintf(&rowBuilder, "  %s %s%s\n", status, defaultMarker, nameHighlighted)
	}

	b.WriteString(attachScrollbarAtWidth(rowBuilder.String(), scrollbar, listWidth))

	return b.String()
}

func (m *WorkflowRulesModel) listContentWidth() int {
	if m.isWideScreen() {
		return m.Width() * 45 / 100
	}
	return m.Width()
}

func (m *WorkflowRulesModel) listNameWidth(contentWidth int) int {
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

func (m *WorkflowRulesModel) renderSelectedRuleRow(b *strings.Builder, status, defaultMarker, name string, rule workflow.WorkflowRule) {
	prefix := fmt.Sprintf("%s %s %s", IconCursorRight, status, defaultMarker)
	nameHighlighted := SelectedItemTextStyle.Render(name)
	if m.searchQuery != "" {
		nameHighlighted = HighlightSubstring(name, m.searchQuery, SelectedItemTextStyle)
	}
	b.WriteString(SelectedItemTextStyle.Render(prefix) + nameHighlighted)
	b.WriteString("\n")

	if m.isWideScreen() {
		return
	}

	summary := truncateToVisualWidth(rule.Summary(), max(m.Width()-8, 1))
	summaryRendered := MutedStyle.Render(summary)
	if m.searchQuery != "" {
		summaryRendered = HighlightSubstring(summary, m.searchQuery, MutedStyle)
	}
	fmt.Fprintf(b, "    %s\n", summaryRendered)
}

// renderHelpText renders the help text for the footer, wrapping if necessary.
func (m *WorkflowRulesModel) renderHelpText() string {
	var parts []string

	if len(m.filteredRules) == 0 {
		if m.searchQuery == "" {
			parts = []string{"/: search", "n: new rule", "w: wizard", "Esc: close"}
		} else {
			parts = []string{"Esc: clear search", "n: new rule", "w: wizard", "Esc: close"}
		}
	} else if m.isWideScreen() {
		parts = []string{"/: search", "j/k: nav", "J/K: priority", "Space: toggle", "n: new", "w: wizard", "e: edit", "d: delete", "t: dry run", "T: test rule", "Esc: close"}
	} else {
		parts = []string{"/: search", "j/k: nav", "J/K: priority", "Enter: preview", "Space: toggle", "n: new", "w: wizard", "e: edit", "d: delete", "t: dry run", "Esc: close"}
	}

	return renderWrappingHelpLine(strings.Join(parts, " · "), m.Width()-2)
}

// GetHelpItems returns help items for this view.
func (m *WorkflowRulesModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewWorkflowRules)
}

// HasTextInput returns true when search is active.
func (m *WorkflowRulesModel) HasTextInput() bool {
	return m.searchActive
}

// ShouldCaptureEsc returns true when expanded view, delete confirmation, or search is active.
func (m *WorkflowRulesModel) ShouldCaptureEsc() bool {
	return m.expandedView || m.DefaultShouldCaptureEsc() || m.SearchShouldCaptureEsc() || m.searchQuery != ""
}

func (m *WorkflowRulesModel) StatusBarLabel() string { return "Workflow Rules" }

// IsFullHeight returns true because the workflow rules view renders its own footer.
func (m *WorkflowRulesModel) IsFullHeight() bool { return true }
