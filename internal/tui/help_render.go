package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// prefixGroupOrder defines the display order for prefix groups in "by prefix" mode.
var prefixGroupOrder = []string{
	",", "!", ".", "d", "s", "c", "m", "t", "b", "z", "F",
}

// prefixGroupLabels maps registry Category names to their prefix group key.
// Categories not present here are shown under "Direct Keys".
var prefixGroupLabels = map[string]string{
	"Status":              ",",
	"Recurrence":          ",",
	"Priority":            "!",
	"Stage (. prefix)":    ".",
	"Due Date":            "d",
	"Snooze":              "s",
	"Linked Tasks":        "c",
	"Task (t prefix)":     "t",
	"Folds":               "z",
	"Move (m prefix)":     "m",
	"Blockers (b prefix)": "b",
	"Filters":             "F",
}

// renderByPrefixContent renders the help items grouped by prefix category.
func (m *HelpViewModel) renderByPrefixContent() string {
	if len(m.filteredItems) == 0 {
		return MutedStyle.Render("No matching help items.")
	}

	contentWidth := max(m.viewport.Width(), 1)
	keyWidth := 16
	columnGap := 4
	columnOverhead := keyWidth + 2
	descWidth := max((contentWidth-2*columnOverhead-columnGap)/2, 1)
	columnWidth := keyWidth + 2 + descWidth

	keyStyle := lipgloss.NewStyle().
		Width(keyWidth).
		Align(lipgloss.Right).
		Foreground(ColorSecondary)
	columnStyle := lipgloss.NewStyle().Width(columnWidth)

	renderItem := func(item filteredHelpItem) string {
		var renderedKey string
		if len(item.keyPositions) > 0 {
			renderedKey = keyStyle.Render(HighlightText(item.Key, item.keyPositions, TagInlineStyle))
		} else {
			renderedKey = keyStyle.Render(item.Key)
		}
		descText := item.Description
		scopeSuffix := ""
		if m.isExplorer {
			scopeSuffix = "  · " + scopeText(item.Views)
			descText = item.Description + scopeSuffix
		}
		wrappedDesc := wrapDescriptionWithIndent(descText, descWidth, keyWidth+2)
		descLines := strings.Split(wrappedDesc, "\n")
		if scopeSuffix != "" {
			descLines = styleScopeLines(descLines)
		}
		var result strings.Builder
		for i, line := range descLines {
			if i == 0 {
				result.WriteString(renderedKey)
				result.WriteString("  ")
				if len(item.descPositions) > 0 {
					result.WriteString(HighlightText(line, item.descPositions, NormalStyle))
				} else {
					result.WriteString(line)
				}
			} else {
				result.WriteString("\n")
				result.WriteString(line)
			}
		}
		return result.String()
	}

	// Bucket items by prefix group key
	buckets := make(map[string][]filteredHelpItem)
	var directItems []filteredHelpItem
	for _, item := range m.filteredItems {
		if group, ok := prefixGroupLabels[item.Category]; ok {
			buckets[group] = append(buckets[group], item)
		} else {
			directItems = append(directItems, item)
		}
	}

	var b strings.Builder
	first := true

	renderSection := func(header string, items []filteredHelpItem) {
		if len(items) == 0 {
			return
		}
		if !first {
			b.WriteString("\n")
		}
		first = false
		b.WriteString(HelpCategoryStyle.Render(header))
		b.WriteString("\n")
		for j := 0; j < len(items); j += 2 {
			col1 := columnStyle.Render(renderItem(items[j]))
			var col2 string
			if j+1 < len(items) {
				col2 = columnStyle.Render(renderItem(items[j+1]))
			}
			row := lipgloss.JoinHorizontal(lipgloss.Top, col1, strings.Repeat(" ", columnGap), col2)
			b.WriteString(row)
			b.WriteString("\n")
		}
	}

	for _, prefix := range prefixGroupOrder {
		items, ok := buckets[prefix]
		if !ok {
			continue
		}
		catName := prefixCategoryNames[prefix]
		header := fmt.Sprintf("%s  (%s)", catName, prefix)
		renderSection(header, items)
	}
	if len(directItems) > 0 {
		renderSection("direct keys", directItems)
	}

	return b.String()
}

func (m *HelpViewModel) renderContent() string {
	if m.byPrefixMode {
		return m.renderByPrefixContent()
	}

	if len(m.filteredItems) == 0 {
		return MutedStyle.Render("No matching help items.")
	}

	var b strings.Builder

	// Group items by category
	categories := make(map[string][]filteredHelpItem)
	var categoryOrder []string

	for _, item := range m.filteredItems {
		if _, exists := categories[item.Category]; !exists {
			categoryOrder = append(categoryOrder, item.Category)
		}
		categories[item.Category] = append(categories[item.Category], item)
	}

	// Calculate column widths based on viewport width (not full width)
	// Each column: keyWidth + separator(2) + descWidth
	// Total: 2 * (keyWidth + 2 + descWidth) + columnGap(4)
	contentWidth := max(m.viewport.Width(), 1)
	keyWidth := 16
	columnGap := 4
	columnOverhead := keyWidth + 2 // key + separator
	descWidth := max((contentWidth-2*columnOverhead-columnGap)/2, 1)
	columnWidth := keyWidth + 2 + descWidth

	keyStyle := lipgloss.NewStyle().
		Width(keyWidth).
		Align(lipgloss.Right).
		Foreground(ColorSecondary)

	// columnStyle ensures entire column block has fixed width for proper alignment
	columnStyle := lipgloss.NewStyle().Width(columnWidth)

	// renderHelpItem renders a single key+description pair with proper wrapping
	renderHelpItem := func(key, desc string, keyPos, descPos []int, views []ViewType) string {
		// Render key with highlight if positions present
		var renderedKey string
		if len(keyPos) > 0 {
			renderedKey = keyStyle.Render(HighlightText(key, keyPos, TagInlineStyle))
		} else {
			renderedKey = keyStyle.Render(key)
		}

		// Build description text, appending plain scope suffix for proper wrapping
		descText := desc
		scopeSuffix := ""
		if m.isExplorer {
			scopeSuffix = "  · " + scopeText(views)
			descText = desc + scopeSuffix
		}

		// Wrap description with indentation for continuation lines
		// Indent = keyWidth + separator(2) so wrapped text aligns under description
		wrappedDesc := wrapDescriptionWithIndent(descText, descWidth, keyWidth+2)

		// Split into lines, apply scope styling across all lines
		descLines := strings.Split(wrappedDesc, "\n")
		if scopeSuffix != "" {
			descLines = styleScopeLines(descLines)
		}
		var result strings.Builder

		for i, line := range descLines {
			if i == 0 {
				// First line: key + separator + description
				result.WriteString(renderedKey)
				result.WriteString("  ")
				if len(descPos) > 0 {
					result.WriteString(HighlightText(line, descPos, NormalStyle))
				} else {
					result.WriteString(line)
				}
			} else {
				// Continuation lines are already indented by wrapDescriptionWithIndent
				result.WriteString("\n")
				result.WriteString(line)
			}
		}

		return result.String()
	}

	// Render each category
	for i, category := range categoryOrder {
		if i > 0 {
			b.WriteString("\n")
		}

		header := HelpCategoryStyle.Render(category)
		b.WriteString(header)
		b.WriteString("\n")

		items := categories[category]

		// Render items in 2-column layout
		for j := 0; j < len(items); j += 2 {
			item1 := items[j]
			col1 := columnStyle.Render(renderHelpItem(item1.Key, item1.Description, item1.keyPositions, item1.descPositions, item1.Views))

			var col2 string
			if j+1 < len(items) {
				item2 := items[j+1]
				col2 = columnStyle.Render(renderHelpItem(item2.Key, item2.Description, item2.keyPositions, item2.descPositions, item2.Views))
			}

			row := lipgloss.JoinHorizontal(lipgloss.Top, col1, strings.Repeat(" ", columnGap), col2)
			b.WriteString(row)
			b.WriteString("\n")
		}
	}

	return b.String()
}

// tabBarLines returns the number of visual lines the tab bar occupies.
func (m *HelpViewModel) tabBarLines() int {
	return lipgloss.Height(m.renderHelpTabBar())
}

func (m *HelpViewModel) renderHelpTabBar() string {
	tabNames := append([]string{"All"}, m.categories...)
	return renderWrappingTabBar(tabNames, m.activeTab, m.innerWidth())
}

func (m *HelpViewModel) View() tea.View {
	pad := " "
	var b strings.Builder

	// Tab bar
	b.WriteString(pad)
	b.WriteString(m.renderHelpTabBar())
	b.WriteString("\n")

	// Tab search bar
	if m.tabSearchMode {
		b.WriteString(pad)
		b.WriteString(SubtitleStyle.Render("Tab: "))
		b.WriteString(m.tabSearchInput.View())
		b.WriteString("\n\n")
	}

	// Search bar
	if m.searchMode {
		b.WriteString(pad)
		b.WriteString(SubtitleStyle.Render("Filter: "))
		b.WriteString(m.searchInput.View())
		b.WriteString("\n\n")
	} else if m.searchQuery != "" {
		filterInfo := fmt.Sprintf("Filter: \"%s\" ", m.searchQuery)
		b.WriteString(pad)
		b.WriteString(HelpFilterActiveStyle.Render(filterInfo))
		b.WriteString(HelpStyle.Render("(/ to edit, Esc to clear)"))
		b.WriteString("\n\n")
	}

	// Viewport with content
	if m.ready {
		for i, line := range strings.Split(m.viewport.View(), "\n") {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(pad)
			b.WriteString(line)
		}
	}

	// Status bar footer
	b.WriteString("\n")

	title := fmt.Sprintf("Help - %s", m.viewName)
	closeKey := "? close"
	if m.isExplorer {
		title = "Keybinding Explorer"
		closeKey = "ctrl+\\ close"
	}
	left := StatusBarStyle.Render(title)

	scrollPercent := ""
	if m.ready && m.viewport.TotalLineCount() > m.viewport.Height() {
		scrollPercent = fmt.Sprintf(" %3.0f%%", m.viewport.ScrollPercent()*100)
	}
	explorerHints := ""
	if m.isExplorer {
		prefixLabel := "by prefix"
		if m.byPrefixMode {
			prefixLabel = "by category"
		}
		explorerHints = fmt.Sprintf(" · p: %s", prefixLabel)
	}
	right := HelpStyle.Render(fmt.Sprintf("shift+←/→: tabs · \\ find tab · / filter · bksp clear · j/k scroll · %s%s%s", closeKey, explorerHints, scrollPercent))

	b.WriteString(renderStatusBarFooter(left, right, m.width))

	return tea.NewView(b.String())
}

func (m *HelpViewModel) StatusBarLabel() string {
	if m.isExplorer {
		return "Explorer"
	}
	return "Help"
}

// HandledKeys returns the keys handled by the keybinding explorer view.
func (m *HelpViewModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}

// wrapDescriptionWithIndent wraps text to fit within width, indenting continuation
// lines to align under the first line of text (after key + separator).
func wrapDescriptionWithIndent(text string, width, indentWidth int) string {
	if width <= 0 {
		return text
	}

	indent := strings.Repeat(" ", indentWidth)
	var lines []string
	remaining := text

	for len(remaining) > 0 {
		// All lines use full width - indent is added as prefix, not subtracted from width
		if len(remaining) <= width {
			if len(lines) > 0 {
				lines = append(lines, indent+remaining)
			} else {
				lines = append(lines, remaining)
			}
			break
		}

		// Find break point (last space within width)
		breakPoint := width
		for i := width; i > 0; i-- {
			if remaining[i-1] == ' ' {
				breakPoint = i - 1
				break
			}
		}
		// If no space found, force break at width
		if breakPoint == width {
			breakPoint = width
		}

		line := remaining[:breakPoint]
		if len(lines) > 0 {
			lines = append(lines, indent+line)
		} else {
			lines = append(lines, line)
		}

		// Skip past the break point and any leading spaces
		remaining = remaining[breakPoint:]
		remaining = strings.TrimLeft(remaining, " ")
	}

	return strings.Join(lines, "\n")
}

// styleScopeLines applies MutedStyle to scope text across wrapped lines.
// Once the "· " marker is found, everything from that point onward (including
// subsequent continuation lines) is styled as muted.
func styleScopeLines(lines []string) []string {
	inScope := false
	for i, line := range lines {
		if inScope {
			// Entire line is scope continuation — style it all (preserve indent)
			trimmed := strings.TrimLeft(line, " ")
			indent := len(line) - len(trimmed)
			lines[i] = strings.Repeat(" ", indent) + MutedStyle.Render(trimmed)
			continue
		}
		trimmed := strings.TrimLeft(line, " ")
		markerIdx := strings.Index(trimmed, "·")
		if markerIdx >= 0 {
			indent := len(line) - len(trimmed)
			absIdx := indent + markerIdx
			lines[i] = line[:absIdx] + MutedStyle.Render(line[absIdx:])
			inScope = true
		}
	}
	return lines
}
