package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/storage"
)

func (m *UndoHistoryModel) View() tea.View {
	if m.showingDetail {
		return tea.NewView(m.renderDetailView())
	}

	var b strings.Builder
	b.WriteString(renderFullscreenTitle("Undo History", "", m.AccentColor()))
	b.WriteString("\n\n")

	if m.HasError() {
		b.WriteString(ErrorIconStyle.Render(m.Error().Error()))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Press Esc to close"))
		return tea.NewView(b.String())
	}

	if len(m.entries) == 0 {
		b.WriteString(MutedStyle.Render("No undo history available."))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Press Esc to close"))
		return tea.NewView(b.String())
	}

	listWidth := max(m.Width()*35/100, 30)
	diffWidth := m.Width() - listWidth - SeparatorWidth
	availableHeight := max(m.Height()-TabbedViewChrome, 1)

	listContent := m.renderEntryList(listWidth, availableHeight)
	diffContent := m.renderDiffPreview(diffWidth, availableHeight)
	separator := SeparatorStyle.Render(strings.Repeat("┃\n", availableHeight))

	row := lipgloss.JoinHorizontal(lipgloss.Top, listContent, " ", separator, " ", diffContent)
	b.WriteString(row)
	b.WriteString("\n")
	b.WriteString(HelpStyle.Render(renderWrappingHelpLine(m.helpText(), m.Width())))

	return tea.NewView(b.String())
}

func (m *UndoHistoryModel) helpText() string {
	helpParts := []string{"j/k: navigate"}
	if m.focusedPane == 1 {
		helpParts = append(helpParts, "h/l: scroll left/right", "Enter: view full line")
	} else {
		helpParts = append(helpParts, "Enter: undo most recent")
	}
	helpParts = append(helpParts, "Tab: switch pane", "Esc: close")
	return strings.Join(helpParts, " · ")
}

func (m *UndoHistoryModel) renderEntryList(width, height int) string {
	var b strings.Builder

	headerStyle := BoldSecondaryStyle
	if m.focusedPane == 0 {
		headerStyle = BoldPrimaryUnderlineStyle
	}
	b.WriteString(headerStyle.Render("Actions"))
	b.WriteString("\n")

	visibleEntries := max((height-2)/2, 1)
	startIdx := 0
	if m.cursor >= visibleEntries {
		startIdx = m.cursor - visibleEntries + 1
	}

	endIdx := min(startIdx+visibleEntries, len(m.entries))
	for i := startIdx; i < endIdx; i++ {
		entry := m.entries[i]
		actionText := entry.Action
		if entry.Context != "" {
			actionText = fmt.Sprintf("%s: %s", entry.Action, entry.Context)
		}

		maxLen := width - 4
		if len(actionText) > maxLen {
			actionText = actionText[:maxLen-3] + "..."
		}

		line := "  " + actionText
		if i == m.cursor {
			line = BoldStyle.Render(line)
		}

		b.WriteString(line)
		b.WriteString("\n")
		fmt.Fprintf(&b, "   %s", MutedStyle.Render(storage.FormatRelativeTime(entry.Timestamp)))
		b.WriteString("\n")
	}

	if len(m.entries) > visibleEntries {
		remaining := len(m.entries) - endIdx
		if remaining > 0 {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("  ... %d more", remaining)))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m *UndoHistoryModel) renderDiffPreview(width, height int) string {
	var b strings.Builder

	headerStyle := BoldSecondaryStyle
	if m.focusedPane == 1 {
		headerStyle = BoldPrimaryUnderlineStyle
	}
	b.WriteString(headerStyle.Render("What Changed"))
	b.WriteString("\n")

	if len(m.diffLines) == 0 {
		b.WriteString(MutedStyle.Render("No changes to show"))
		return b.String()
	}

	added, removed := SummarizeDiff(m.diffLines)
	summaryText := fmt.Sprintf("+%d / -%d lines", added, removed)
	if m.diffHScroll > 0 {
		summaryText += fmt.Sprintf(" (h-scroll: %d)", m.diffHScroll)
	}
	b.WriteString(MutedStyle.Render(summaryText))
	b.WriteString("\n\n")

	visibleLines := max(height-5, 1)
	startIdx := m.diffScroll
	if startIdx >= len(m.diffLines) {
		startIdx = max(len(m.diffLines)-1, 0)
	}

	endIdx := min(startIdx+visibleLines, len(m.diffLines))
	contentWidth := width - 3

	for i := startIdx; i < endIdx; i++ {
		b.WriteString(m.renderDiffLine(i, contentWidth))
		b.WriteString("\n")
	}

	if len(m.diffLines) > visibleLines {
		remaining := len(m.diffLines) - endIdx
		if remaining > 0 {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("... %d more lines", remaining)))
		}
	}

	return b.String()
}

func (m *UndoHistoryModel) renderDiffLine(index, contentWidth int) string {
	line := m.diffLines[index]
	content := m.visibleDiffContent(line.Content, contentWidth)
	isSelected := m.focusedPane == 1 && index == m.diffCursor

	lineStyle, prefix := m.diffLineStyle(line)
	if isSelected {
		lineStyle = lineStyle.Background(ColorSelectedBg)
	}

	return lineStyle.Render(prefix + content)
}

func (m *UndoHistoryModel) visibleDiffContent(content string, contentWidth int) string {
	if m.diffHScroll > 0 {
		if len(content) > m.diffHScroll {
			content = content[m.diffHScroll:]
		} else {
			content = ""
		}
	}

	hasMore := len(content) > contentWidth
	if len(content) > contentWidth {
		content = content[:contentWidth]
	}
	if hasMore && len(content) >= 3 {
		content = content[:len(content)-1] + "→"
	}

	return content
}

func (m *UndoHistoryModel) diffLineStyle(line DiffLine) (lipgloss.Style, string) {
	switch line.Kind {
	case DiffRemove:
		return DiffRemoveStyle, "- "
	case DiffAdd:
		return DiffAddStyle, "+ "
	default:
		return DiffContextStyle, "  "
	}
}
