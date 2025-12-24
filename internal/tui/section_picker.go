package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// SectionPickerModel allows selecting a single section to assign to a task.
type SectionPickerModel struct {
	BaseSearchableView
	task     *models.Task
	project  *models.Project
	sections []SectionInfo
	filtered []SectionInfo
	cursor   int
}

// ShowSectionPickerMsg is sent to open the section picker for assignment.
type ShowSectionPickerMsg struct {
	Task    *models.Task
	Project *models.Project
}

// SectionPickerDoneMsg is sent when the section picker is done.
type SectionPickerDoneMsg struct {
	Task      *models.Task
	Project   *models.Project
	Section   string
	Cancelled bool
}

// NewSectionPickerModel creates a new section picker for task assignment.
func NewSectionPickerModel(task *models.Task, project *models.Project) *SectionPickerModel {
	m := &SectionPickerModel{
		task:    task,
		project: project,
		cursor:  0,
	}
	m.InitSearchInput("Type to filter...", 50, 30)
	m.buildSectionList()

	// Pre-select current section
	for i, s := range m.filtered {
		if s.Name == task.Section {
			m.cursor = i
			break
		}
	}

	return m
}

// buildSectionList builds a sorted list of sections with counts.
func (m *SectionPickerModel) buildSectionList() {
	sectionCounts := make(map[string]int)

	// Count tasks per section
	for _, task := range m.project.Tasks {
		if task.Section != "" {
			sectionCounts[task.Section]++
		}
	}

	// Include all project-defined sections, even if empty
	for _, section := range m.project.GetSections() {
		if _, exists := sectionCounts[section]; !exists {
			sectionCounts[section] = 0
		}
	}

	// Add special "No Section" option
	sectionCounts[""] = 0
	for _, task := range m.project.Tasks {
		if task.Section == "" {
			sectionCounts[""]++
		}
	}

	m.sections = make([]SectionInfo, 0, len(sectionCounts))
	for name, count := range sectionCounts {
		m.sections = append(m.sections, SectionInfo{Name: name, TaskCount: count})
	}

	// Sort: Backlog first, then alphabetically, with empty last
	sort.Slice(m.sections, func(i, j int) bool {
		if m.sections[i].Name == "" {
			return false
		}
		if m.sections[j].Name == "" {
			return true
		}
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
func (m *SectionPickerModel) filterSectionList(query string) {
	if query == "" {
		m.filtered = m.sections
	} else {
		var filtered []SectionInfo
		queryLower := strings.ToLower(query)
		for _, section := range m.sections {
			name := section.Name
			if name == "" {
				name = "(no section)"
			}
			if strings.Contains(strings.ToLower(name), queryLower) {
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
func (m *SectionPickerModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *SectionPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
				return SectionPickerDoneMsg{
					Task:      m.task,
					Project:   m.project,
					Cancelled: true,
				}
			}

		case "enter":
			// Select the current section
			if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				section := m.filtered[m.cursor].Name
				return m, func() tea.Msg {
					return SectionPickerDoneMsg{
						Task:      m.task,
						Project:   m.project,
						Section:   section,
						Cancelled: false,
					}
				}
			}
			return m, func() tea.Msg {
				return SectionPickerDoneMsg{
					Task:      m.task,
					Project:   m.project,
					Cancelled: true,
				}
			}

		case "j", "down":
			m.cursor = WrapCursor(m.cursor, len(m.filtered), 1)
			return m, nil

		case "k", "up":
			m.cursor = WrapCursor(m.cursor, len(m.filtered), -1)
			return m, nil

		case "/":
			return m, m.ActivateSearch()
		}
	}

	return m, nil
}

// View renders the section picker.
func (m *SectionPickerModel) View() string {
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
	content.WriteString(titleStyle.Render("Assign Section"))
	content.WriteString("\n")
	content.WriteString(MutedStyle.Render(fmt.Sprintf("Task #%d: %s", m.task.ShortID, util.Truncate(m.task.Title, 30))))
	content.WriteString("\n\n")

	if m.IsSearchMode() {
		content.WriteString("Filter: ")
		content.WriteString(m.SearchInputView())
		content.WriteString("\n\n")
	}

	// Current section
	currentSection := m.task.Section
	if currentSection == "" {
		currentSection = "(no section)"
	}
	content.WriteString(MutedStyle.Render(fmt.Sprintf("Current: %s", currentSection)))
	content.WriteString("\n\n")

	if len(m.filtered) == 0 {
		if len(m.sections) == 0 {
			content.WriteString(MutedStyle.Render("No sections in project"))
		} else {
			content.WriteString(MutedStyle.Render("No matching sections"))
		}
		content.WriteString("\n")
	} else {
		// Show list
		maxVisible := 12
		if m.Height() > 0 {
			maxVisible = m.Height() - 16
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

			// Display name
			displayName := section.Name
			if displayName == "" {
				displayName = "(no section)"
			}

			// Cursor indicator
			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}

			// Build line
			line := fmt.Sprintf("%s%s ", cursor, displayName)
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
		content.WriteString(MutedStyle.Render("enter select | / search | esc cancel"))
	} else {
		content.WriteString(MutedStyle.Render("esc cancel"))
	}

	return RenderCenteredBox(content.String(), boxWidth, m.Width(), m.Height(), TagManagerBoxStyle)
}
