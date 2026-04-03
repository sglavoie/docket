package tui

import (
	"fmt"
	"strings"
)

// countLinesForRange counts how many display lines are needed to render items
// from startIdx to endIdx (inclusive), including category headers and spacing.
// This is used to calculate scroll offsets that account for header overhead.
func (m *CommandPaletteModel) countLinesForRange(startIdx, endIdx int, showHeaders bool) int {
	if startIdx < 0 || endIdx < startIdx || len(m.filteredItems) == 0 {
		return 0
	}
	if endIdx >= len(m.filteredItems) {
		endIdx = len(m.filteredItems) - 1
	}

	lines := 0
	var currentCategory ItemCategory
	if startIdx > 0 {
		currentCategory = m.filteredItems[startIdx-1].Item.Category
	}

	for i := startIdx; i <= endIdx; i++ {
		item := m.filteredItems[i].Item
		if showHeaders && item.Category != currentCategory {
			if currentCategory != "" {
				lines++
			}
			lines++
			currentCategory = item.Category
		}
		lines++
	}

	return lines
}

// calculateScrollOffset determines the scroll offset needed to keep the cursor
// visible within maxVisible lines, accounting for category headers.
// It preserves the current scroll position when possible, only adjusting when
// the cursor would be outside the visible range.
func (m *CommandPaletteModel) calculateScrollOffset(maxVisible int, showHeaders bool) int {
	if m.cursor < 0 || len(m.filteredItems) == 0 {
		m.scrollOffset = 0
		return 0
	}

	if m.scrollAnchor >= 0 {
		m.scrollOffset = m.scrollAnchor
		m.scrollAnchor = -1
		return m.scrollOffset
	}

	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	if m.scrollOffset >= len(m.filteredItems) {
		m.scrollOffset = len(m.filteredItems) - 1
	}

	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
		return m.scrollOffset
	}

	linesNeeded := m.countLinesForRange(m.scrollOffset, m.cursor, showHeaders)
	if linesNeeded <= maxVisible {
		return m.scrollOffset
	}

	for m.scrollOffset < len(m.filteredItems) {
		linesNeeded = m.countLinesForRange(m.scrollOffset, m.cursor, showHeaders)
		if linesNeeded <= maxVisible {
			break
		}
		m.scrollOffset++
	}

	return m.scrollOffset
}

// renderItemList renders the filtered item list with cursor indicator.
// Each row displays: cursor + category badge + name (prominent) + extra info + shortcut (right-aligned)
// When no search query is active, items are grouped by category with section headers.
func (m *CommandPaletteModel) renderItemList() string {
	if len(m.filteredItems) == 0 {
		return MutedStyle.Render("Type to search commands, tasks, projects...")
	}

	query := strings.TrimSpace(m.searchInput.Value())
	categoryFilter, searchQuery := m.parseQueryPrefix(query)
	showHeaders := searchQuery == "" && categoryFilter == ""

	var b strings.Builder
	hint := m.getContextualHint()
	if hint != "" {
		b.WriteString(HelpStyle.Render(hint))
		b.WriteString("\n\n")
	}

	paneHeight := max(m.height-OverlayListChrome, 5)
	maxVisible := paneHeight
	if hint != "" {
		maxVisible -= 2
	}
	if maxVisible < 5 {
		maxVisible = 5
	}

	contentWidth := max(m.listWidth-4, 1)
	scrollOffset := m.calculateScrollOffset(maxVisible, showHeaders)

	var currentCategory ItemCategory
	if scrollOffset > 0 {
		currentCategory = m.filteredItems[scrollOffset-1].Item.Category
	}

	linesRendered := 0
	for i := scrollOffset; i < len(m.filteredItems) && linesRendered < maxVisible; i++ {
		filtered := m.filteredItems[i]
		item := filtered.Item

		if showHeaders && item.Category != currentCategory {
			if currentCategory != "" {
				b.WriteString("\n")
				linesRendered++
				if linesRendered >= maxVisible {
					break
				}
			}
			header := m.renderCategoryHeader(item.Category)
			b.WriteString(header)
			b.WriteString("\n")
			linesRendered++
			currentCategory = item.Category
			if linesRendered >= maxVisible {
				break
			}
		}

		numberHint := -1
		if m.isNumberJumpEnabled() && i < 9 {
			numberHint = i + 1
		}

		line := m.renderPaletteItemWithHighlight(filtered, i == m.cursor, contentWidth, numberHint)
		b.WriteString(line)
		b.WriteString("\n")
		linesRendered++
	}

	if len(m.filteredItems) > maxVisible {
		b.WriteString(m.renderScrollIndicator(scrollOffset, maxVisible, contentWidth))
	}

	return b.String()
}

// renderScrollIndicator renders a visual progress bar showing scroll position.
func (m *CommandPaletteModel) renderScrollIndicator(scrollOffset, maxVisible, width int) string {
	total := len(m.filteredItems)
	if total <= maxVisible {
		return ""
	}

	maxScroll := total - maxVisible
	if maxScroll <= 0 {
		maxScroll = 1
	}

	progress := float64(scrollOffset) / float64(maxScroll)
	if progress > 1 {
		progress = 1
	}

	barWidth := max(width-20, 10)
	filled := min(max(int(float64(barWidth)*progress), 0), barWidth)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	showing := scrollOffset + 1
	showingEnd := min(scrollOffset+maxVisible, total)
	posInfo := fmt.Sprintf(" %d-%d/%d", showing, showingEnd, total)

	return MutedStyle.Render(bar + posInfo)
}

// renderCategoryHeader renders a section header for a category.
func (m *CommandPaletteModel) renderCategoryHeader(cat ItemCategory) string {
	name := getCategoryDisplayName(cat)
	count := 0
	for _, filtered := range m.filteredItems {
		if filtered.Item.Category == cat {
			count++
		}
	}

	header := fmt.Sprintf("── %s (%d) ", name, count)
	return MutedStyle.Render(header)
}
