package help

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// Keep enough prose beside each key to read a short action without wrapping.
const helpMinDescriptionWidth = 24

// helpSectionOrder is the display order of `?` help sections. Its names are the
// categories the projections actually emit: OperationCategoryLabel produces the
// registry ones, the keymap-projected providers add View, Actions, and Filters,
// and GrammarHelpItems adds the last. A category absent from this list still
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

// helpSectionOther is the catch-all OperationCategoryLabel emits for an
// uncategorized operation. It renders after every named section, so a remainder
// reads as one rather than as a peer.
const helpSectionOther = "Other"

// helpSectionForItem names the section an item renders under. An item carrying
// no category is a remainder, not a section of its own.
func helpSectionForItem(item operation.HelpItem) string {
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
		message := "No matching help items.\nClear or change the search."
		if m.activeTab > 0 && m.activeTab <= len(m.categories) {
			message = fmt.Sprintf("No matches in %s.\nClear or change the search.", m.categories[m.activeTab-1])
			if m.otherCategoryMatches > 0 {
				noun := "matches"
				if m.otherCategoryMatches == 1 {
					noun = "match"
				}
				action := "Press Tab to show all matches."
				if m.tabSearchMode {
					action = "Finish finding a tab with Enter, then press Tab to show all matches."
				}
				message = fmt.Sprintf("No matches in %s; %d %s in other categories.\n%s",
					m.categories[m.activeTab-1], m.otherCategoryMatches, noun, action)
			}
		}
		return theme.MutedStyle.Width(max(m.viewport.Width(), 1)).Render(message)
	}

	buckets := make(map[string][]filteredHelpItem)
	categories := make([]string, 0, len(m.filteredItems))
	keyWidth := 1
	const exactSection = "Exact matches"
	for _, item := range m.filteredItems {
		section := helpSectionForItem(item.HelpItem)
		// Shortcuts share one alignment across categories and task groups.
		// Grammar forms are prose-sized and keep their own key column.
		if section != helpGrammarCategory {
			keyWidth = max(keyWidth, lipgloss.Width(item.Key))
		}
		if m.searchQuery != "" && item.exactMatch {
			buckets[exactSection] = append(buckets[exactSection], item)
			continue
		}
		categories = append(categories, section)
		buckets[section] = append(buckets[section], item)
	}

	var b strings.Builder
	first := true
	// At the terminal floor the daily groups through Undo must remain on the
	// first screen. Keep their styling but spend no rows on decorative spacing.
	compactHeadings := m.viewport.Height() < 20
	writeHeading := func(title string) {
		heading := theme.HelpCategoryStyle
		if first || compactHeadings {
			heading = heading.MarginTop(0)
		} else {
			b.WriteString("\n")
		}
		first = false
		b.WriteString(heading.Render(title))
		b.WriteString("\n")
	}
	sections := helpSectionsForCategories(categories)
	if len(buckets[exactSection]) > 0 {
		sections = append([]string{exactSection}, sections...)
	}
	for _, section := range sections {
		items := buckets[section]
		if len(items) == 0 {
			continue
		}
		if section != "Tasks" {
			writeHeading(section)
		}
		// Task groups are declared alongside the key bindings. Keep exact search
		// hits together and retain the normal order for every other category.
		if section == "Tasks" {
			items = append([]filteredHelpItem(nil), items...)
			sort.SliceStable(items, func(i, j int) bool { return taskHelpOrder(items[i]) < taskHelpOrder(items[j]) })
			if compactHeadings {
				writeHeading(section)
			}
		}
		groupKeyWidth := keyWidth
		if section == helpGrammarCategory {
			groupKeyWidth = 1
		}
		for start := 0; start < len(items); {
			end := len(items)
			if section == "Tasks" {
				group := taskHelpGroup(items[start])
				if !compactHeadings {
					writeHeading(group)
				}
				end = start + 1
				for end < len(items) && taskHelpGroup(items[end]) == group {
					end++
				}
			}
			b.WriteString(renderHelpItemGroup(items[start:end], max(m.viewport.Width(), 1), groupKeyWidth))
			start = end
		}
	}

	return b.String()
}

// Use the shared shortcut width, expanding it for grammar forms when needed.
func renderHelpItemGroup(items []filteredHelpItem, contentWidth, keyWidth int) string {
	for _, item := range items {
		keyWidth = max(keyWidth, lipgloss.Width(item.Key))
	}
	columnGap := 4
	columnOverhead := keyWidth + 2
	columns := 2
	descWidth := (contentWidth - 2*columnOverhead - columnGap) / 2
	if descWidth < helpMinDescriptionWidth {
		columns = 1
		descWidth = max(contentWidth-columnOverhead, 1)
	} else {
		// Short shortcut groups need no more space than their descriptions.
		// Keep grammar's prose at the available width, including exact matches
		// that combine grammar forms with ordinary shortcuts.
		preferredWidth := helpMinDescriptionWidth
		for _, item := range items {
			if item.Category == helpGrammarCategory {
				preferredWidth = descWidth
				break
			}
			description := item.Description
			if len(item.matchedAliases) > 0 {
				description += " · " + strings.Join(item.matchedAliases, ", ")
			}
			preferredWidth = max(preferredWidth, lipgloss.Width(description))
		}
		descWidth = min(descWidth, preferredWidth)
	}
	columnWidth := keyWidth + 2 + descWidth

	keyStyle := lipgloss.NewStyle().
		Width(keyWidth).
		Align(lipgloss.Right).
		Foreground(theme.ColorSecondary)
	columnStyle := lipgloss.NewStyle().Width(columnWidth)

	renderItem := func(item filteredHelpItem) string {
		var renderedKey string
		if len(item.keyPositions) > 0 {
			renderedKey = keyStyle.Render(textfit.HighlightText(item.Key, item.keyPositions, theme.TagInlineStyle))
		} else {
			renderedKey = keyStyle.Render(item.Key)
		}
		description := item.Description
		if len(item.descPositions) > 0 {
			description = textfit.HighlightText(description, item.descPositions, theme.NormalStyle)
		}
		if len(item.matchedAliases) > 0 {
			description += theme.MutedStyle.Render(" · " + strings.Join(item.matchedAliases, ", "))
		}
		descLines := textfit.WrapStyled(description, descWidth)
		var result strings.Builder
		for i, line := range descLines {
			if i == 0 {
				result.WriteString(renderedKey)
				result.WriteString("  ")
				result.WriteString(line)
			} else {
				result.WriteString("\n")
				result.WriteString(strings.Repeat(" ", columnOverhead))
				result.WriteString(line)
			}
		}
		return result.String()
	}

	var b strings.Builder
	for j := 0; j < len(items); {
		col1 := columnStyle.Render(renderItem(items[j]))
		j++
		if columns == 2 && j < len(items) {
			col2 := columnStyle.Render(renderItem(items[j]))
			col1 = lipgloss.JoinHorizontal(lipgloss.Top, col1, strings.Repeat(" ", columnGap), col2)
			j++
		}
		b.WriteString(col1)
		b.WriteString("\n")
	}
	return b.String()
}

func (m *HelpViewModel) renderContent() string {
	return m.renderBySectionContent()
}

func (m *HelpViewModel) renderHelpTabBar() string {
	return m.renderFullHelpTabBar()
}

func (m *HelpViewModel) renderFullHelpTabBar() string {
	tabNames := append([]string{"All"}, m.categories...)
	if m.searchQuery != "" && len(m.tabMatchCounts) == len(tabNames) {
		for i, count := range m.tabMatchCounts {
			tabNames[i] = fmt.Sprintf("%s (%d)", tabNames[i], count)
		}
	}
	// Compact spacing is what keeps every Outliner section's tab on one row at
	// 80 columns; the default padding wraps the last tab onto a row of its own.
	return chrome.RenderWrappingTabBar(tabNames, m.activeTab, m.innerWidth(), chrome.TabSpacingCompact)
}

func (m *HelpViewModel) renderHelpHeader(tabBar string) string {
	pad := " "
	var b strings.Builder

	b.WriteString(chrome.RenderFullscreenTitleWithPulse("Help", m.viewName, m.CurrentWorkspacePulse(), m.width))
	b.WriteString("\n")
	// A wrapped tab bar is several rows; each takes the same pad as the first.
	for _, row := range strings.Split(tabBar, "\n") {
		b.WriteString(pad)
		b.WriteString(row)
		b.WriteString("\n")
	}

	if m.tabSearchMode {
		b.WriteString(pad)
		b.WriteString(theme.SubtitleStyle.Render("Tab: "))
		b.WriteString(m.tabSearchInput.View())
		b.WriteString("\n\n")
	}

	caseNote := ""
	if m.searchCaseRelaxed {
		caseNote = theme.MutedStyle.Render(" (case ignored)")
	}

	if m.searchMode {
		b.WriteString(pad)
		b.WriteString(theme.SubtitleStyle.Render("Filter: "))
		b.WriteString(m.searchInput.View())
		b.WriteString(caseNote)
		b.WriteString("\n\n")
	} else if m.searchQuery != "" {
		filterInfo := fmt.Sprintf("Filter: \"%s\"", m.searchQuery)
		b.WriteString(pad)
		b.WriteString(theme.HelpFilterActiveStyle.Render(filterInfo))
		b.WriteString(caseNote)
		b.WriteString("\n\n")
	}

	return b.String()
}

func (m *HelpViewModel) ContextualGuidance() (chrome.GuidanceSpec, bool) {
	if m.searchMode {
		hints := chrome.RequiredGuidanceHints("Enter apply filter", "Esc cancel edit")
		if m.otherCategoryMatches > 0 {
			hints = append(hints, chrome.RequiredGuidanceHints("Tab show all matches")...)
		}
		return chrome.GuidanceSpec{
			Hints: append(hints,
				chrome.OptionalGuidanceHints(10, "Shift+←/→ tabs", "\\ find tab")...),
			Supplemental: m.helpScrollSupplemental(),
			Rows:         2,
		}, true
	}
	if m.tabSearchMode {
		return chrome.GuidanceSpec{
			Hints: append(chrome.RequiredGuidanceHints("Enter apply tab", "Esc cancel find"),
				chrome.OptionalGuidanceHints(10, "/ edit filter")...),
			Supplemental: m.helpScrollSupplemental(),
			Rows:         2,
		}, true
	}
	spec, ok := chrome.GuidanceSpecFromKeyMap(m.buildKeyMap(), 1)
	if !ok {
		return chrome.GuidanceSpec{}, false
	}
	spec.Supplemental = m.helpScrollSupplemental()
	if len(m.filteredItems) == 0 {
		spec.Supplemental = chrome.JoinGuidanceContext("no matching help items", spec.Supplemental)
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
	return chrome.RenderGuidance(spec, m.width)
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

func taskHelpOrder(item filteredHelpItem) int {
	if item.Order > 0 {
		return item.Order
	}
	return 400
}

func taskHelpGroup(item filteredHelpItem) string {
	if item.Group == "" {
		return "Details"
	}
	return item.Group
}
