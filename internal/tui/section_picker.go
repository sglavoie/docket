package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// SectionPickerModel allows selecting a single section to assign to a task.
type SectionPickerModel struct {
	task        *models.Task
	project     *models.Project
	sections    []SectionInfo
	filtered    []SectionInfo
	cursor      int
	width       int
	height      int
	searchMode  bool
	searchInput textinput.Model
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
	searchInput := textinput.New()
	searchInput.Placeholder = "Type to filter..."
	searchInput.CharLimit = 50
	searchInput.Width = 30

	m := &SectionPickerModel{
		task:        task,
		project:     project,
		cursor:      0,
		searchInput: searchInput,
	}
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

// SetSize sets the view dimensions.
func (m *SectionPickerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
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
		if m.searchMode {
			return m.handleSearchMode(msg)
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

		case "/":
			m.searchMode = true
			m.searchInput.Focus()
			return m, textinput.Blink
		}
	}

	return m, nil
}

// handleSearchMode handles keyboard input while in search mode.
func (m *SectionPickerModel) handleSearchMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searchMode = false
		m.searchInput.Blur()
		m.searchInput.SetValue("")
		m.filterSectionList("")
		return m, nil

	case "enter":
		m.searchMode = false
		m.searchInput.Blur()
		return m, nil

	default:
		oldVal := m.searchInput.Value()
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		if oldVal != m.searchInput.Value() {
			m.filterSectionList(m.searchInput.Value())
		}
		return m, cmd
	}
}

// View renders the section picker.
func (m *SectionPickerModel) View() string {
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
	content.WriteString(titleStyle.Render("Assign Section"))
	content.WriteString("\n")
	content.WriteString(MutedStyle.Render(fmt.Sprintf("Task #%d: %s", m.task.ShortID, util.Truncate(m.task.Title, 30))))
	content.WriteString("\n\n")

	if m.searchMode {
		content.WriteString("Filter: ")
		content.WriteString(m.searchInput.View())
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
		if m.height > 0 {
			maxVisible = m.height - 16
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
	if m.searchMode {
		content.WriteString(MutedStyle.Render("enter done | esc clear"))
	} else if len(m.filtered) > 0 {
		content.WriteString(MutedStyle.Render("enter select | / search | esc cancel"))
	} else {
		content.WriteString(MutedStyle.Render("esc cancel"))
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
