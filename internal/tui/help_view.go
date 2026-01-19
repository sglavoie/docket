package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// HelpItem represents a single help entry.
type HelpItem struct {
	Key         string // e.g., "j/k", "ctrl+O"
	Description string // e.g., "Move cursor"
	Category    string // e.g., "Navigation", "Actions"
}

// HelpProvider is an interface for views that can provide help items.
type HelpProvider interface {
	GetHelpItems() []HelpItem
}

// HelpViewModel is the model for the help view.
type HelpViewModel struct {
	items            []HelpItem
	filteredItems    []HelpItem
	viewport         viewport.Model
	searchInput      textinput.Model
	searchMode       bool
	searchQuery      string
	width            int
	height           int
	sourceView       ViewType
	viewName         string
	ready            bool
	sectionPositions []int // Line numbers where each section header starts
	currentSection   int   // Current section index for n/N navigation
}

// NewHelpViewModel creates a new help view model.
func NewHelpViewModel(items []HelpItem, sourceView ViewType, viewName string) *HelpViewModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Filter help..."
	searchInput.CharLimit = 50
	searchInput.Width = 30

	m := &HelpViewModel{
		items:       items,
		sourceView:  sourceView,
		viewName:    viewName,
		searchMode:  false,
		searchInput: searchInput,
		searchQuery: "",
		ready:       false,
	}
	m.filteredItems = items
	return m
}

// HasActiveSearch returns true if search mode is active or a search filter is applied.
func (m *HelpViewModel) HasActiveSearch() bool {
	return m.searchMode || m.searchQuery != ""
}

// SetSize sets the view dimensions.
func (m *HelpViewModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	headerHeight := 4 // Title + search bar + spacing
	footerHeight := 2 // Footer hint

	if !m.ready {
		m.viewport = viewport.New(width, height-headerHeight-footerHeight)
		m.viewport.YPosition = headerHeight
		m.ready = true
	} else {
		m.viewport.Width = width
		m.viewport.Height = height - headerHeight - footerHeight
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

	case tea.KeyMsg:
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
		case "up", "k":
			m.viewport.LineUp(1)
		case "down", "j":
			m.viewport.LineDown(1)
		case "g":
			m.viewport.GotoTop()
		case "G":
			m.viewport.GotoBottom()
		case "pgup", "ctrl+u":
			m.viewport.HalfViewUp()
		case "pgdown", "ctrl+d":
			m.viewport.HalfViewDown()
		case "n":
			// Next section (with wrap)
			if len(m.sectionPositions) > 0 {
				startOffset := m.viewport.YOffset
				for i := 0; i < len(m.sectionPositions); i++ {
					m.currentSection++
					if m.currentSection >= len(m.sectionPositions) {
						m.currentSection = 0
					}
					m.goToSection(m.currentSection)
					if m.viewport.YOffset != startOffset {
						break
					}
				}
			}
		case "N":
			// Previous section (with wrap)
			if len(m.sectionPositions) > 0 {
				startOffset := m.viewport.YOffset
				for i := 0; i < len(m.sectionPositions); i++ {
					m.currentSection--
					if m.currentSection < 0 {
						m.currentSection = len(m.sectionPositions) - 1
					}
					m.goToSection(m.currentSection)
					if m.viewport.YOffset != startOffset {
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
	if m.searchQuery == "" {
		m.filteredItems = m.items
		return
	}

	query := strings.ToLower(m.searchQuery)
	var filtered []HelpItem

	for _, item := range m.items {
		keyMatch := strings.Contains(strings.ToLower(item.Key), query)
		descMatch := strings.Contains(strings.ToLower(item.Description), query)
		catMatch := strings.Contains(strings.ToLower(item.Category), query)

		if keyMatch || descMatch || catMatch {
			filtered = append(filtered, item)
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
	categories := make(map[string][]HelpItem)
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

	// Calculate column widths
	// Each column: keyWidth + separator(2) + descWidth
	// Total: 2 * (keyWidth + 2 + descWidth) + columnGap(4)
	keyWidth := 12
	columnGap := 4
	columnOverhead := keyWidth + 2 // key + separator
	descWidth := (m.width - 2*columnOverhead - columnGap) / 2
	if descWidth < 20 {
		descWidth = 20
	}

	keyStyle := lipgloss.NewStyle().
		Width(keyWidth).
		Align(lipgloss.Right).
		Foreground(ColorSecondary)

	descStyle := lipgloss.NewStyle().
		Width(descWidth).
		MaxWidth(descWidth)

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
			// First column
			item1 := items[j]
			col1 := lipgloss.JoinHorizontal(lipgloss.Top,
				keyStyle.Render(item1.Key),
				"  ",
				descStyle.Render(item1.Description),
			)

			// Second column (if exists)
			var col2 string
			if j+1 < len(items) {
				item2 := items[j+1]
				col2 = lipgloss.JoinHorizontal(lipgloss.Top,
					keyStyle.Render(item2.Key),
					"  ",
					descStyle.Render(item2.Description),
				)
			}

			row := lipgloss.JoinHorizontal(lipgloss.Top, col1, "    ", col2)
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

func (m *HelpViewModel) View() string {
	var b strings.Builder

	// Title
	title := fmt.Sprintf("Help - %s", m.viewName)
	b.WriteString(HelpViewTitleStyle.Render(title))
	b.WriteString("\n")

	// Search bar
	if m.searchMode {
		b.WriteString(SubtitleStyle.Render("Filter: "))
		b.WriteString(m.searchInput.View())
		b.WriteString("\n")
	} else if m.searchQuery != "" {
		filterInfo := fmt.Sprintf("Filter: \"%s\" (%d matches) ", m.searchQuery, len(m.filteredItems))
		b.WriteString(HelpFilterActiveStyle.Render(filterInfo))
		b.WriteString(HelpStyle.Render("(/ to edit, Esc to clear)"))
		b.WriteString("\n")
	} else {
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Viewport with content
	if m.ready {
		b.WriteString(m.viewport.View())
	}

	// Footer
	b.WriteString("\n")
	scrollPercent := ""
	if m.ready && m.viewport.TotalLineCount() > m.viewport.Height {
		scrollPercent = fmt.Sprintf(" %3.0f%%", m.viewport.ScrollPercent()*100)
	}
	footer := HelpStyle.Render(fmt.Sprintf("esc: back | ? close | / filter | j/k scroll | n/N sections | g/G top/bottom%s", scrollPercent))
	b.WriteString(footer)

	return b.String()
}

// CloseHelpMsg is sent when the help view should close.
type CloseHelpMsg struct{}

// ShowHelpMsg is sent to show help for a specific view.
type ShowHelpMsg struct {
	SourceView ViewType
	ViewName   string
	Items      []HelpItem
}
