package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TagFilterSelectorModel allows selecting multiple tags to filter by.
type TagFilterSelectorModel struct {
	BaseSearchableView
	projectTree *ProjectTreeModel
	tags        []TagInfo
	filtered    []TagInfo
	selected    map[string]bool
	cursor      int
}

// ShowTagFilterSelectorMsg is sent to open the tag filter selector.
type ShowTagFilterSelectorMsg struct {
	ProjectTree *ProjectTreeModel
}

// TagFilterSelectorDoneMsg is sent when the tag filter selector is done.
type TagFilterSelectorDoneMsg struct {
	SelectedTags []string
	Cancelled    bool
}

// NewTagFilterSelectorModel creates a new tag filter selector.
func NewTagFilterSelectorModel(pt *ProjectTreeModel) *TagFilterSelectorModel {
	m := &TagFilterSelectorModel{
		projectTree: pt,
		selected:    make(map[string]bool),
		cursor:      0,
	}
	m.InitSearchInput("Type to filter...", 50, 30)
	m.buildTagList()

	// Pre-select currently filtered tags
	for tag := range pt.filterTags {
		m.selected[tag] = true
	}

	return m
}

// SetSize sets the view dimensions.
func (m *TagFilterSelectorModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// buildTagList scans all tasks and builds a sorted list of tags with counts.
func (m *TagFilterSelectorModel) buildTagList() {
	tagCounts := make(map[string]int)
	for _, task := range m.projectTree.project.Tasks {
		for _, tag := range task.Tags {
			tagCounts[tag]++
		}
	}

	m.tags = make([]TagInfo, 0, len(tagCounts))
	for name, count := range tagCounts {
		m.tags = append(m.tags, TagInfo{Name: name, TaskCount: count})
	}

	sort.Slice(m.tags, func(i, j int) bool {
		return m.tags[i].Name < m.tags[j].Name
	})

	m.filtered = m.tags
}

// filterTags filters the tag list by search query.
func (m *TagFilterSelectorModel) filterTagList(query string) {
	if query == "" {
		m.filtered = m.tags
	} else {
		var filtered []TagInfo
		queryLower := strings.ToLower(query)
		for _, tag := range m.tags {
			if strings.Contains(strings.ToLower(tag.Name), queryLower) {
				filtered = append(filtered, tag)
			}
		}
		m.filtered = filtered
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = 0
	}
}

// Init initializes the model.
func (m *TagFilterSelectorModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *TagFilterSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		if m.IsSearchMode() {
			handled, cmd, valueChanged := m.HandleSearchInput(msg)
			if valueChanged {
				m.filterTagList(m.SearchQuery())
			}
			if handled {
				return m, cmd
			}
			return m, cmd
		}

		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg {
				return TagFilterSelectorDoneMsg{Cancelled: true}
			}

		case "enter":
			// Confirm selection
			var selectedTags []string
			for tag := range m.selected {
				selectedTags = append(selectedTags, tag)
			}
			sort.Strings(selectedTags)
			return m, func() tea.Msg {
				return TagFilterSelectorDoneMsg{
					SelectedTags: selectedTags,
					Cancelled:    false,
				}
			}

		case "j", "down":
			m.cursor = WrapCursor(m.cursor, len(m.filtered), 1)
			return m, nil

		case "k", "up":
			m.cursor = WrapCursor(m.cursor, len(m.filtered), -1)
			return m, nil

		case " ":
			// Toggle selection
			if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				tag := m.filtered[m.cursor]
				if m.selected[tag.Name] {
					delete(m.selected, tag.Name)
				} else {
					m.selected[tag.Name] = true
				}
			}
			return m, nil

		case "/":
			return m, m.ActivateSearch()

		case "c":
			// Clear all selections
			m.selected = make(map[string]bool)
			return m, nil
		}
	}

	return m, nil
}

// View renders the tag filter selector.
func (m *TagFilterSelectorModel) View() string {
	// Calculate box width
	boxWidth := 50
	if m.Width() > 0 && m.Width() < boxWidth+10 {
		boxWidth = m.Width() - 10
	}

	// Build content
	var content strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary)
	content.WriteString(titleStyle.Render("Filter by Tags"))
	content.WriteString("\n\n")

	if m.IsSearchMode() {
		content.WriteString("Filter: ")
		content.WriteString(m.SearchInputView())
		content.WriteString("\n\n")
	}

	// Selection count
	selectedCount := len(m.selected)
	if selectedCount == 0 {
		content.WriteString(MutedStyle.Render("No tags selected (showing all)"))
	} else {
		content.WriteString(MutedStyle.Render(fmt.Sprintf("%d tag%s selected", selectedCount, pluralize(selectedCount, "", "s"))))
	}
	content.WriteString("\n\n")

	if len(m.filtered) == 0 {
		if len(m.tags) == 0 {
			content.WriteString(MutedStyle.Render("No tags in project"))
		} else {
			content.WriteString(MutedStyle.Render("No matching tags"))
		}
		content.WriteString("\n")
	} else {
		// Show list with checkboxes
		maxVisible := 15
		if m.Height() > 0 {
			maxVisible = m.Height() - 14
		}
		if maxVisible < 5 {
			maxVisible = 5
		}

		start := 0
		if m.cursor >= maxVisible {
			start = m.cursor - maxVisible + 1
		}
		end := start + maxVisible
		if end > len(m.filtered) {
			end = len(m.filtered)
		}

		for i := start; i < end; i++ {
			tag := m.filtered[i]

			// Checkbox
			checkbox := "[ ]"
			checkStyle := MutedStyle
			if m.selected[tag.Name] {
				checkbox = "[x]"
				checkStyle = SelectionCheckStyle
			}

			// Cursor indicator
			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}

			// Build line
			line := fmt.Sprintf("%s%s %s ", cursor, checkStyle.Render(checkbox), tag.Name)
			countStr := fmt.Sprintf("(%d)", tag.TaskCount)

			if i == m.cursor {
				content.WriteString(TagManagerItemSelectedStyle.Render(line))
			} else {
				content.WriteString(TagManagerItemStyle.Render(line))
			}
			content.WriteString(TagManagerCountStyle.Render(countStr))
			content.WriteString("\n")
		}

		if len(m.filtered) > maxVisible {
			content.WriteString(MutedStyle.Render(fmt.Sprintf("\n... and %d more (use j/k to scroll)", len(m.filtered)-maxVisible)))
			content.WriteString("\n")
		}
	}

	// Footer hint
	content.WriteString("\n")
	if m.IsSearchMode() {
		content.WriteString(MutedStyle.Render("enter done | esc clear"))
	} else if len(m.filtered) > 0 {
		content.WriteString(MutedStyle.Render("space toggle | / search | c clear | enter apply | esc cancel"))
	} else {
		content.WriteString(MutedStyle.Render("enter apply | esc cancel"))
	}

	return RenderCenteredBox(content.String(), boxWidth, m.Width(), m.Height(), TagManagerBoxStyle)
}
