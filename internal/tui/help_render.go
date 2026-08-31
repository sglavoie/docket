package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// helpKeyColumnWidth is the fixed cell width of the help key column. A display
// key wider than this wraps mid-token, so every composed spelling must fit. The
// widest one is the review pair, "=review =unreview".
const helpKeyColumnWidth = 17

// helpSectionOrder is the display order of `?` help sections. Its names are the
// categories the projections actually emit: operationCategoryLabel produces the
// registry ones, the keymap-projected providers add View, Actions, and Filters,
// and grammarHelpItems adds the last. A category absent from this list still
// renders — helpSectionsForCategories appends it in first-seen order — so a new
// category is never silently swallowed.
var helpSectionOrder = []string{
	"Tasks",
	"Filters",
	"Sorting",
	"Discovery",
	"Modes",
	"Scope",
	"Navigation",
	"Projects",
	"Notes",
	"Tags",
	"View",
	"Actions",
	"System",
	helpGrammarCategory,
}

// helpSectionOther is the catch-all operationCategoryLabel emits for an
// uncategorized operation. It renders after every named section, so a remainder
// reads as one rather than as a peer.
const helpSectionOther = "Other"

// helpAliasSuffix names the aliases a query matched, so a row whose label does
// not contain the typed word still says why it is here. It is dropped whole
// rather than truncated when the description column has no room for it: the
// description is the row's answer and the aliases are its justification, and a
// clipped justification reads as a clipped answer.
func helpAliasSuffix(aliases []string, description string, descWidth int) string {
	if len(aliases) == 0 {
		return ""
	}
	suffix := " · " + strings.Join(aliases, ", ")
	if lipgloss.Width(description)+lipgloss.Width(suffix) > descWidth {
		return ""
	}
	return suffix
}

// helpSectionForItem names the section an item renders under. An item carrying
// no category is a remainder, not a section of its own.
func helpSectionForItem(item HelpItem) string {
	if strings.TrimSpace(item.Category) == "" {
		return helpSectionOther
	}
	return item.Category
}

// helpSectionsForCategories orders the distinct sections present in categories:
// the canonical helpSectionOrder first, then any unlisted category in first-seen
// order, then Other. The tab bar and the rendered body both read it, so they
// cannot disagree about which sections exist or where they sit.
func helpSectionsForCategories(categories []string) []string {
	known := make(map[string]bool, len(helpSectionOrder))
	for _, section := range helpSectionOrder {
		known[section] = true
	}
	present := make(map[string]bool, len(categories))
	var unlisted []string
	for _, category := range categories {
		if present[category] {
			continue
		}
		present[category] = true
		if !known[category] && category != helpSectionOther {
			unlisted = append(unlisted, category)
		}
	}
	sections := make([]string, 0, len(present))
	for _, section := range helpSectionOrder {
		if present[section] {
			sections = append(sections, section)
		}
	}
	sections = append(sections, unlisted...)
	if present[helpSectionOther] {
		sections = append(sections, helpSectionOther)
	}
	return sections
}

// renderBySectionContent renders first-level (per-view) help grouped by the
// category its own projection assigned. Bucketing appends in item order, so a
// section preserves the binding order the strategy declared and the teach order
// the grammar declared.
func (m *HelpViewModel) renderBySectionContent() string {
	if len(m.filteredItems) == 0 {
		return MutedStyle.Render("No matching help items.")
	}

	buckets := make(map[string][]filteredHelpItem)
	categories := make([]string, 0, len(m.filteredItems))
	for _, item := range m.filteredItems {
		section := helpSectionForItem(item.HelpItem)
		categories = append(categories, section)
		buckets[section] = append(buckets[section], item)
	}

	contentWidth := max(m.viewport.Width(), 1)
	keyWidth := helpKeyColumnWidth
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
		wrappedDesc := wrapDescriptionWithIndent(item.Description, descWidth, keyWidth+2)
		descLines := strings.Split(wrappedDesc, "\n")
		if len(item.descPositions) > 0 {
			descLines[0] = HighlightText(descLines[0], item.descPositions, NormalStyle)
		}
		if suffix := helpAliasSuffix(item.matchedAliases, item.Description, descWidth); suffix != "" {
			descLines[0] += MutedStyle.Render(suffix)
		}
		var result strings.Builder
		for i, line := range descLines {
			if i == 0 {
				result.WriteString(renderedKey)
				result.WriteString("  ")
				result.WriteString(line)
			} else {
				result.WriteString("\n")
				result.WriteString(line)
			}
		}
		return result.String()
	}

	var b strings.Builder
	first := true
	for _, section := range helpSectionsForCategories(categories) {
		items := buckets[section]
		if len(items) == 0 {
			continue
		}
		if !first {
			b.WriteString("\n")
		}
		first = false
		b.WriteString(HelpCategoryStyle.Render(section))
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

	return b.String()
}

func (m *HelpViewModel) renderContent() string {
	return m.renderBySectionContent()
}

// tabBarLines returns the number of visual lines the tab bar occupies.
func (m *HelpViewModel) tabBarLines() int {
	return lipgloss.Height(m.renderHelpTabBar())
}

func (m *HelpViewModel) renderHelpTabBar() string {
	return m.renderFullHelpTabBar()
}

func (m *HelpViewModel) renderFullHelpTabBar() string {
	tabNames := append([]string{"All"}, m.categories...)
	return renderWrappingTabBar(tabNames, m.activeTab, m.innerWidth())
}

func (m *HelpViewModel) renderHelpHeader(tabBar string) string {
	pad := " "
	var b strings.Builder

	b.WriteString(renderFullscreenTitleWithPulse("Help", m.viewName, m.currentWorkspacePulse(), m.width))
	b.WriteString("\n")
	b.WriteString(pad)
	b.WriteString(tabBar)
	b.WriteString("\n")

	if m.tabSearchMode {
		b.WriteString(pad)
		b.WriteString(SubtitleStyle.Render("Tab: "))
		b.WriteString(m.tabSearchInput.View())
		b.WriteString("\n\n")
	}

	caseNote := ""
	if m.searchCaseRelaxed {
		caseNote = MutedStyle.Render(" (case ignored)")
	}

	if m.searchMode {
		b.WriteString(pad)
		b.WriteString(SubtitleStyle.Render("Filter: "))
		b.WriteString(m.searchInput.View())
		b.WriteString(caseNote)
		b.WriteString("\n\n")
	} else if m.searchQuery != "" {
		filterInfo := fmt.Sprintf("Filter: \"%s\"", m.searchQuery)
		b.WriteString(pad)
		b.WriteString(HelpFilterActiveStyle.Render(filterInfo))
		b.WriteString(caseNote)
		b.WriteString("\n\n")
	}

	return b.String()
}

func (m *HelpViewModel) ContextualGuidance() (GuidanceSpec, bool) {
	if m.searchMode {
		return GuidanceSpec{
			Hints: append(requiredGuidanceHints("Enter apply filter", "Esc cancel edit"),
				optionalGuidanceHints(10, "Shift+←/→ tabs", "\\ find tab")...),
			Supplemental: m.helpScrollSupplemental(),
			Rows:         2,
		}, true
	}
	if m.tabSearchMode {
		return GuidanceSpec{
			Hints: append(requiredGuidanceHints("Enter apply tab", "Esc cancel find"),
				optionalGuidanceHints(10, "/ edit filter")...),
			Supplemental: m.helpScrollSupplemental(),
			Rows:         2,
		}, true
	}
	spec, ok := guidanceSpecFromKeyMap(m.buildKeyMap(), 1)
	if !ok {
		return GuidanceSpec{}, false
	}
	spec.Supplemental = m.helpScrollSupplemental()
	if len(m.filteredItems) == 0 {
		spec.Supplemental = joinGuidanceContext("no matching help items", spec.Supplemental)
	}
	return spec, true
}

func (m *HelpViewModel) helpScrollSupplemental() string {
	if !m.ready || m.viewport.TotalLineCount() <= m.viewport.Height() {
		return ""
	}
	return fmt.Sprintf("%.0f%%", m.viewport.ScrollPercent()*100)
}

func (m *HelpViewModel) renderHelpFooter() string {
	spec, ok := m.ContextualGuidance()
	if !ok {
		return ""
	}
	return renderGuidance(spec, m.width)
}

func (m *HelpViewModel) View() tea.View {
	var b strings.Builder

	b.WriteString(m.renderHelpHeader(m.renderHelpTabBar()))

	// Viewport with content
	if m.ready {
		for i, line := range strings.Split(m.viewport.View(), "\n") {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(" ")
			b.WriteString(line)
		}
	}

	// Status bar footer
	b.WriteString("\n")
	b.WriteString(m.renderHelpFooter())

	return tea.NewView(b.String())
}

func (m *HelpViewModel) StatusBarLabel() string {
	return "Help"
}

// HandledKeys returns the keys handled by Help.
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
