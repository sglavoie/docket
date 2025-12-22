package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sglavoie/tt/internal/models"
)

// SectionInfo holds section name and task count for display.
type SectionInfo struct {
	Name      string
	TaskCount int
}

// SectionFilterSelectorModel allows selecting multiple sections to filter by.
type SectionFilterSelectorModel struct {
	BaseSearchableView
	projectTree *ProjectTreeModel
	sections    []SectionInfo
	filtered    []SectionInfo
	selected    map[string]bool
	cursor      int
}

// ShowSectionFilterSelectorMsg is sent to open the section filter selector.
type ShowSectionFilterSelectorMsg struct {
	ProjectTree *ProjectTreeModel
}

// SectionFilterSelectorDoneMsg is sent when the section filter selector is done.
type SectionFilterSelectorDoneMsg struct {
	SelectedSections []string
	Cancelled        bool
}

// NewSectionFilterSelectorModel creates a new section filter selector.
func NewSectionFilterSelectorModel(pt *ProjectTreeModel) *SectionFilterSelectorModel {
	m := &SectionFilterSelectorModel{
		projectTree: pt,
		selected:    make(map[string]bool),
		cursor:      0,
	}
	m.InitSearchInput("Type to filter...", 50, 30)
	m.buildSectionList()

	// Pre-select currently filtered sections
	for section := range pt.filterSections {
		m.selected[section] = true
	}

	return m
}

// SetSize sets the view dimensions.
func (m *SectionFilterSelectorModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// buildSectionList scans all tasks and builds a sorted list of sections with counts.
func (m *SectionFilterSelectorModel) buildSectionList() {
	sectionCounts := make(map[string]int)

	// Count tasks per section
	for _, task := range m.projectTree.project.Tasks {
		if task.Section != "" {
			sectionCounts[task.Section]++
		}
	}

	// Include all project-defined sections, even if empty
	for _, section := range m.projectTree.project.GetSections() {
		if _, exists := sectionCounts[section]; !exists {
			// Skip empty Backlog
			if section == models.SectionBacklog {
				continue
			}
			sectionCounts[section] = 0
		}
	}

	m.sections = make([]SectionInfo, 0, len(sectionCounts))
	for name, count := range sectionCounts {
		m.sections = append(m.sections, SectionInfo{Name: name, TaskCount: count})
	}

	// Sort: Backlog first (if present), then alphabetically
	sort.Slice(m.sections, func(i, j int) bool {
		if m.sections[i].Name == models.SectionBacklog {
			return true
		}
		if m.sections[j].Name == models.SectionBacklog {
			return false
		}
		return m.sections[i].Name < m.sections[j].Name
	})

	m.filtered = m.sections
}

// filterSectionList filters the section list by search query.
func (m *SectionFilterSelectorModel) filterSectionList(query string) {
	if query == "" {
		m.filtered = m.sections
	} else {
		var filtered []SectionInfo
		queryLower := strings.ToLower(query)
		for _, section := range m.sections {
			if strings.Contains(strings.ToLower(section.Name), queryLower) {
				filtered = append(filtered, section)
			}
		}
		m.filtered = filtered
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = 0
	}
}

// Init initializes the model.
func (m *SectionFilterSelectorModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *SectionFilterSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		if m.IsSearchMode() {
			handled, cmd, valueChanged := m.HandleSearchInput(msg)
			if valueChanged {
				m.filterSectionList(m.SearchQuery())
			}
			if handled {
				return m, cmd
			}
			return m, cmd
		}

		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg {
				return SectionFilterSelectorDoneMsg{Cancelled: true}
			}

		case "enter":
			// Confirm selection
			var selectedSections []string
			for section := range m.selected {
				selectedSections = append(selectedSections, section)
			}
			sort.Strings(selectedSections)
			return m, func() tea.Msg {
				return SectionFilterSelectorDoneMsg{
					SelectedSections: selectedSections,
					Cancelled:        false,
				}
			}

		case "j", "down":
			if len(m.filtered) > 0 {
				m.cursor++
				if m.cursor >= len(m.filtered) {
					m.cursor = 0
				}
			}
			return m, nil

		case "k", "up":
			if len(m.filtered) > 0 {
				m.cursor--
				if m.cursor < 0 {
					m.cursor = len(m.filtered) - 1
				}
			}
			return m, nil

		case " ":
			// Toggle selection
			if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				section := m.filtered[m.cursor]
				if m.selected[section.Name] {
					delete(m.selected, section.Name)
				} else {
					m.selected[section.Name] = true
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

// View renders the section filter selector.
func (m *SectionFilterSelectorModel) View() string {
	var b strings.Builder

	// Calculate box width
	boxWidth := 50
	if m.width > 0 && m.width < boxWidth+10 {
		boxWidth = m.width - 10
	}

	// Build content
	var content strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary)
	content.WriteString(titleStyle.Render("Filter by Sections"))
	content.WriteString("\n\n")

	if m.IsSearchMode() {
		content.WriteString("Filter: ")
		content.WriteString(m.SearchInputView())
		content.WriteString("\n\n")
	}

	// Selection count
	selectedCount := len(m.selected)
	if selectedCount == 0 {
		content.WriteString(MutedStyle.Render("No sections selected (showing all)"))
	} else {
		content.WriteString(MutedStyle.Render(fmt.Sprintf("%d section%s selected", selectedCount, pluralize(selectedCount, "", "s"))))
	}
	content.WriteString("\n\n")

	if len(m.filtered) == 0 {
		if len(m.sections) == 0 {
			content.WriteString(MutedStyle.Render("No sections in project"))
		} else {
			content.WriteString(MutedStyle.Render("No matching sections"))
		}
		content.WriteString("\n")
	} else {
		// Show list with checkboxes
		maxVisible := 15
		if m.height > 0 {
			maxVisible = m.height - 14
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
			section := m.filtered[i]

			// Checkbox
			checkbox := "[ ]"
			checkStyle := MutedStyle
			if m.selected[section.Name] {
				checkbox = "[x]"
				checkStyle = SelectionCheckStyle
			}

			// Cursor indicator
			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}

			// Build line
			line := fmt.Sprintf("%s%s %s ", cursor, checkStyle.Render(checkbox), section.Name)
			countStr := fmt.Sprintf("(%d)", section.TaskCount)

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

	// Apply box styling
	box := TagManagerBoxStyle.Width(boxWidth).Render(content.String())

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
