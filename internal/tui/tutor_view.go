package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// TutorTopic represents a single topic in the tutor view.
type TutorTopic struct {
	Title    string
	Category string
	Content  string
}

// TutorModel implements the tutor view with a two-pane topic browser.
type TutorModel struct {
	topics         []TutorTopic
	filteredTopics []TutorTopic
	selectedIndex  int
	activePane     int // 0 = list, 1 = content
	viewport       viewport.Model
	searchInput    textinput.Model
	searchMode     bool
	searchQuery    string
	activeTab      int
	categories     []string
	listOffset     int
	width          int
	height         int
	ready          bool
}

// NewTutorModel creates a new tutor view.
func NewTutorModel() *TutorModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Search topics..."
	searchInput.CharLimit = 50
	searchInput.SetWidth(25)
	applyThemedTextInput(&searchInput)

	m := &TutorModel{
		topics:      tutorTopics(),
		searchInput: searchInput,
	}
	m.filteredTopics = m.topics
	m.categories = m.buildCategories()
	return m
}

func (m *TutorModel) buildCategories() []string {
	return []string{"Concepts", "CLI", "TUI"}
}

// HasTextInput returns true when search is active.
func (m *TutorModel) HasTextInput() bool {
	return m.searchMode
}

// IsFullHeight returns true because the tutor view renders its own footer.
func (m *TutorModel) IsFullHeight() bool {
	return true
}

// ShouldCaptureEsc returns true when the view needs to handle esc internally.
func (m *TutorModel) ShouldCaptureEsc() bool {
	return m.searchMode || m.searchQuery != ""
}

func (m *TutorModel) Init() tea.Cmd {
	return nil
}

func (m *TutorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyPressMsg:
		if m.searchMode {
			switch msg.String() {
			case "esc":
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
				m.filterTopics()
				m.selectedIndex = 0
				m.listOffset = 0
				m.updateViewport()
				return m, cmd
			}
		}

		switch msg.String() {
		case "esc":
			if m.searchQuery != "" {
				m.searchQuery = ""
				m.searchInput.SetValue("")
				m.filterTopics()
				m.selectedIndex = 0
				m.listOffset = 0
				m.updateViewport()
				return m, nil
			}
			return m, func() tea.Msg { return BackMsg{} }
		case "/":
			m.searchMode = true
			m.searchInput.Focus()
			return m, textinput.Blink
		case "tab":
			m.activePane = 1 - m.activePane
			return m, nil
		case "h":
			if m.activePane == 0 {
				CycleTab(&m.activeTab, len(m.categories)+1, -1)
				m.filterTopics()
				m.selectedIndex = 0
				m.listOffset = 0
				m.updateViewport()
			}
			return m, nil
		case "l":
			if m.activePane == 0 {
				CycleTab(&m.activeTab, len(m.categories)+1, 1)
				m.filterTopics()
				m.selectedIndex = 0
				m.listOffset = 0
				m.updateViewport()
			}
			return m, nil
		case "[":
			CycleTab(&m.activeTab, len(m.categories)+1, -1)
			m.filterTopics()
			m.selectedIndex = 0
			m.listOffset = 0
			m.updateViewport()
			return m, nil
		case "]":
			CycleTab(&m.activeTab, len(m.categories)+1, 1)
			m.filterTopics()
			m.selectedIndex = 0
			m.listOffset = 0
			m.updateViewport()
			return m, nil
		case "up", "k":
			if m.activePane == 0 {
				if m.selectedIndex > 0 {
					m.selectedIndex--
					m.updateViewport()
				}
			} else {
				m.viewport.ScrollUp(1)
			}
			return m, nil
		case "down", "j":
			if m.activePane == 0 {
				if m.selectedIndex < len(m.filteredTopics)-1 {
					m.selectedIndex++
					m.updateViewport()
				}
			} else {
				m.viewport.ScrollDown(1)
			}
			return m, nil
		case "g":
			if m.activePane == 1 {
				m.viewport.GotoTop()
			}
			return m, nil
		case "G":
			if m.activePane == 1 {
				m.viewport.GotoBottom()
			}
			return m, nil
		case "pgup", "ctrl+u":
			m.viewport.HalfPageUp()
			return m, nil
		case "pgdown", "ctrl+d":
			m.viewport.HalfPageDown()
			return m, nil
		case "enter":
			if m.activePane == 0 {
				m.activePane = 1
				m.viewport.GotoTop()
			}
			return m, nil
		}
	}

	if m.activePane == 1 {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *TutorModel) HandledKeys() []string {
	return []string{
		"/", "tab", "h", "l", "[", "]",
		"up", "k", "down", "j",
		"g", "G",
		"ctrl+u", "ctrl+d",
		"enter",
	}
}

func (m *TutorModel) filterTopics() {
	source := m.topics
	if m.activeTab > 0 && m.activeTab <= len(m.categories) {
		cat := m.categories[m.activeTab-1]
		source = nil
		for _, t := range m.topics {
			if t.Category == cat {
				source = append(source, t)
			}
		}
	}

	if m.searchQuery == "" {
		m.filteredTopics = source
		return
	}

	query := strings.ToLower(m.searchQuery)
	var filtered []TutorTopic
	for _, t := range source {
		if strings.Contains(strings.ToLower(t.Title), query) ||
			strings.Contains(strings.ToLower(t.Content), query) ||
			strings.Contains(strings.ToLower(t.Category), query) {
			filtered = append(filtered, t)
		}
	}
	m.filteredTopics = filtered
}

func (m *TutorModel) updateViewport() {
	if !m.ready {
		return
	}
	m.viewport.SetContent(m.renderTopicContent())
	m.viewport.GotoTop()
}

func (m *TutorModel) innerWidth() int {
	w := max(
		// 1 char padding each side
		m.width-2, 40)
	return w
}

func (m *TutorModel) listWidth() int {
	lw := min(max(m.innerWidth()*30/100, 18), 30)
	return lw
}

func (m *TutorModel) contentWidth() int {
	return m.innerWidth() - m.listWidth() - 1 // -1 for divider
}

func (m *TutorModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewTutor)
}

func (m *TutorModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	vpWidth := m.contentWidth() - 1 // 1 char padding in split pane
	tabBarHeight := 2
	searchHeight := 2
	footerHeight := 2
	vpHeight := height - tabBarHeight - searchHeight - footerHeight - 2 // -2: 1 for spacing, 1 for focus bar

	if vpWidth < 30 {
		vpWidth = 30
	}
	if vpHeight < 5 {
		vpHeight = 5
	}

	if !m.ready {
		m.viewport = viewport.New(viewport.WithWidth(vpWidth), viewport.WithHeight(vpHeight))
		m.viewport.YPosition = 0
		m.ready = true
	} else {
		m.viewport.SetWidth(vpWidth)
		m.viewport.SetHeight(vpHeight)
	}

	m.viewport.SetContent(m.renderTopicContent())
}

// topicLineIndex returns the line number (0-based) of the given topic index
// in the rendered list, accounting for category headers and blank separators.
func (m *TutorModel) topicLineIndex(topicIdx int) int {
	line := 0
	lastCat := ""
	for i, t := range m.filteredTopics {
		if t.Category != lastCat {
			if lastCat != "" {
				line++ // blank separator between categories
			}
			line++ // category header
			lastCat = t.Category
		}
		if i == topicIdx {
			return line
		}
		line++
	}
	return line
}

// listViewHeight returns the visible height for the topic list pane.
func (m *TutorModel) listViewHeight() int {
	return m.viewport.Height()
}

// ensureSelectedVisible adjusts listOffset so the selected item is visible.
// When the selected item is the first in its category, the category header
// (and any blank separator above it) is also kept visible.
func (m *TutorModel) ensureSelectedVisible() {
	if len(m.filteredTopics) == 0 {
		m.listOffset = 0
		return
	}
	h := m.listViewHeight()
	line := m.topicLineIndex(m.selectedIndex)

	// If this is the first topic in its category, include the header above it.
	headerLine := line
	if m.selectedIndex == 0 || m.filteredTopics[m.selectedIndex].Category != m.filteredTopics[m.selectedIndex-1].Category {
		// The category header is the line right before this topic.
		// For non-first categories there's also a blank separator line before the header.
		if m.selectedIndex > 0 {
			headerLine = line - 2 // blank + header
		} else {
			headerLine = line - 1 // just header
		}
		if headerLine < 0 {
			headerLine = 0
		}
	}

	if headerLine < m.listOffset {
		m.listOffset = headerLine
	} else if line >= m.listOffset+h {
		m.listOffset = line - h + 1
	}
}

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
			header := t.Category
			if lipgloss.Width(header) > listW {
				header = header[:listW-1] + "…"
			}
			lines = append(lines, mutedCat.Render(header))
			lastCat = t.Category
		}

		title := t.Title
		maxTitleW := listW - 3
		if lipgloss.Width(title) > maxTitleW {
			title = title[:maxTitleW-1] + "…"
		}

		if idx == m.selectedIndex {
			if m.activePane == 0 {
				if m.searchQuery != "" {
					lines = append(lines, "▸ "+HighlightSubstring(title, m.searchQuery, cursorStyle))
				} else {
					lines = append(lines, cursorStyle.Render("▸ "+title))
				}
			} else {
				if m.searchQuery != "" {
					lines = append(lines, "▸ "+HighlightSubstring(title, m.searchQuery, normalStyle.Bold(true)))
				} else {
					lines = append(lines, normalStyle.Bold(true).Render("▸ "+title))
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
		divider := lipgloss.NewStyle().Foreground(ColorBorder).Render("│")
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
	leftFooter := HelpStyle.Render(fmt.Sprintf("Tab: switch pane (%s) · [ / ]: tabs · /: search · j/k: navigate · Esc: close", paneHint))
	rightFooter := ""
	if m.ready && m.viewport.TotalLineCount() > m.viewport.Height() {
		rightFooter = MutedStyle.Render(fmt.Sprintf("%3.0f%%", m.viewport.ScrollPercent()*100))
	}
	b.WriteString(pad)
	b.WriteString(renderStatusBarFooter(leftFooter, rightFooter, m.innerWidth()))

	return tea.NewView(b.String())
}

func (m *TutorModel) StatusBarLabel() string { return "Tutorial" }
