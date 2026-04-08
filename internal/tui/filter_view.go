package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
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
	filter      *CommonFilterState // Used when projectTree is nil (e.g., from outliner)
	project     *models.Project    // Used when projectTree is nil
	items       []FilterItem
	cursor      int
	width       int
	height      int
}

// ShowFilterMsg is sent to open the filter summary view.
type ShowFilterMsg struct {
	ProjectTree  *ProjectTreeModel
	CommonFilter *CommonFilterState // Used when ProjectTree is nil (e.g., from outliner)
	Project      *models.Project    // Used when ProjectTree is nil
}

// CloseFilterMsg is sent when the filter view should close.
type CloseFilterMsg struct{}

// NewFilterViewModel creates a new filter view model.
func NewFilterViewModel(pt *ProjectTreeModel) *FilterViewModel {
	m := &FilterViewModel{
		projectTree: pt,
		filter:      &pt.filter.CommonFilterState,
		project:     pt.project,
		cursor:      0,
	}
	m.buildFilterList()
	return m
}

// NewFilterViewModelForOutliner creates a filter view model backed by a standalone CommonFilterState.
func NewFilterViewModelForOutliner(filter *CommonFilterState, project *models.Project) *FilterViewModel {
	m := &FilterViewModel{
		filter:  filter,
		project: project,
		cursor:  0,
	}
	m.buildFilterList()
	return m
}

// SetSize sets the view dimensions.
func (m *FilterViewModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// buildFilterList scans the filter and creates FilterItem entries.
func (m *FilterViewModel) buildFilterList() {
	m.items = []FilterItem{}
	keyNum := 1

	if m.filter.Type != "" {
		m.items = append(m.items, FilterItem{
			Name:       "Type",
			Value:      string(m.filter.Type),
			Key:        fmt.Sprintf("%d", keyNum),
			FilterType: FilterTypeFilter,
		})
		keyNum++
	}

	if m.filter.Status != "" {
		m.items = append(m.items, FilterItem{
			Name:       "Status",
			Value:      string(m.filter.Status),
			Key:        fmt.Sprintf("%d", keyNum),
			FilterType: FilterStatusFilter,
		})
		keyNum++
	}

	if m.filter.Priority == 1 {
		m.items = append(m.items, FilterItem{
			Name:       "Priority",
			Value:      "high only",
			Key:        fmt.Sprintf("%d", keyNum),
			FilterType: FilterPriorityFilter,
		})
		keyNum++
	}

	if len(m.filter.Tags) > 0 {
		tags := make([]string, 0, len(m.filter.Tags))
		for tag := range m.filter.Tags {
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

	if m.filter.DueDate != DueDateAll {
		var dueName string
		switch m.filter.DueDate {
		case DueDateOverdue:
			dueName = "overdue"
		case DueDateToday:
			dueName = "today"
		case DueDateThisWeek:
			dueName = "this week"
		case DueDateNone:
			dueName = "no due date"
		default:
		}
		m.items = append(m.items, FilterItem{
			Name:       "Due Date",
			Value:      dueName,
			Key:        fmt.Sprintf("%d", keyNum),
			FilterType: FilterDueDateFilter,
		})
		keyNum++
	}

	if m.projectTree != nil && m.projectTree.searchQuery != "" {
		m.items = append(m.items, FilterItem{
			Name:       "Search",
			Value:      fmt.Sprintf("\"%s\"", m.projectTree.searchQuery),
			Key:        fmt.Sprintf("%d", keyNum),
			FilterType: FilterSearchFilter,
		})
	}

	m.cursor = ClampCursor(m.cursor, len(m.items))
}

// clearFilter clears a specific filter type.
func (m *FilterViewModel) clearFilter(filterType FilterType) {
	switch filterType {
	case FilterTypeFilter:
		m.filter.Type = ""
	case FilterStatusFilter:
		m.filter.Status = ""
	case FilterPriorityFilter:
		m.filter.Priority = 0
	case FilterTagFilter:
		m.filter.Tags = make(map[string]bool)
	case FilterDueDateFilter:
		m.filter.DueDate = DueDateAll
	case FilterSearchFilter:
		// Search query is projectTree-specific
		if m.projectTree != nil {
			m.projectTree.searchQuery = ""
			m.projectTree.searchInput.SetValue("")
		}
	default:
	}
	// Update visibility only if projectTree is available
	if m.projectTree != nil {
		m.projectTree.updateVisibility()
		m.projectTree.clampCursor()
	}
	m.buildFilterList()
}

// clearAllFilters clears all active filters.
func (m *FilterViewModel) clearAllFilters() {
	m.filter.Type = ""
	m.filter.Status = ""
	m.filter.Priority = 0
	m.filter.Tags = make(map[string]bool)
	m.filter.DueDate = DueDateAll
	// Search query is projectTree-specific
	if m.projectTree != nil {
		m.projectTree.searchQuery = ""
		m.projectTree.searchInput.SetValue("")
	}
	// Update visibility only if projectTree is available
	if m.projectTree != nil {
		m.projectTree.updateVisibility()
		m.projectTree.clampCursor()
	}
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

	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "ctrl+f":
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
func (m *FilterViewModel) View() tea.View {
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
				cursor = "  "
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

	// Build subtitle
	subtitle := ""
	if len(m.items) > 0 {
		subtitle = fmt.Sprintf("%d active", len(m.items))
	}

	// Footer
	var helpText string
	if len(m.items) > 0 {
		helpText = fmt.Sprintf("q close · 1-%d clear · c clear all", len(m.items))
	} else {
		helpText = "q close"
	}

	var out strings.Builder
	out.WriteString(renderFullscreenTitle("Active Filters", subtitle))
	out.WriteString("\n\n")
	out.WriteString(content.String())
	out.WriteString("\n")
	out.WriteString(renderStatusBarFooter(HelpStyle.Render(helpText), "", m.width))

	return tea.NewView(out.String())
}

func (m *FilterViewModel) StatusBarLabel() string { return "Filters" }
