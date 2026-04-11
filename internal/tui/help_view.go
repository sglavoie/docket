package tui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/fuzzy"
)

// HelpItem represents a single help entry.
type HelpItem struct {
	Key         string     // e.g., "j/k", "ctrl+O"
	Description string     // e.g., "Move cursor"
	Category    string     // e.g., "Navigation", "Actions"
	Views       []ViewType // nil = global; populated by GetAllDeduplicated for explorer scope display
}

// filteredHelpItem pairs a HelpItem with fuzzy match positions for highlighting.
type filteredHelpItem struct {
	HelpItem
	keyPositions  []int
	descPositions []int
}

// HelpProvider is an interface for views that can provide help items.
// Two patterns are acceptable:
//  1. Registry-backed (most views): return GetKeybindingRegistry().GetForView(ViewXxx)
//  2. Custom (views with dynamic descriptions): build and return []HelpItem directly.
//
// Use custom only when descriptions must change at runtime (e.g., toggling archive state).
type HelpProvider interface {
	GetHelpItems() []HelpItem
}

// HelpViewModel is the model for the help view.
type HelpViewModel struct {
	items          []HelpItem
	filteredItems  []filteredHelpItem
	viewport       viewport.Model
	searchInput    textinput.Model
	searchMode     bool
	searchQuery    string
	width          int
	height         int
	sourceView     ViewType
	viewName       string
	ready          bool
	activeTab      int      // Current tab (category) index
	categories     []string // Ordered list of categories derived from items
	isExplorer     bool     // True when used as global keybinding explorer
	tabSearchMode  bool     // True when tab search input is active
	tabSearchInput textinput.Model
}

// NewHelpViewModel creates a new help view model.
func NewHelpViewModel(items []HelpItem, sourceView ViewType, viewName string) *HelpViewModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Filter help..."
	searchInput.CharLimit = 50
	searchInput.SetWidth(30)

	tabSearchInput := textinput.New()
	tabSearchInput.Placeholder = "Search tabs..."
	tabSearchInput.CharLimit = 30
	tabSearchInput.SetWidth(20)

	m := &HelpViewModel{
		items:          items,
		sourceView:     sourceView,
		viewName:       viewName,
		searchMode:     false,
		searchInput:    searchInput,
		searchQuery:    "",
		ready:          false,
		tabSearchInput: tabSearchInput,
	}
	m.filteredItems = make([]filteredHelpItem, len(items))
	for i, item := range items {
		m.filteredItems[i] = filteredHelpItem{HelpItem: item}
	}
	m.categories = m.buildCategories()
	return m
}

// NewKeybindingExplorerModel creates a global keybinding explorer with auto-focused search.
func NewKeybindingExplorerModel() *HelpViewModel {
	items := GetKeybindingRegistry().GetAllDeduplicated()
	searchInput := textinput.New()
	searchInput.Placeholder = "Search keybindings..."
	searchInput.CharLimit = 50
	searchInput.SetWidth(30)
	searchInput.Focus()

	tabSearchInput := textinput.New()
	tabSearchInput.Placeholder = "Search tabs..."
	tabSearchInput.CharLimit = 30
	tabSearchInput.SetWidth(20)

	m := &HelpViewModel{
		items:          items,
		sourceView:     ViewKeybindingExplorer,
		viewName:       "All Views",
		searchMode:     true,
		searchInput:    searchInput,
		searchQuery:    "",
		ready:          false,
		isExplorer:     true,
		tabSearchInput: tabSearchInput,
	}
	m.filteredItems = make([]filteredHelpItem, len(items))
	for i, item := range items {
		m.filteredItems[i] = filteredHelpItem{HelpItem: item}
	}
	m.categories = m.buildCategories()
	return m
}

// buildCategories extracts ordered unique categories from all items.
func (m *HelpViewModel) buildCategories() []string {
	seen := make(map[string]bool)
	var cats []string
	for _, item := range m.items {
		if !seen[item.Category] {
			seen[item.Category] = true
			cats = append(cats, item.Category)
		}
	}
	sort.Strings(cats)
	return cats
}

// matchTab fuzzy-matches the query against category names and selects the best match.
func (m *HelpViewModel) matchTab(query string) {
	if query == "" {
		m.activeTab = 0 // "All" tab
		m.filterItems()
		m.viewport.SetContent(m.renderContent())
		m.viewport.GotoTop()
		return
	}
	bestIdx := 0
	bestScore := 0
	for i, cat := range m.categories {
		score, _ := fuzzy.ScoreTermsWithPositions(cat, query)
		if score > bestScore {
			bestScore = score
			bestIdx = i + 1 // +1 because tab 0 is "All"
		}
	}
	if bestScore > 0 {
		m.activeTab = bestIdx
	} else {
		m.activeTab = 0
	}
	m.filterItems()
	m.viewport.SetContent(m.renderContent())
	m.viewport.GotoTop()
}

// HasActiveSearch returns true if search mode is active or a search filter is applied.
func (m *HelpViewModel) HasActiveSearch() bool {
	return m.searchMode || m.searchQuery != "" || m.tabSearchMode
}

// HasTextInput returns true when help search or tab search is active.
func (m *HelpViewModel) HasTextInput() bool {
	return m.searchMode || m.tabSearchMode
}

// IsFullHeight returns true because the help view renders its own footer.
func (m *HelpViewModel) IsFullHeight() bool {
	return true
}

// ShouldCaptureEsc returns true when help search or tab search is active.
func (m *HelpViewModel) ShouldCaptureEsc() bool {
	return m.HasActiveSearch()
}

// SetSize sets the view dimensions.
func (m *HelpViewModel) innerWidth() int {
	return max(m.width-2, 0)
}

func (m *HelpViewModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	searchHeight := 0
	if m.searchMode || m.searchQuery != "" {
		searchHeight = 2
	}
	tabSearchHeight := 0
	if m.tabSearchMode {
		tabSearchHeight = 2
	}
	statusBarHeight := 3 // blank line + separator + bar
	tabBarHeight := m.tabBarLines()

	viewportWidth := m.innerWidth()
	viewportHeight := height - tabBarHeight - searchHeight - tabSearchHeight - statusBarHeight + 1

	if viewportWidth < 1 {
		viewportWidth = 1
	}
	if viewportHeight < 5 {
		viewportHeight = 5
	}

	if !m.ready {
		m.viewport = viewport.New(viewport.WithWidth(viewportWidth), viewport.WithHeight(viewportHeight))
		m.viewport.YPosition = 0
		m.ready = true
	} else {
		m.viewport.SetWidth(viewportWidth)
		m.viewport.SetHeight(viewportHeight)
	}

	m.viewport.SetContent(m.renderContent())
}

func (m *HelpViewModel) Init() tea.Cmd {
	if m.isExplorer {
		return textinput.Blink
	}
	return nil
}

func (m *HelpViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyPressMsg:
		if m.searchMode {
			switch msg.String() {
			case "esc":
				// Exit search mode but keep the filter active
				m.searchMode = false
				m.searchInput.Blur()
				return m, nil
			case "enter", "/":
				m.searchMode = false
				m.searchInput.Blur()
				return m, nil
			case `\`:
				m.searchMode = false
				m.searchInput.Blur()
				m.tabSearchMode = true
				m.tabSearchInput.SetValue("")
				m.tabSearchInput.Focus()
				return m, textinput.Blink
			case "[":
				CycleTab(&m.activeTab, len(m.categories)+1, -1)
				m.filterItems()
				m.viewport.SetContent(m.renderContent())
				m.viewport.GotoTop()
				return m, nil
			case "]":
				CycleTab(&m.activeTab, len(m.categories)+1, 1)
				m.filterItems()
				m.viewport.SetContent(m.renderContent())
				m.viewport.GotoTop()
				return m, nil
			default:
				var cmd tea.Cmd
				m.searchInput, cmd = m.searchInput.Update(msg)
				m.searchQuery = m.searchInput.Value()
				m.filterItems()
				m.viewport.SetContent(m.renderContent())
				m.viewport.GotoTop()
				return m, cmd
			}
		}

		if m.tabSearchMode {
			switch msg.String() {
			case "esc", "enter", `\`:
				m.tabSearchMode = false
				m.tabSearchInput.Blur()
				m.tabSearchInput.SetValue("")
				return m, nil
			case "/":
				m.tabSearchMode = false
				m.tabSearchInput.Blur()
				m.tabSearchInput.SetValue("")
				m.searchMode = true
				m.searchInput.Focus()
				return m, textinput.Blink
			default:
				var cmd tea.Cmd
				m.tabSearchInput, cmd = m.tabSearchInput.Update(msg)
				m.matchTab(m.tabSearchInput.Value())
				return m, cmd
			}
		}

		// Explorer-specific: ctrl+\ toggles close
		if m.isExplorer && msg.String() == `ctrl+\` {
			return m, func() tea.Msg { return CloseKeybindingExplorerMsg{} }
		}

		switch msg.String() {
		case "?":
			if m.isExplorer {
				return m, nil // ? does not close the explorer
			}
			return m, func() tea.Msg {
				return CloseHelpMsg{}
			}
		case "/":
			m.searchMode = true
			m.searchInput.Focus()
			return m, textinput.Blink
		case `\`:
			m.tabSearchMode = true
			m.tabSearchInput.SetValue("")
			m.tabSearchInput.Focus()
			return m, textinput.Blink
		case "backspace":
			needsRefresh := false
			if m.searchQuery != "" {
				m.searchQuery = ""
				m.searchInput.SetValue("")
				needsRefresh = true
			}
			if m.activeTab != 0 {
				m.activeTab = 0
				needsRefresh = true
			}
			if needsRefresh {
				m.filterItems()
				m.viewport.SetContent(m.renderContent())
				m.viewport.GotoTop()
			}
			return m, nil
		case "esc":
			if m.searchQuery != "" {
				m.searchQuery = ""
				m.searchInput.SetValue("")
				m.filterItems()
				m.viewport.SetContent(m.renderContent())
				m.viewport.GotoTop()
				return m, nil
			}
			if m.isExplorer {
				return m, func() tea.Msg { return CloseKeybindingExplorerMsg{} }
			}
		case "[", "h":
			CycleTab(&m.activeTab, len(m.categories)+1, -1)
			m.filterItems()
			m.viewport.SetContent(m.renderContent())
			m.viewport.GotoTop()
			return m, nil
		case "]", "l":
			CycleTab(&m.activeTab, len(m.categories)+1, 1)
			m.filterItems()
			m.viewport.SetContent(m.renderContent())
			m.viewport.GotoTop()
			return m, nil
		case "up", "k":
			m.viewport.ScrollUp(1)
		case "down", "j":
			m.viewport.ScrollDown(1)
		case "g":
			m.viewport.GotoTop()
		case "G":
			m.viewport.GotoBottom()
		case "pgup", "ctrl+u":
			m.viewport.HalfPageUp()
		case "pgdown", "ctrl+d":
			m.viewport.HalfPageDown()
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *HelpViewModel) filterItems() {
	// Start with all items or tab-filtered items
	source := m.items
	if m.activeTab > 0 && m.activeTab <= len(m.categories) {
		activeCategory := m.categories[m.activeTab-1]
		source = nil
		for _, item := range m.items {
			if item.Category == activeCategory {
				source = append(source, item)
			}
		}
	}

	if m.searchQuery == "" {
		m.filteredItems = make([]filteredHelpItem, len(source))
		for i, item := range source {
			m.filteredItems[i] = filteredHelpItem{HelpItem: item}
		}
		return
	}

	var filtered []filteredHelpItem

	for _, item := range source {
		keyScore, keyPos := fuzzy.ScoreTermsWithPositions(item.Key, m.searchQuery)
		descScore, descPos := fuzzy.ScoreTermsWithPositions(item.Description, m.searchQuery)
		catScore, _ := fuzzy.ScoreTermsWithPositions(item.Category, m.searchQuery)

		scopeScore := 0
		if m.isExplorer {
			scopeScore, _ = fuzzy.ScoreTermsWithPositions(scopeText(item.Views), m.searchQuery)
		}

		if keyScore > 0 || descScore > 0 || catScore > 0 || scopeScore > 0 {
			filtered = append(filtered, filteredHelpItem{
				HelpItem:      item,
				keyPositions:  keyPos,
				descPositions: descPos,
			})
		}
	}

	m.filteredItems = filtered
}

func (m *HelpViewModel) renderContent() string {
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
	right := HelpStyle.Render(fmt.Sprintf("[/]: tabs · \\ find tab · / filter · bksp clear · j/k scroll · %s%s", closeKey, scrollPercent))

	b.WriteString(renderStatusBarFooter(left, right, m.width))

	return tea.NewView(b.String())
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

// CloseHelpMsg is sent when the help view should close.
type CloseHelpMsg struct{}

// ShowHelpMsg is sent to show help for a specific view.
type ShowHelpMsg struct {
	SourceView ViewType
	ViewName   string
	Items      []HelpItem
}

func (m *HelpViewModel) StatusBarLabel() string {
	if m.isExplorer {
		return "Explorer"
	}
	return "Help"
}

// HandledKeys returns the keys handled by the keybinding explorer view.
func (m *HelpViewModel) HandledKeys() []string {
	return []string{
		"/", `\`, "backspace", "h", "l", "[", "]",
		"j", "k", "up", "down",
		"g", "G",
		"ctrl+u", "ctrl+d",
		"n", "N",
		`ctrl+\`,
		"?", "esc",
	}
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

// scopeText returns a human-readable scope label for a set of views.
func scopeText(views []ViewType) string {
	if len(views) == 0 {
		return "Global"
	}
	if len(views) <= 3 {
		names := make([]string, len(views))
		for i, v := range views {
			names[i] = v.String()
		}
		return strings.Join(names, ", ")
	}
	return fmt.Sprintf("%d views", len(views))
}

// ShowKeybindingExplorerMsg opens the global keybinding explorer.
type ShowKeybindingExplorerMsg struct{}

// CloseKeybindingExplorerMsg closes the global keybinding explorer.
type CloseKeybindingExplorerMsg struct{}
