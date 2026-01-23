package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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

	if m.projectTree.filter.Type != "" {
		m.items = append(m.items, FilterItem{
			Name:       "Type",
			Value:      string(m.projectTree.filter.Type),
			Key:        fmt.Sprintf("%d", keyNum),
			FilterType: FilterTypeFilter,
		})
		keyNum++
	}

	if m.projectTree.filter.Status != "" {
		m.items = append(m.items, FilterItem{
			Name:       "Status",
			Value:      string(m.projectTree.filter.Status),
			Key:        fmt.Sprintf("%d", keyNum),
			FilterType: FilterStatusFilter,
		})
		keyNum++
	}

	if m.projectTree.filter.Priority == 1 {
		m.items = append(m.items, FilterItem{
			Name:       "Priority",
			Value:      "high only",
			Key:        fmt.Sprintf("%d", keyNum),
			FilterType: FilterPriorityFilter,
		})
		keyNum++
	}

	if len(m.projectTree.filter.Tags) > 0 {
		tags := make([]string, 0, len(m.projectTree.filter.Tags))
		for tag := range m.projectTree.filter.Tags {
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

	if m.projectTree.filter.DueDate != DueDateAll {
		var dueName string
		switch m.projectTree.filter.DueDate {
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
		m.projectTree.filter.Type = ""
	case FilterStatusFilter:
		m.projectTree.filter.Status = ""
	case FilterPriorityFilter:
		m.projectTree.filter.Priority = 0
	case FilterTagFilter:
		m.projectTree.filter.Tags = make(map[string]bool)
	case FilterDueDateFilter:
		m.projectTree.filter.DueDate = DueDateAll
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
	m.projectTree.filter.Type = ""
	m.projectTree.filter.Status = ""
	m.projectTree.filter.Priority = 0
	m.projectTree.filter.Tags = make(map[string]bool)
	m.projectTree.filter.DueDate = DueDateAll
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
	// Build content
	var content strings.Builder

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
		content.WriteString(HelpStyle.Render(fmt.Sprintf("q close | 1-%d clear | c clear all", len(m.items))))
	} else {
		content.WriteString(HelpStyle.Render("q close"))
	}

	// Build subtitle showing count
	subtitle := ""
	if len(m.items) > 0 {
		subtitle = fmt.Sprintf("%d active", len(m.items))
	}

	// Calculate box width
	boxWidth := 46
	if m.width > 0 && m.width < boxWidth+10 {
		boxWidth = m.width - 10
	}

	return renderOverlayCard(content.String(), m.width, m.height, OverlayCardConfig{
		Title:    "Active Filters",
		Subtitle: subtitle,
		Width:    boxWidth,
		Centered: true,
	})
}
