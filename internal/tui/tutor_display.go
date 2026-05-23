package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (m *TutorModel) renderTopicList() string {
	listW := m.listWidth()
	var lines []string

	cursorStyle := TitleStyle
	normalStyle := OverlayDetailStyle
	mutedCat := lipgloss.NewStyle().Foreground(ColorMuted).Bold(true)

	if len(m.filteredTopics) == 0 {
		return MutedStyle.Render("No topics found.")
	}

	// Build all lines with category headers
	lastCat := ""
	idx := 0
	for _, t := range m.filteredTopics {
		if t.Category != lastCat {
			if lastCat != "" {
				lines = append(lines, "")
			}
			header := truncateToVisualWidth(t.Category, listW)
			lines = append(lines, mutedCat.Render(header))
			lastCat = t.Category
		}

		title := truncateToVisualWidth(t.Title, listW-3)

		if idx == m.selectedIndex {
			if m.activePane == 0 {
				if m.searchQuery != "" {
					lines = append(lines, "\u25f8 "+HighlightSubstring(title, m.searchQuery, cursorStyle))
				} else {
					lines = append(lines, cursorStyle.Render("\u25f8 "+title))
				}
			} else {
				if m.searchQuery != "" {
					lines = append(lines, "\u25f8 "+HighlightSubstring(title, m.searchQuery, normalStyle.Bold(true)))
				} else {
					lines = append(lines, normalStyle.Bold(true).Render("\u25f8 "+title))
				}
			}
		} else {
			if m.searchQuery != "" {
				lines = append(lines, "  "+HighlightSubstring(title, m.searchQuery, normalStyle))
			} else {
				lines = append(lines, "  "+normalStyle.Render(title))
			}
		}
		idx++
	}

	// Apply scroll offset
	m.ensureSelectedVisible()
	h := m.listViewHeight()
	start := min(m.listOffset, len(lines))
	end := min(start+h, len(lines))
	visible := lines[start:end]

	return strings.Join(visible, "\n")
}

func (m *TutorModel) renderTopicContent() string {
	if len(m.filteredTopics) == 0 {
		return MutedStyle.Render("No topic selected.")
	}

	idx := m.selectedIndex
	if idx >= len(m.filteredTopics) {
		idx = len(m.filteredTopics) - 1
	}
	topic := m.filteredTopics[idx]

	contentW := max(
		// some padding
		m.contentWidth()-2, 20)

	return renderTutorContent(topic, contentW, m.searchQuery)
}

func (m *TutorModel) renderTabBar() string {
	tabNames := append([]string{"All"}, m.categories...)
	return renderWrappingTabBar(tabNames, m.activeTab, m.innerWidth())
}

func (m *TutorModel) View() tea.View {
	pad := " " // 1 char left padding
	var b strings.Builder

	// Tab bar
	b.WriteString(pad)
	b.WriteString(m.renderTabBar())
	b.WriteString("\n\n")

	// Search bar
	if m.searchMode {
		b.WriteString(pad)
		b.WriteString(SubtitleStyle.Render("Search: "))
		b.WriteString(m.searchInput.View())
		b.WriteString("\n\n")
	} else if m.searchQuery != "" {
		filterInfo := fmt.Sprintf("Search: \"%s\" ", m.searchQuery)
		b.WriteString(pad)
		b.WriteString(HelpFilterActiveStyle.Render(filterInfo))
		b.WriteString(HelpStyle.Render("(/ to edit, Esc to clear)"))
		b.WriteString("\n\n")
	}

	// Split pane
	if m.ready {
		leftContent := m.renderTopicList()
		rightContent := m.viewport.View()

		lw := m.listWidth()
		rw := m.innerWidth() - lw - 1
		vpH := m.viewport.Height()

		// Focus indicator bar
		leftColor := ColorBorder
		rightColor := ColorBorder
		if m.activePane == 0 {
			leftColor = ColorPrimary
		} else {
			rightColor = ColorPrimary
		}
		leftBar := renderHorizontalDivider(lw, WithDividerColor(leftColor))
		rightBar := renderHorizontalDivider(rw, WithDividerColor(rightColor))
		divider := lipgloss.NewStyle().Foreground(ColorBorder).Render("\u2502")
		b.WriteString(pad)
		b.WriteString(leftBar + divider + rightBar)
		b.WriteString("\n")

		pane := renderOverlaySplitPane(leftContent, rightContent, lw, rw, vpH)
		// Add left padding to each line of the split pane
		for i, line := range strings.Split(pane, "\n") {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(pad)
			b.WriteString(line)
		}
	}

	// Footer
	b.WriteString("\n")
	paneHint := "list"
	if m.activePane == 1 {
		paneHint = "content"
	}
	leftFooter := HelpStyle.Render(fmt.Sprintf("Tab: switch pane (%s) \u00b7 shift+\u2190/\u2192: tabs \u00b7 /: search \u00b7 j/k: navigate \u00b7 Esc: close", paneHint))
	rightFooter := ""
	if m.ready && m.viewport.TotalLineCount() > m.viewport.Height() {
		rightFooter = MutedStyle.Render(fmt.Sprintf("%3.0f%%", m.viewport.ScrollPercent()*100))
	}
	b.WriteString(pad)
	b.WriteString(renderStatusBarFooter(leftFooter, rightFooter, m.innerWidth()))

	return tea.NewView(b.String())
}
