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
	Key         string // e.g., "j/k", "ctrl+O"
	Description string // e.g., "Move cursor"
	Category    string // e.g., "Navigation", "Actions"
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
	items            []HelpItem
	filteredItems    []filteredHelpItem
	viewport         viewport.Model
	searchInput      textinput.Model
	searchMode       bool
	searchQuery      string
	width            int
	height           int
	sourceView       ViewType
	viewName         string
	ready            bool
	sectionPositions []int    // Line numbers where each section header starts
	currentSection   int      // Current section index for n/N navigation
	activeTab        int      // Current tab (category) index
	categories       []string // Ordered list of categories derived from items
}

// NewHelpViewModel creates a new help view model.
func NewHelpViewModel(items []HelpItem, sourceView ViewType, viewName string) *HelpViewModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Filter help..."
	searchInput.CharLimit = 50
	searchInput.SetWidth(30)

	m := &HelpViewModel{
		items:       items,
		sourceView:  sourceView,
		viewName:    viewName,
		searchMode:  false,
		searchInput: searchInput,
		searchQuery: "",
		ready:       false,
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

// HasActiveSearch returns true if search mode is active or a search filter is applied.
func (m *HelpViewModel) HasActiveSearch() bool {
	return m.searchMode || m.searchQuery != ""
}

// HasTextInput returns true when help search is active.
func (m *HelpViewModel) HasTextInput() bool {
	return m.HasActiveSearch()
}

// IsFullHeight returns true because the help view renders its own footer.
func (m *HelpViewModel) IsFullHeight() bool {
	return true
}

// ShouldCaptureEsc returns true when help search is active.
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
	statusBarHeight := 3 // blank line + separator + bar
	tabBarHeight := m.tabBarLines()

	viewportWidth := m.innerWidth()
	viewportHeight := height - tabBarHeight - searchHeight - statusBarHeight + 1

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
			case "enter":
				m.searchMode = false
				m.searchInput.Blur()
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

		switch msg.String() {
		case "?":
			return m, func() tea.Msg {
				return CloseHelpMsg{}
			}
		case "/":
			m.searchMode = true
			m.searchInput.Focus()
			return m, textinput.Blink
		case "esc":
			if m.searchQuery != "" {
				m.searchQuery = ""
				m.searchInput.SetValue("")
				m.filterItems()
				m.viewport.SetContent(m.renderContent())
				m.viewport.GotoTop()
				return m, nil
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
		case "n":
			// Next section (with wrap)
			if len(m.sectionPositions) > 0 {
				startOffset := m.viewport.YOffset()
				for i := 0; i < len(m.sectionPositions); i++ {
					m.currentSection++
					if m.currentSection >= len(m.sectionPositions) {
						m.currentSection = 0
					}
					m.goToSection(m.currentSection)
					if m.viewport.YOffset() != startOffset {
						break
					}
				}
			}
		case "N":
			// Previous section (with wrap)
			if len(m.sectionPositions) > 0 {
				startOffset := m.viewport.YOffset()
				for i := 0; i < len(m.sectionPositions); i++ {
					m.currentSection--
					if m.currentSection < 0 {
						m.currentSection = len(m.sectionPositions) - 1
					}
					m.goToSection(m.currentSection)
					if m.viewport.YOffset() != startOffset {
						break
					}
				}
			}
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

		if keyScore > 0 || descScore > 0 || catScore > 0 {
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
		m.sectionPositions = nil
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

	// Track section positions for n/N navigation
	m.sectionPositions = make([]int, 0, len(categoryOrder))
	lineCount := 0

	// Calculate column widths based on viewport width (not full width)
	// Each column: keyWidth + separator(2) + descWidth
	// Total: 2 * (keyWidth + 2 + descWidth) + columnGap(4)
	contentWidth := max(m.viewport.Width(), 40)
	keyWidth := 12
	columnGap := 4
	columnOverhead := keyWidth + 2 // key + separator
	descWidth := max((contentWidth-2*columnOverhead-columnGap)/2, 20)
	columnWidth := keyWidth + 2 + descWidth

	keyStyle := lipgloss.NewStyle().
		Width(keyWidth).
		Align(lipgloss.Right).
		Foreground(ColorSecondary)

	// columnStyle ensures entire column block has fixed width for proper alignment
	columnStyle := lipgloss.NewStyle().Width(columnWidth)

	// renderHelpItem renders a single key+description pair with proper wrapping
	renderHelpItem := func(key, desc string, keyPos, descPos []int) string {
		// Render key with highlight if positions present
		var renderedKey string
		if len(keyPos) > 0 {
			renderedKey = keyStyle.Render(HighlightText(key, keyPos, TagInlineStyle))
		} else {
			renderedKey = keyStyle.Render(key)
		}

		// Wrap description with indentation for continuation lines
		// Indent = keyWidth + separator(2) so wrapped text aligns under description
		wrappedDesc := wrapDescriptionWithIndent(desc, descWidth, keyWidth+2)

		// Split into lines and build the block
		descLines := strings.Split(wrappedDesc, "\n")
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
			lineCount++
		}

		// Record section position before writing header
		// HelpCategoryStyle has MarginTop(1) which adds a blank line before the header
		// We want to position at the blank line so the header appears consistently at top
		m.sectionPositions = append(m.sectionPositions, lineCount)

		// Category header - count actual lines in rendered output
		header := HelpCategoryStyle.Render(category)
		b.WriteString(header)
		b.WriteString("\n")
		lineCount += strings.Count(header, "\n") + 1

		items := categories[category]

		// Render items in 2-column layout
		for j := 0; j < len(items); j += 2 {
			// First column - wrap in columnStyle to ensure fixed width for proper alignment
			item1 := items[j]
			col1 := columnStyle.Render(renderHelpItem(item1.Key, item1.Description, item1.keyPositions, item1.descPositions))

			// Second column (if exists)
			var col2 string
			if j+1 < len(items) {
				item2 := items[j+1]
				col2 = columnStyle.Render(renderHelpItem(item2.Key, item2.Description, item2.keyPositions, item2.descPositions))
			}

			row := lipgloss.JoinHorizontal(lipgloss.Top, col1, strings.Repeat(" ", columnGap), col2)
			b.WriteString(row)
			b.WriteString("\n")
			// Count actual lines in row (may be multi-line due to wrapped descriptions)
			lineCount += strings.Count(row, "\n") + 1
		}
	}

	return b.String()
}

// goToSection scrolls the viewport to show the given section index at the top.
func (m *HelpViewModel) goToSection(idx int) {
	if idx < 0 || idx >= len(m.sectionPositions) {
		return
	}
	if idx == 0 {
		m.viewport.GotoTop()
	} else {
		m.viewport.SetYOffset(m.sectionPositions[idx])
	}
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
	left := StatusBarStyle.Render(title)

	scrollPercent := ""
	if m.ready && m.viewport.TotalLineCount() > m.viewport.Height() {
		scrollPercent = fmt.Sprintf(" %3.0f%%", m.viewport.ScrollPercent()*100)
	}
	right := HelpStyle.Render(fmt.Sprintf("[/]: tabs · / filter · j/k scroll · n/N sections · ? close%s", scrollPercent))

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

func (m *HelpViewModel) StatusBarLabel() string { return "Help" }
