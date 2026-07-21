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
	var inFrequentSection bool
	var inSuggestedSection bool
	if startIdx > 0 {
		prev := m.filteredItems[startIdx-1]
		inFrequentSection = prev.IsFrequent
		inSuggestedSection = prev.IsSuggested
		if !inFrequentSection && !inSuggestedSection {
			currentCategory = prev.Item.Category
		}
	}

	for i := startIdx; i <= endIdx; i++ {
		fi := m.filteredItems[i]
		if showHeaders {
			if fi.IsFrequent && !inFrequentSection {
				// Start of recent section: one header line, no blank before it
				lines++
				inFrequentSection = true
			} else if fi.IsSuggested && !inSuggestedSection {
				// Start of suggested section: blank separator + header
				lines++ // blank separator
				lines++ // suggested header
				inSuggestedSection = true
				inFrequentSection = false
			} else if !fi.IsFrequent && !fi.IsSuggested {
				if inFrequentSection || inSuggestedSection {
					// Transition from special section to regular categories
					lines++ // blank separator
					lines++ // category header
					currentCategory = fi.Item.Category
					inFrequentSection = false
					inSuggestedSection = false
				} else if fi.Item.Category != currentCategory {
					if currentCategory != "" {
						lines++ // blank separator
					}
					lines++ // category header
					currentCategory = fi.Item.Category
				}
			}
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

	state := m.itemListRenderState(scrollOffset)

	linesRendered := 0
	for i := scrollOffset; i < len(m.filteredItems) && linesRendered < maxVisible; i++ {
		filtered := m.filteredItems[i]

		if showHeaders && m.renderItemListHeaders(&b, filtered, &state, &linesRendered, maxVisible) {
			break
		}

		line := m.renderPaletteItemWithHighlight(filtered, i == m.cursor, contentWidth, -1)
		if appendPaletteLines(&b, &linesRendered, maxVisible, line) {
			break
		}
	}

	if len(m.filteredItems) > maxVisible {
		b.WriteString(m.renderScrollIndicator(scrollOffset, maxVisible, contentWidth))
	}

	return b.String()
}

type paletteItemListState struct {
	currentCategory    ItemCategory
	inFrequentSection  bool
	inSuggestedSection bool
}

func (m *CommandPaletteModel) itemListRenderState(scrollOffset int) paletteItemListState {
	state := paletteItemListState{}
	if scrollOffset == 0 {
		return state
	}

	prev := m.filteredItems[scrollOffset-1]
	state.inFrequentSection = prev.IsFrequent
	state.inSuggestedSection = prev.IsSuggested
	if !state.inFrequentSection && !state.inSuggestedSection {
		state.currentCategory = prev.Item.Category
	}
	return state
}

func (m *CommandPaletteModel) renderItemListHeaders(
	b *strings.Builder,
	filtered filteredPaletteItem,
	state *paletteItemListState,
	linesRendered *int,
	maxVisible int,
) bool {
	switch {
	case filtered.IsFrequent && !state.inFrequentSection:
		state.inFrequentSection = true
		state.inSuggestedSection = false
		return appendPaletteLines(b, linesRendered, maxVisible, m.renderFrequentHeader())
	case filtered.IsSuggested && !state.inSuggestedSection:
		state.inSuggestedSection = true
		state.inFrequentSection = false
		return appendPaletteLines(b, linesRendered, maxVisible, "", m.renderSuggestedHeader())
	case filtered.IsFrequent || filtered.IsSuggested:
		return false
	}

	headers := m.categoryHeaderLines(filtered.Item.Category, state)
	return appendPaletteLines(b, linesRendered, maxVisible, headers...)
}

func (m *CommandPaletteModel) categoryHeaderLines(cat ItemCategory, state *paletteItemListState) []string {
	switch {
	case state.inFrequentSection || state.inSuggestedSection:
		state.inFrequentSection = false
		state.inSuggestedSection = false
		state.currentCategory = cat
		return []string{"", m.renderCategoryHeader(cat)}
	case cat == state.currentCategory:
		return nil
	case state.currentCategory == "":
		state.currentCategory = cat
		return []string{m.renderCategoryHeader(cat)}
	default:
		state.currentCategory = cat
		return []string{"", m.renderCategoryHeader(cat)}
	}
}

func appendPaletteLines(b *strings.Builder, linesRendered *int, maxVisible int, lines ...string) bool {
	for _, line := range lines {
		if *linesRendered >= maxVisible {
			return true
		}
		if line != "" {
			b.WriteString(line)
		}
		b.WriteString("\n")
		*linesRendered++
	}
	return *linesRendered >= maxVisible
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
// Frequent items (duplicates) are excluded from the count.
func (m *CommandPaletteModel) renderCategoryHeader(cat ItemCategory) string {
	name := getCategoryDisplayName(cat)
	count := 0
	for _, filtered := range m.filteredItems {
		if !filtered.IsFrequent && !filtered.IsSuggested && filtered.Item.Category == cat {
			count++
		}
	}

	header := fmt.Sprintf("── %s (%d) ", name, count)
	return MutedStyle.Render(header)
}

// renderFrequentHeader renders the section header for the Recent section.
func (m *CommandPaletteModel) renderFrequentHeader() string {
	count := 0
	for _, fi := range m.filteredItems {
		if fi.IsFrequent {
			count++
		}
	}
	header := fmt.Sprintf("── Recent (%d) ", count)
	return MutedStyle.Render(header)
}

// renderSuggestedHeader renders the contextual-command section header.
func (m *CommandPaletteModel) renderSuggestedHeader() string {
	count := 0
	for _, fi := range m.filteredItems {
		if fi.IsSuggested {
			count++
		}
	}
	header := fmt.Sprintf("── Contextual (%d) ", count)
	return MutedStyle.Render(header)
}
