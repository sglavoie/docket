package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// FilterType identifies the category of filter.
type FilterType int

const (
	FilterCompleted FilterType = iota
	FilterTypeFilter
	FilterStatusFilter
	FilterPriorityFilter
	FilterTagFilter
	FilterDueDateFilter
	FilterSearchFilter
)

// FilterItem represents a single active filter.
type FilterItem struct {
	Name       string
	Value      string
	Key        string
	FilterType FilterType
}

// FilterViewModel is the model for the filter summary view.
type FilterViewModel struct {
	projectTree *ProjectTreeModel
	items       []FilterItem
	cursor      int
	width       int
	height      int
}

// ShowFilterMsg is sent to open the filter summary view.
type ShowFilterMsg struct {
	ProjectTree *ProjectTreeModel
}

// CloseFilterMsg is sent when the filter view should close.
type CloseFilterMsg struct{}

// NewFilterViewModel creates a new filter view model.
func NewFilterViewModel(pt *ProjectTreeModel) *FilterViewModel {
	m := &FilterViewModel{
		projectTree: pt,
		cursor:      0,
	}
	m.buildFilterList()
	return m
}

// SetSize sets the view dimensions.
func (m *FilterViewModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// buildFilterList scans the ProjectTreeModel and creates FilterItem entries.
func (m *FilterViewModel) buildFilterList() {
	m.items = []FilterItem{}
	keyNum := 1

	if m.projectTree.filterType != "" {
		m.items = append(m.items, FilterItem{
			Name:       "Type",
			Value:      string(m.projectTree.filterType),
			Key:        fmt.Sprintf("%d", keyNum),
			FilterType: FilterTypeFilter,
		})
		keyNum++
	}

	if m.projectTree.filterStatus != "" {
		m.items = append(m.items, FilterItem{
			Name:       "Status",
			Value:      string(m.projectTree.filterStatus),
			Key:        fmt.Sprintf("%d", keyNum),
			FilterType: FilterStatusFilter,
		})
		keyNum++
	}

	if m.projectTree.filterPriority == 1 {
		m.items = append(m.items, FilterItem{
			Name:       "Priority",
			Value:      "high only",
			Key:        fmt.Sprintf("%d", keyNum),
			FilterType: FilterPriorityFilter,
		})
		keyNum++
	}

	if len(m.projectTree.filterTags) > 0 {
		tags := make([]string, 0, len(m.projectTree.filterTags))
		for tag := range m.projectTree.filterTags {
			tags = append(tags, tag)
		}
		sort.Strings(tags)
		m.items = append(m.items, FilterItem{
			Name:       "Tags",
			Value:      strings.Join(tags, ", "),
			Key:        fmt.Sprintf("%d", keyNum),
			FilterType: FilterTagFilter,
		})
		keyNum++
	}

	if m.projectTree.filterDueDate != DueDateAll {
		var dueName string
		switch m.projectTree.filterDueDate {
		case DueDateOverdue:
			dueName = "overdue"
		case DueDateToday:
			dueName = "today"
		case DueDateThisWeek:
			dueName = "this week"
		case DueDateNone:
			dueName = "no due date"
		}
		m.items = append(m.items, FilterItem{
			Name:       "Due Date",
			Value:      dueName,
			Key:        fmt.Sprintf("%d", keyNum),
			FilterType: FilterDueDateFilter,
		})
		keyNum++
	}

	if m.projectTree.searchQuery != "" {
		m.items = append(m.items, FilterItem{
			Name:       "Search",
			Value:      fmt.Sprintf("\"%s\"", m.projectTree.searchQuery),
			Key:        fmt.Sprintf("%d", keyNum),
			FilterType: FilterSearchFilter,
		})
	}

	// Clamp cursor to valid range
	if m.cursor >= len(m.items) {
		m.cursor = len(m.items) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// clearFilter clears a specific filter type.
func (m *FilterViewModel) clearFilter(filterType FilterType) {
	switch filterType {
	case FilterTypeFilter:
		m.projectTree.filterType = ""
	case FilterStatusFilter:
		m.projectTree.filterStatus = ""
	case FilterPriorityFilter:
		m.projectTree.filterPriority = 0
	case FilterTagFilter:
		m.projectTree.filterTags = make(map[string]bool)
	case FilterDueDateFilter:
		m.projectTree.filterDueDate = DueDateAll
	case FilterSearchFilter:
		m.projectTree.searchQuery = ""
		m.projectTree.searchInput.SetValue("")
	}
	m.projectTree.updateVisibility()
	m.projectTree.clampCursor()
	m.buildFilterList()
}

// clearAllFilters clears all active filters.
func (m *FilterViewModel) clearAllFilters() {
	m.projectTree.filterType = ""
	m.projectTree.filterStatus = ""
	m.projectTree.filterPriority = 0
	m.projectTree.filterTags = make(map[string]bool)
	m.projectTree.filterDueDate = DueDateAll
	m.projectTree.searchQuery = ""
	m.projectTree.searchInput.SetValue("")
	m.projectTree.updateVisibility()
	m.projectTree.clampCursor()
	m.buildFilterList()
}

// Init initializes the model.
func (m *FilterViewModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *FilterViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+f":
			return m, func() tea.Msg {
				return CloseFilterMsg{}
			}

		case "j", "down":
			if len(m.items) > 0 {
				m.cursor++
				if m.cursor >= len(m.items) {
					m.cursor = 0
				}
			}
			return m, nil

		case "k", "up":
			if len(m.items) > 0 {
				m.cursor--
				if m.cursor < 0 {
					m.cursor = len(m.items) - 1
				}
			}
			return m, nil

		case "enter", "x":
			// Clear the filter at cursor position
			if len(m.items) > 0 && m.cursor < len(m.items) {
				m.clearFilter(m.items[m.cursor].FilterType)
			}
			return m, nil

		case "c":
			// Clear all filters
			m.clearAllFilters()
			return m, nil

		case "1", "2", "3", "4", "5", "6", "7":
			// Clear filter by number key
			idx := int(msg.String()[0] - '1')
			if idx >= 0 && idx < len(m.items) {
				m.clearFilter(m.items[idx].FilterType)
			}
			return m, nil
		}
	}

	return m, nil
}

// View renders the filter view.
func (m *FilterViewModel) View() string {
	var b strings.Builder

	// Calculate box width
	boxWidth := 46
	if m.width > 0 && m.width < boxWidth+10 {
		boxWidth = m.width - 10
	}

	// Build content
	var content strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary)
	content.WriteString(titleStyle.Render("Active Filters"))
	content.WriteString("\n\n")

	if len(m.items) == 0 {
		content.WriteString(MutedStyle.Render("No filters active"))
		content.WriteString("\n")
	} else {
		// Render each filter item
		for i, item := range m.items {
			cursor := "  "
			style := FilterItemStyle
			if i == m.cursor {
				cursor = "> "
				style = FilterItemSelectedStyle
			}

			line := fmt.Sprintf("%s%s. %s: ", cursor, item.Key, item.Name)
			content.WriteString(style.Render(line))
			content.WriteString(FilterValueStyle.Render(item.Value))
			content.WriteString(FilterClearStyle.Render("  [x]"))
			content.WriteString("\n")
		}

		content.WriteString("\n")
		clearAllLine := "  c  Clear all filters"
		content.WriteString(FilterClearStyle.Render(clearAllLine))
		content.WriteString("\n")
	}

	// Footer hint
	content.WriteString("\n")
	if len(m.items) > 0 {
		content.WriteString(MutedStyle.Render(fmt.Sprintf("q close | 1-%d clear | c clear all", len(m.items))))
	} else {
		content.WriteString(MutedStyle.Render("q close"))
	}

	// Apply box styling
	box := FilterViewBoxStyle.Width(boxWidth).Render(content.String())

	// Center the box
	boxHeight := lipgloss.Height(box)
	topPadding := (m.height - boxHeight) / 2
	if topPadding < 0 {
		topPadding = 0
	}
	leftPadding := (m.width - lipgloss.Width(box)) / 2
	if leftPadding < 0 {
		leftPadding = 0
	}

	// Build final output with padding
	for i := 0; i < topPadding; i++ {
		b.WriteString("\n")
	}

	lines := strings.Split(box, "\n")
	padding := strings.Repeat(" ", leftPadding)
	for _, line := range lines {
		b.WriteString(padding)
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}
