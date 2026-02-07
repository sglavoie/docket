package tui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	searchInput.Width = 25

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

// ShouldCaptureEsc returns true when the view needs to handle esc internally.
func (m *TutorModel) ShouldCaptureEsc() bool {
	return m.searchMode || m.searchQuery != ""
}

func (m *TutorModel) Init() tea.Cmd {
	return nil
}

func (m *TutorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
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
		case "q", "esc":
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
				// Switch tab left
				if m.activeTab > 0 {
					m.activeTab--
				} else {
					m.activeTab = len(m.categories)
				}
				m.filterTopics()
				m.selectedIndex = 0
				m.listOffset = 0
				m.updateViewport()
			}
			return m, nil
		case "l":
			if m.activePane == 0 {
				// Switch tab right
				if m.activeTab < len(m.categories) {
					m.activeTab++
				} else {
					m.activeTab = 0
				}
				m.filterTopics()
				m.selectedIndex = 0
				m.listOffset = 0
				m.updateViewport()
			}
			return m, nil
		case "[":
			if m.activeTab > 0 {
				m.activeTab--
			} else {
				m.activeTab = len(m.categories)
			}
			m.filterTopics()
			m.selectedIndex = 0
			m.listOffset = 0
			m.updateViewport()
			return m, nil
		case "]":
			if m.activeTab < len(m.categories) {
				m.activeTab++
			} else {
				m.activeTab = 0
			}
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
	w := m.width - 2 // 1 char padding each side
	if w < 40 {
		w = 40
	}
	return w
}

func (m *TutorModel) listWidth() int {
	lw := m.innerWidth() * 30 / 100
	if lw < 18 {
		lw = 18
	}
	if lw > 30 {
		lw = 30
	}
	return lw
}

func (m *TutorModel) contentWidth() int {
	return m.innerWidth() - m.listWidth() - 1 // -1 for divider
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
		m.viewport = viewport.New(vpWidth, vpHeight)
		m.viewport.YPosition = 0
		m.ready = true
	} else {
		m.viewport.Width = vpWidth
		m.viewport.Height = vpHeight
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
	return m.viewport.Height
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

	cursorStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
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
				lines = append(lines, cursorStyle.Render("▸ "+title))
			} else {
				lines = append(lines, normalStyle.Bold(true).Render("▸ "+title))
			}
		} else {
			lines = append(lines, "  "+normalStyle.Render(title))
		}
		idx++
	}

	// Apply scroll offset
	m.ensureSelectedVisible()
	h := m.listViewHeight()
	start := m.listOffset
	if start > len(lines) {
		start = len(lines)
	}
	end := start + h
	if end > len(lines) {
		end = len(lines)
	}
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

	contentW := m.contentWidth() - 2 // some padding
	if contentW < 20 {
		contentW = 20
	}

	return renderTutorContent(topic, contentW)
}

// tutorColorRule defines a regex-based colorization rule for tutor content.
type tutorColorRule struct {
	re    *regexp.Regexp
	style lipgloss.Style
	group int // 0 = whole match, >0 = submatch group index to style
}

var tutorColorRules []tutorColorRule

func init() {
	type rule struct {
		pattern string
		style   lipgloss.Style
		group   int
	}

	rules := []rule{
		// Statuses (longest first to match in_progress before partial)
		{`\bin_progress\b`, InProgressStyle, 0},
		{`\bcanceled\b`, GetStatusStyle("canceled"), 0},
		{`\bblocked\b`, BlockedStyle, 0},
		{`\breview\b`, ReviewStyle, 0},
		{`\btodo\b`, TodoStyle, 0},
		{`\bdone\b`, DoneStyle, 0},

		// Task types (singular and plural)
		{`\bEpics?\b`, EpicStyle, 0},
		{`\bStories\b`, StoryStyle, 0},
		{`\bStory\b`, StoryStyle, 0},
		{`\bTasks?\b`, TaskStyle, 0},
		{`\bBugs?\b`, BugStyle, 0},
		{`\bIdeas?\b`, IdeaStyle, 0},

		// "tt" as standalone word
		{`\btt\b`, lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true), 0},

		// Priority patterns: "Priority 1", "!1" through "!5"
		{`Priority 1|!1`, Priority1Style, 0},
		{`Priority 2|!2`, Priority2Style, 0},
		{`Priority 3|!3`, Priority3Style, 0},
		{`Priority 4|!4`, Priority4Style, 0},
		{`Priority 5|!5`, Priority5Style, 0},

		// Section names
		{`\bActive\b`, lipgloss.NewStyle().Foreground(lipgloss.Color("34")), 0},
		{`\bNext\b`, lipgloss.NewStyle().Foreground(lipgloss.Color("61")), 0},
		{`\bInbox\b`, lipgloss.NewStyle().Foreground(lipgloss.Color("33")), 0},
		{`\bWaiting\b`, lipgloss.NewStyle().Foreground(lipgloss.Color("214")), 0},
		{`\bSomeday\b`, lipgloss.NewStyle().Foreground(lipgloss.Color("135")), 0},
		{`\bReview\b`, ReviewStyle, 0},

		// Tags: #word
		{`#[a-zA-Z][a-zA-Z0-9_-]*`, lipgloss.NewStyle().Foreground(ColorPrimary), 0},

		// Contexts
		{`"work"`, lipgloss.NewStyle().Foreground(ColorWarning), 0},
		{`"personal"`, lipgloss.NewStyle().Foreground(ColorWarning), 0},
		{`"side-projects"`, lipgloss.NewStyle().Foreground(ColorWarning), 0},

		// Keybinding highlights (yellow)
		{`Ctrl\+K`, lipgloss.NewStyle().Foreground(ColorWarning), 0},
		{`\bEnter\b`, lipgloss.NewStyle().Foreground(ColorWarning), 0},
		{`\bEsc\b`, lipgloss.NewStyle().Foreground(ColorWarning), 0},
		{`\bTab\b`, lipgloss.NewStyle().Foreground(ColorWarning), 0},
		{`\bj/k\b`, lipgloss.NewStyle().Foreground(ColorWarning), 0},
		// Comma-prefix bindings at start of line: ,s ,d ,b ,o ,x
		{`^(,[sdboxr])\s`, lipgloss.NewStyle().Foreground(ColorWarning), 1},
		// "using , followed" — color the comma
		{`using (,) followed`, lipgloss.NewStyle().Foreground(ColorWarning), 1},
		// "Press X" patterns for single-char keys
		{`Press ([qQ?:@f]) `, lipgloss.NewStyle().Foreground(ColorWarning), 1},
		// "using the s prefix"
		{`the (s) prefix`, lipgloss.NewStyle().Foreground(ColorWarning), 1},
		// "Use !" pattern
		{`Use (!) followed`, lipgloss.NewStyle().Foreground(ColorWarning), 1},
		// "d for days, w for weeks"
		{`(d) for days`, lipgloss.NewStyle().Foreground(ColorWarning), 1},
		{`(w) for weeks`, lipgloss.NewStyle().Foreground(ColorWarning), 1},
		// "set with !", "with #", "with @"
		{`(?:set |tags )with ([!#@])`, lipgloss.NewStyle().Foreground(ColorWarning), 1},
		{`assignment with (@)`, lipgloss.NewStyle().Foreground(ColorWarning), 1},
		// Command palette commands
		{`type (ask ai) to`, lipgloss.NewStyle().Foreground(ColorWarning), 1},
		// Score badge patterns
		{`\[S:\d+\]`, lipgloss.NewStyle().Foreground(lipgloss.Color("42")), 0},
		{`\[S:~\d+\]`, lipgloss.NewStyle().Foreground(lipgloss.Color("208")), 0},
		// Shift+S keybinding
		{`Shift\+S`, lipgloss.NewStyle().Foreground(ColorWarning), 0},
		// FS prefix
		{`\bFS\b`, lipgloss.NewStyle().Foreground(ColorWarning), 0},
		// Fv prefix and sub-keys (Fv, Fvs, Fvl, Fvd)
		{`\bFv[sld]?\b`, lipgloss.NewStyle().Foreground(ColorWarning), 0},
		// :sv palette prefix
		{`:sv\b`, lipgloss.NewStyle().Foreground(ColorWarning), 0},
		// "by s", "then l", "then d" — color the single key after by/then
		{`(?:by|then) ([sld]) `, lipgloss.NewStyle().Foreground(ColorWarning), 1},
	}

	for _, r := range rules {
		tutorColorRules = append(tutorColorRules, tutorColorRule{
			re:    regexp.MustCompile(r.pattern),
			style: r.style,
			group: r.group,
		})
	}
}

// colorizeTutorLine applies keyword colorization to a single line of tutor content.
func colorizeTutorLine(line string) string {
	type replacement struct {
		start, end int
		styled     string
	}

	var replacements []replacement
	occupied := make([]bool, len(line))

	for _, rule := range tutorColorRules {
		if rule.group == 0 {
			matches := rule.re.FindAllStringIndex(line, -1)
			for _, m := range matches {
				overlap := false
				for i := m[0]; i < m[1]; i++ {
					if i < len(occupied) && occupied[i] {
						overlap = true
						break
					}
				}
				if overlap {
					continue
				}
				for i := m[0]; i < m[1]; i++ {
					if i < len(occupied) {
						occupied[i] = true
					}
				}
				replacements = append(replacements, replacement{
					start:  m[0],
					end:    m[1],
					styled: rule.style.Render(line[m[0]:m[1]]),
				})
			}
		} else {
			// Group-based: only style the specified submatch group
			allSubmatch := rule.re.FindAllStringSubmatchIndex(line, -1)
			for _, m := range allSubmatch {
				gi := rule.group * 2
				if gi+1 >= len(m) || m[gi] < 0 {
					continue
				}
				start, end := m[gi], m[gi+1]
				overlap := false
				for i := start; i < end; i++ {
					if i < len(occupied) && occupied[i] {
						overlap = true
						break
					}
				}
				if overlap {
					continue
				}
				// Mark the full match as occupied to prevent other rules matching context
				for i := m[0]; i < m[1]; i++ {
					if i < len(occupied) {
						occupied[i] = true
					}
				}
				replacements = append(replacements, replacement{
					start:  start,
					end:    end,
					styled: rule.style.Render(line[start:end]),
				})
			}
		}
	}

	if len(replacements) == 0 {
		return line
	}

	// Sort by start position (already mostly sorted since we iterate rules in order,
	// but matches within a rule are sorted)
	for i := 1; i < len(replacements); i++ {
		for j := i; j > 0 && replacements[j].start < replacements[j-1].start; j-- {
			replacements[j], replacements[j-1] = replacements[j-1], replacements[j]
		}
	}

	var result strings.Builder
	pos := 0
	for _, r := range replacements {
		result.WriteString(line[pos:r.start])
		result.WriteString(r.styled)
		pos = r.end
	}
	result.WriteString(line[pos:])
	return result.String()
}

func renderTutorContent(topic TutorTopic, width int) string {
	titleStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	headingStyle := lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true)
	codeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
	proseStyle := lipgloss.NewStyle()

	var b strings.Builder
	b.WriteString(titleStyle.Render(colorizeTutorLine(topic.Title)))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(ColorMuted).Render(topic.Category))
	b.WriteString("\n\n")

	for _, line := range strings.Split(topic.Content, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "# "):
			b.WriteString(headingStyle.Render(colorizeTutorLine(trimmed[2:])))
		case strings.HasPrefix(line, "  ") && (strings.HasPrefix(trimmed, "tt ") || strings.HasPrefix(trimmed, "$ ")):
			b.WriteString("  " + codeStyle.Render(colorizeTutorLine(trimmed)))
		case trimmed == "":
			// blank line
		default:
			wrapped := wrapTutorText(trimmed, width)
			colorized := make([]string, 0)
			for _, wl := range strings.Split(wrapped, "\n") {
				colorized = append(colorized, colorizeTutorLine(wl))
			}
			b.WriteString(proseStyle.Render(strings.Join(colorized, "\n")))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// wrapText wraps text to the given width.
func wrapTutorText(text string, width int) string {
	if width <= 0 || len(text) <= width {
		return text
	}

	var lines []string
	remaining := text
	for len(remaining) > width {
		bp := width
		for i := width; i > 0; i-- {
			if remaining[i-1] == ' ' {
				bp = i - 1
				break
			}
		}
		lines = append(lines, remaining[:bp])
		remaining = strings.TrimLeft(remaining[bp:], " ")
	}
	if remaining != "" {
		lines = append(lines, remaining)
	}
	return strings.Join(lines, "\n")
}

func (m *TutorModel) renderTabBar() string {
	tabNames := append([]string{"All"}, m.categories...)
	return renderWrappingTabBar(tabNames, m.activeTab, m.innerWidth())
}

func (m *TutorModel) View() string {
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
		vpH := m.viewport.Height

		// Focus indicator bar
		leftColor := lipgloss.TerminalColor(ColorBorder)
		rightColor := lipgloss.TerminalColor(ColorBorder)
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
	leftFooter := HelpStyle.Render(fmt.Sprintf("Tab: switch pane (%s) | [/]: tabs | /: search | j/k: navigate | q: close", paneHint))
	rightFooter := ""
	if m.ready && m.viewport.TotalLineCount() > m.viewport.Height {
		rightFooter = MutedStyle.Render(fmt.Sprintf("%3.0f%%", m.viewport.ScrollPercent()*100))
	}
	b.WriteString(pad)
	b.WriteString(renderStatusBarFooter(leftFooter, rightFooter, m.innerWidth()))

	return b.String()
}

// tutorTopics returns all tutor topics organized by category.
func tutorTopics() []TutorTopic {
	return []TutorTopic{
		// ── Concepts (alphabetical) ──────────────────────────

		{
			Title:    "Attachments",
			Category: "Concepts",
			Content: `# What Are Attachments

Attach files, URLs, or code references to tasks. Files are
copied into the project's .attachments directory. URLs are
stored as links without downloading.

# Code References

Code references use the file:line format (e.g., src/main.go:42).
These are stored as references without copying and open in your
configured code editor at the specified line.

  tt attach 5 ./report.pdf        # Attach file (copied)
  tt attach 5 --url https://...   # Attach URL (reference)
  tt attach 5 src/main.go:42      # Code reference (opens in IDE)

# Managing Attachments

In the TUI, press a in the attachments view to add. Configure
the code editor and attachment editor in Settings > Attachments.`,
		},
		{
			Title:    "Contexts",
			Category: "Concepts",
			Content: `# What Are Contexts

Contexts group related projects together. You might have a
"work" context, a "personal" context, and a "side-projects"
context. Switching contexts changes which projects appear
in your dashboard and when cycling through projects.

# How Contexts Work

When a context is active, only projects in that context
appear in the dashboard and project list. This keeps your
view focused on the relevant area of your life.`,
		},
		{
			Title:    "Dependencies",
			Category: "Concepts",
			Content: `# Blocking Model

Tasks can depend on other tasks. When a task is blocked by
another, it cannot progress until the blocker is resolved.

# How Blocking Works

When all blockers of a task are completed or canceled, the
task automatically becomes unblocked. Blocked tasks are shown
with a distinct indicator in all views.

# Dependency Chains

Dependencies can form chains where task A blocks B which
blocks C. Completing A unblocks B, and completing B then
unblocks C.`,
		},
		{
			Title:    "Markdown Format",
			Category: "Concepts",
			Content: `# How Tasks Are Stored

tt stores all tasks in human-readable markdown files. Each
project has a tasks.md file in the data directory. You can
read and even edit these files directly if needed.

# Heading Depth = Hierarchy

Task hierarchy is represented by heading depth. Epics use ##,
stories use ###, and leaf tasks use #### (relative to their
parent depth).

  ## Epic: User Authentication
  <!-- id: abc123, status: in_progress, priority: 1 -->

  Description of the epic goes here.

  ### Story: Login Flow
  <!-- id: def456, status: todo, priority: 2 -->

# Metadata in HTML Comments

Task metadata like ID, status, priority, tags, and dates is
stored in HTML comments immediately after the heading. This
keeps the markdown readable while preserving all task data.`,
		},
		{
			Title:    "Notes",
			Category: "Concepts",
			Content: `# What Are Notes

Each project can have associated notes for documentation,
meeting minutes, or reference material. Notes are stored
as markdown files alongside your tasks.

# Global Notes

Global notes are not tied to any project. Use them for
cross-project references or general documentation.

# Linking Notes to Tasks

Notes can be linked to specific tasks for context. This
creates a bidirectional reference visible in both the
note and task views.`,
		},
		{
			Title:    "Priority",
			Category: "Concepts",
			Content: `# Priority Scale

tt uses a 1-5 priority scale where 1 is the highest (critical)
and 5 is the lowest (someday). Priority affects sort order in
all views, with higher-priority tasks appearing first.

# What Each Level Means

Priority 1 (critical) is for urgent, drop-everything items.
Priority 2 (high) is important work that should be done soon.
Priority 3 (normal) is regular work with no special urgency.
Priority 4 (low) is less important work. Priority 5 (someday)
is for things you might do eventually.`,
		},
		{
			Title:    "Recurrence",
			Category: "Concepts",
			Content: `# Recurring Tasks

Recurring tasks automatically repeat on a schedule. When a
recurring task is completed, a new instance is created with
the next due date.

# Recurrence Patterns

Supported patterns include daily, weekly, monthly, and custom
intervals using number + unit: d for days, w for weeks.

# How It Works

When you complete a recurring task, tt creates a new copy with
the status reset to todo and the due date advanced by the
recurrence interval. The completed instance remains in your
history.`,
		},
		{
			Title:    "Saved Views",
			Category: "Concepts",
			Content: `# What Are Saved Views

A saved view is a named preset that captures your current filter,
sort, and view-mode settings. Instead of manually re-applying
filters every time you want to see the same slice of tasks, you
save the combination once and load it with a couple of keystrokes.

# Creating a Saved View

Set up the filters and sort order you want, then press Fv followed
by s (or use the command palette). A dialog asks for a name. Once
saved, the view name appears as a badge in the filter bar whenever
it is active.

# Loading a Saved View

Press Fv then l to open the command palette pre-filtered to saved
views. Select a view and press Enter to apply it. The project tree
immediately updates to match the saved filter/sort/view-mode state.
You can also type :sv in the command palette to browse saved views.

# Deleting a Saved View

Press Fv then d to open the saved views palette. Select the view
you want to remove, press Tab to open the action menu, then choose
Delete. The view is removed from the config file.

# Storage

Saved views are stored in the JSON config file (~/.tt.json) and
are available across all projects. They capture view mode, type/
status/priority/tag/section/due-date/attribute/score filters,
sort field and direction, and the active custom section tab.`,
		},
		{
			Title:    "Sections (GTD)",
			Category: "Concepts",
			Content: `# Section Overview

Sections follow the Getting Things Done methodology to organize
your workflow. Each task belongs to one section that indicates
its current stage.

# Available Sections

Active contains tasks you are working on right now. Next holds
tasks queued up to start soon. Inbox is for unsorted incoming
tasks. Waiting is for tasks blocked on external input. Someday
captures tasks you might do eventually. Review holds tasks
awaiting code review, QA, or approval.

# Custom Sections

Beyond the built-in sections, you can create your own to match
your workflow. Custom sections behave like built-in ones and
can receive tasks, be renamed, or be removed. Assigning a task
to a section that does not exist yet creates it automatically.`,
		},
		{
			Title:    "Scoring",
			Category: "Concepts",
			Content: `# What Is Scoring

Scoring assigns a numeric value to each task based on configurable
dimensions. This helps you prioritize objectively by comparing tasks
on consistent criteria rather than gut feeling alone.

# Profiles

A scoring profile defines dimensions and a formula. Each project
can have one profile assigned. Profiles are reusable across projects.

# Formulas

Five formulas are available. weighted_sum multiplies each dimension
by its weight and sums the results. RICE multiplies Reach, Impact,
and Confidence then divides by Effort. WSJF sums numerator dimensions
and divides by the sum of denominator dimensions. ICE multiplies
Impact, Confidence, and Ease. multiply takes the product of all
dimensions.

# Dimensions

Dimensions can be manual (user-set per task), derived (computed from
task fields like priority or due date), or have a default value when
unset. Each dimension has a min and max range.

# Storage

Dimension values are persisted in task metadata as score.<key>: <value>
entries. The ~ indicator appears when a score uses default values
rather than explicitly set dimensions.

# When to Use Each

WSJF works well in SAFe or agile environments where you need to
prioritize a backlog by cost of delay relative to job size. It
surfaces time-sensitive, high-value work that is cheap to deliver.

ICE is best for quick, lightweight prioritization when you want a
simple subjective score. Each dimension is rated on the same scale,
making it fast to apply without much ceremony.

RICE suits product teams evaluating feature ideas against the effort
to build them. By separating reach from impact and factoring in
confidence, it reduces bias toward flashy but uncertain bets.`,
		},
		{
			Title:    "Snooze",
			Category: "Concepts",
			Content: `# Snoozing Tasks

Snooze temporarily hides a task from active views. The task
reappears automatically when the snooze period expires.

# How Snoozed Tasks Reappear

Snoozed tasks are excluded from normal list views and the
dashboard. When the snooze time passes, the task returns to
its previous section and appears in your task list again.

# Use Cases

Snooze is useful for tasks you cannot act on right now but
do not want to forget. Instead of leaving them in your active
list as noise, snooze them until the right time.`,
		},
		{
			Title:    "Statuses",
			Category: "Concepts",
			Content: `# Available Statuses

Tasks move through six statuses: todo, in_progress, review,
done, blocked, and canceled.

# Transitions

todo is the default status for new tasks. Moving to in_progress
means work has started. review indicates the task is awaiting
code review, QA, or approval. done marks a task as finished.
blocked indicates the task cannot proceed until an external
dependency is resolved. canceled removes the task from active
work without completing it.

# Status Flow

A typical task flows from todo to in_progress to review to
done. Tasks can be blocked at any point and reopened back to
todo. Canceled tasks are terminal and do not appear in active
views.`,
		},
		{
			Title:    "Tags",
			Category: "Concepts",
			Content: `# What Are Tags

Tags are lightweight labels for cross-cutting categorization.
A task can have multiple tags, and tags work across projects.

# Use Cases

Use tags for themes that cut across your project hierarchy:
#backend, #frontend, #urgent, #tech-debt, #design. Tags complement
sections and priority by adding a free-form dimension to task
organization.`,
		},
		{
			Title:    "Task Types",
			Category: "Concepts",
			Content: `# Type Hierarchy

tt supports five task types organized in a hierarchy. Epics are
the largest unit, containing stories, tasks, bugs, and ideas.
Stories can contain tasks and bugs. Tasks, bugs, and ideas are
leaf nodes that cannot contain children.

# When to Use Each

Epics represent large features or initiatives spanning weeks.
Stories are user-facing deliverables within an epic. Tasks are
concrete work items. Bugs track defects. Ideas capture future
possibilities without commitment.`,
		},
		{
			Title:    "Workflow Rules",
			Category: "Concepts",
			Content: `# What Are Workflow Rules

Workflow rules automate repetitive actions. When a trigger
condition is met, the associated action executes automatically.
Triggers can be state changes or events.

# Practical Examples

Auto-move tasks to Active section when started:

  Trigger: status changes to in_progress
  Action: set section to active

Auto-tag completed tasks:

  Trigger: status changes to done
  Action: add tag "completed"`,
		},

		// ── CLI (alphabetical) ────────────────────────────────

		{
			Title:    "Contexts",
			Category: "CLI",
			Content: `# Creating Contexts

  tt context new work
  tt context new personal

# Managing Projects in Contexts

  tt context add work "Project Alpha"
  tt context add work "Project Beta"

# Switching Contexts

Use tt select to switch between projects and contexts.

  tt select`,
		},
		{
			Title:    "Notes",
			Category: "CLI",
			Content: `# Managing Notes

  tt note add "Sprint Planning Notes"
  tt note list
  tt note view <id>

# Global Notes

  tt note add --global "Architecture Decisions"`,
		},
		{
			Title:    "Projects",
			Category: "CLI",
			Content: `# Creating Projects

Every task in tt belongs to a project. Create your first project
and start organizing work immediately.

  tt project new "My Project"
  tt project new "Work Tasks" --context work

# Selecting Projects

Switch between projects to focus on different areas of work. The
selected project is used by all task commands.

  tt select
  tt project list

# Pinning and Archiving

Pin frequently-used projects so they appear at the top of your
dashboard. Archive completed projects to keep things tidy.

  tt project pin "My Project"
  tt project archive "Old Project"`,
		},
		{
			Title:    "Quick Capture",
			Category: "CLI",
			Content: `# Inline Syntax

Add tasks quickly with inline modifiers. Priority is set with !,
tags with #, and section assignment with @.

  tt add "Fix login bug !1 #backend @active"
  tt add "Update docs !3 #frontend"
  tt add "Research caching #perf @next"

# Priority Shorthand

Priority ranges from 1 (highest) to 5 (lowest). Use ! followed by
the number anywhere in the task title.

  tt add "Critical fix !1"
  tt add "Nice to have !4"

# Tags and Sections

Tags help you categorize tasks across projects. Sections follow
GTD methodology for workflow organization.

  tt add "Deploy service #ops #urgent @active"`,
		},
		{
			Title:    "Scoring",
			Category: "CLI",
			Content: `# Managing Scoring Profiles

  tt score profile list
  tt score profile add
  tt score profile show <name>
  tt score profile edit <name>
  tt score profile delete <name>

# Assigning Profiles to Projects

  tt score assign <profile-name>
  tt score unassign

# Setting Dimension Values

  tt score set <task-id> <dimension> <value>
  tt score show <task-id>

# Ranking Tasks

  tt score rank`,
		},
		{
			Title:    "Search",
			Category: "CLI",
			Content: `# Searching Tasks

Search across all tasks in the current project or globally
across all projects.

  tt search "login bug"
  tt search --all "authentication"

# Filtering Options

Combine search with filters to narrow results by status,
priority, tag, or section.

  tt task list --status todo --tag backend
  tt task list --priority 1`,
		},
		{
			Title:    "Task Management",
			Category: "CLI",
			Content: `# Creating Tasks

Add tasks with type and parent options for hierarchy.

  tt task add --type epic "User Authentication"
  tt task add --type story "Login Flow" --parent <epic-id>
  tt task add --type bug "Fix timeout" --parent <story-id>

# Status Transitions

  tt task start <id>
  tt task complete <id>
  tt task cancel <id>
  tt task block <id>

# Priority and Sections

  tt task priority <id> 2
  tt task section <id> active
  tt task section <id> next

# Dependencies

  tt task depends <task-id> <blocker-id>

# Snooze

  tt task snooze <id> 2d
  tt task snooze <id> 1w`,
		},
		{
			Title:    "Workflow Rules",
			Category: "CLI",
			Content: `# Managing Workflow Rules

Create, list, and delete workflow rules from the command line.

  tt workflow add
  tt workflow list
  tt workflow delete <id>`,
		},

		// ── TUI (alphabetical) ────────────────────────────────

		{
			Title:    "Command Palette",
			Category: "TUI",
			Content: `# Opening the Palette

Press : (colon) from any view to open the command palette.
Type to filter commands and press Enter to execute.

# What It Contains

The command palette lists all available actions for the current
view. It is the fastest way to discover and access features
without memorizing keybindings.`,
		},
		{
			Title:    "Help",
			Category: "TUI",
			Content: `# Context-Sensitive Help

Press ? in any view to see all keybindings available in that
screen. The help overlay shows only the actions relevant to
your current view.

# AI Assistant

Press : to open the command palette, then type ask ai to launch
the AI assistant. It can answer questions about your tasks,
suggest next steps, and generate tt commands. Requires the
claude CLI to be installed.`,
		},
		{
			Title:    "Kanban",
			Category: "TUI",
			Content: `# Kanban Board

Press Ctrl+K in the project tree view to open the kanban board.
Tasks are displayed in columns by status, giving a visual
overview of work in progress.`,
		},
		{
			Title:    "Navigation",
			Category: "TUI",
			Content: `# View Stack

The TUI uses a stack-based navigation model. Each screen you open
is pushed onto a stack. Press q to go back to the previous screen,
or Esc to dismiss the current action.

# Movement

Most views use j/k or arrow keys for movement. Enter opens or
selects items. Tab switches between panes in split views.

# Getting Help

Press ? in any view to see all available keybindings for that
screen. Help is context-sensitive and shows only relevant actions.`,
		},
		{
			Title:    "Priority",
			Category: "TUI",
			Content: `# Setting Priority in TUI

Use ! followed by a number to set priority on the selected task:

  !1  Critical (highest)
  !2  High
  !3  Normal
  !4  Low
  !5  Someday (lowest)`,
		},
		{
			Title:    "Sections",
			Category: "TUI",
			Content: `# Section Picker

Press @ on a selected task to open the section picker. Choose
from Active, Next, Inbox, Waiting, or Someday to reassign
the task quickly.`,
		},
		{
			Title:    "Scoring",
			Category: "TUI",
			Content: `# Score Editor

Press Shift+S on a selected task to open the score editor overlay.
This lets you set dimension values for the task interactively.

# Sort by Score

Press 5 in the project tree to sort tasks by their computed score.
Higher scores appear first. Tasks without scores appear at the end.

# Score Badge

Tasks with a scoring profile show a badge like [S:42] in green when
all dimensions are explicitly set, or [S:~42] in orange when some
dimensions use default values. The badge appears in the priority view
and kanban task rows.

# Score in Sidebar

The sidebar shows a score breakdown with each dimension value and
the computed total when a scored task is selected.

# Score Threshold Filter

Use the FS prefix to filter tasks by minimum score. Press F then S
to open the score filter popup. Keys 1 through 5 set thresholds
at 10, 25, 50, 75, and 100 respectively. Press 0 to clear the
filter. Works in both project tree and kanban views.`,
		},
		{
			Title:    "Snooze",
			Category: "TUI",
			Content: `# Snooze Bindings

Snooze a selected task using the s prefix followed by a
duration key. The task will be hidden until the snooze
period expires.`,
		},
		{
			Title:    "Status Changes",
			Category: "TUI",
			Content: `# Comma-Prefix Bindings

Change task status using , followed by a letter:

  ,s  Start (set to in_progress)
  ,d  Mark done
  ,b  Block task
  ,o  Reopen (set to todo)
  ,x  Cancel task
  ,r  Review (send to review)

Each binding sets the status directly. Select a task first,
then press the key combination.`,
		},
		{
			Title:    "Tags",
			Category: "TUI",
			Content: `# Filtering by Tag

Press f to open the filter view. From there you can filter
tasks by tag across all tasks in the current project.`,
		},
	}
}
