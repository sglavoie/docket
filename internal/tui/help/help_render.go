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

// helpColumnGap is the blank run between two columns of one group.
const helpColumnGap = 4

// helpShortcutMaxColumns is how many columns an ordinary shortcut group may
// spread across; helpGrammarMaxColumns caps any group holding a grammar form,
// whose prose-sized spellings and descriptions cannot fit whole in a third.
const (
	helpShortcutMaxColumns = 3
	helpGrammarMaxColumns  = 2
)

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
	descWidth := helpMinDescriptionWidth
	const exactSection = "Exact matches"
	for _, item := range m.filteredItems {
		section := helpSectionForItem(item.HelpItem)
		// Shortcuts share one grid across categories and task groups: one key
		// column and one description measure, the widest whole entry, so every
		// group takes the same column count and its columns line up with the
		// next group's. Grammar forms are prose-sized and keep their own.
		if section != helpGrammarCategory {
			keyWidth = max(keyWidth, lipgloss.Width(item.Key))
			descWidth = max(descWidth, helpItemDescriptionWidth(item))
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
		// The heading's own top margin is the one blank row between groups.
		if first || compactHeadings {
			heading = heading.MarginTop(0)
		}
		first = false
		b.WriteString(heading.Render(title))
		b.WriteString("\n")
	}
	// The row legend leads a screen with room for it. A short one keeps the
	// daily groups on its first screen instead, so there it closes the
	// reference rather than opening it.
	marks := m.visibleRowMarks()
	writeMarks := func() {
		if len(marks) == 0 {
			return
		}
		markWidth := 1
		for _, mark := range marks {
			markWidth = max(markWidth, lipgloss.Width(mark.Key))
		}
		if line, ok := helpRowMarksLine(marks, max(m.viewport.Width(), 1)); ok {
			// A legend that fits beside its heading costs the reference one row.
			first = false
			b.WriteString(line)
			b.WriteString("\n")
			return
		}
		writeHeading(helpRowMarksHeading)
		b.WriteString(renderHelpItemGroup(marks, max(m.viewport.Width(), 1), markWidth, descWidth))
	}
	if !compactHeadings {
		writeMarks()
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
			b.WriteString(renderHelpItemGroup(items[start:end], max(m.viewport.Width(), 1), groupKeyWidth, descWidth))
			start = end
		}
	}
	if compactHeadings {
		writeMarks()
	}

	return b.String()
}

// helpItemDescriptionWidth is the width an item's description occupies whole,
// including the aliases a search matched, which render beside it.
func helpItemDescriptionWidth(item filteredHelpItem) int {
	description := item.Description
	if len(item.matchedAliases) > 0 {
		description += " · " + strings.Join(item.matchedAliases, ", ")
	}
	return lipgloss.Width(description)
}

// helpColumnLayout chooses a group's column count and description width, given
// what one column's key and its two-column separator cost.
//
// A shortcut group takes three columns only when each can hold the widest whole
// description, so the threshold is measured from the entries rather than fixed:
// contentWidth >= 3*(overhead+preferred) + 2*helpColumnGap. Below it, two
// columns keep their rule — any room of at least helpMinDescriptionWidth,
// wrapping a longer description — and below that one column takes the width.
// A group holding a grammar form never exceeds helpGrammarMaxColumns and keeps
// its prose at the full available width.
func helpColumnLayout(contentWidth, overhead, preferred int, grammar bool) (columns, descWidth int) {
	available := func(columns int) int {
		return (contentWidth - columns*overhead - (columns-1)*helpColumnGap) / columns
	}
	maxColumns := helpShortcutMaxColumns
	if grammar {
		maxColumns = helpGrammarMaxColumns
	}
	for columns := maxColumns; columns > 2; columns-- {
		if available(columns) >= preferred {
			return columns, preferred
		}
	}
	if width := available(2); width >= helpMinDescriptionWidth {
		if grammar {
			return 2, width
		}
		// Short shortcut groups need no more space than their descriptions.
		return 2, min(width, preferred)
	}
	return 1, max(contentWidth-overhead, 1)
}

// renderHelpItemGroup lays one group out on the shared shortcut grid, keyWidth
// and shortcutDescWidth, expanding the key column for grammar forms when needed.
func renderHelpItemGroup(items []filteredHelpItem, contentWidth, keyWidth, shortcutDescWidth int) string {
	grammar := false
	preferredWidth := max(helpMinDescriptionWidth, shortcutDescWidth)
	for _, item := range items {
		keyWidth = max(keyWidth, lipgloss.Width(item.Key))
		// Exact matches can combine grammar forms with ordinary shortcuts; one
		// grammar form puts the whole group on grammar's layout.
		if item.Category == helpGrammarCategory {
			grammar = true
			continue
		}
		preferredWidth = max(preferredWidth, helpItemDescriptionWidth(item))
	}
	columnOverhead := keyWidth + 2
	columns, descWidth := helpColumnLayout(contentWidth, columnOverhead, preferredWidth, grammar)
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
	gap := strings.Repeat(" ", helpColumnGap)
	for start := 0; start < len(items); start += columns {
		end := min(start+columns, len(items))
		cells := make([]string, 0, 2*(end-start)-1)
		for j := start; j < end; j++ {
			if j > start {
				cells = append(cells, gap)
			}
			cells = append(cells, columnStyle.Render(renderItem(items[j])))
		}
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, cells...))
		b.WriteString("\n")
	}
	return b.String()
}

// helpRowMarksHeading heads the legend of the opener's cursor row.
const helpRowMarksHeading = "On this row"

// helpRowMarksLine is the whole legend on one row beside its heading, or
// ok=false when it would not fit the width whole.
func helpRowMarksLine(marks []filteredHelpItem, width int) (string, bool) {
	parts := make([]string, 0, len(marks))
	for _, mark := range marks {
		parts = append(parts, theme.HelpKeyStyle.Render(mark.Key)+" "+mark.Description)
	}
	line := " " + theme.HelpCategoryStyle.MarginTop(0).Render(helpRowMarksHeading) + "   " + strings.Join(parts, theme.MutedStyle.Render("   ·   "))
	if lipgloss.Width(line) > width {
		return "", false
	}
	return line, true
}

// SetRowMarks installs the legend of the row help was opened from.
func (m *HelpViewModel) SetRowMarks(marks []operation.HelpItem) {
	m.rowMarks = marks
	m.filterItems()
}

// visibleRowMarks is the row legend where it leads: the unsearched All view,
// since a search or a category asks about keys, not about the row.
func (m *HelpViewModel) visibleRowMarks() []filteredHelpItem {
	if len(m.rowMarks) == 0 || m.searchQuery != "" || m.activeTab != 0 {
		return nil
	}
	marks := make([]filteredHelpItem, len(m.rowMarks))
	for i, item := range m.rowMarks {
		marks[i] = filteredHelpItem{HelpItem: item}
	}
	return marks
}

func (m *HelpViewModel) renderContent() string {
	if m.actionSearch() {
		content, _ := m.actionSearchDocument()
		return content
	}
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
	if m.actionSearch() {
		b.WriteString(m.renderActionContext())
	}

	return b.String()
}

func (m *HelpViewModel) ContextualGuidance() (chrome.GuidanceSpec, bool) {
	if m.actionSearch() && !m.tabSearchMode {
		return m.actionGuidance(), true
	}
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
