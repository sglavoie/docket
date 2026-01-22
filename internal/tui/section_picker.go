package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// SectionPickerModel allows selecting a single section to assign to a task.
type SectionPickerModel struct {
	*BasePicker[SectionInfo]
	task    *models.Task
	project *models.Project
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
	sections := buildSectionPickerList(project)

	picker := NewBasePicker(sections, PickerConfig{
		Title:          "Assign Section",
		EmptyMessage:   "No sections in project",
		NoMatchMessage: "No matching sections",
		MultiSelect:    false,
		ShowCounts:     true,
	})

	// Pre-select current section
	picker.SetCursorToID(task.Section)

	return &SectionPickerModel{
		BasePicker: picker,
		task:       task,
		project:    project,
	}
}

// buildSectionPickerList builds a sorted list of sections with counts for assignment.
func buildSectionPickerList(project *models.Project) []SectionInfo {
	sectionCounts := make(map[string]int)

	// Count tasks per section
	for _, task := range project.Tasks {
		if task.Section != "" {
			sectionCounts[task.Section]++
		}
	}

	// Include all project-defined sections, even if empty
	for _, section := range project.GetSections() {
		if _, exists := sectionCounts[section]; !exists {
			sectionCounts[section] = 0
		}
	}

	// Add special "No Section" option
	sectionCounts[""] = 0
	for _, task := range project.Tasks {
		if task.Section == "" {
			sectionCounts[""]++
		}
	}

	sections := make([]SectionInfo, 0, len(sectionCounts))
	for name, count := range sectionCounts {
		sections = append(sections, SectionInfo{Name: name, TaskCount: count})
	}

	// Sort: Next first, then alphabetically, with empty last
	sort.Slice(sections, func(i, j int) bool {
		if sections[i].Name == "" {
			return false
		}
		if sections[j].Name == "" {
			return true
		}
		if sections[i].Name == models.SectionNext {
			return true
		}
		if sections[j].Name == models.SectionNext {
			return false
		}
		return sections[i].Name < sections[j].Name
	})

	return sections
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
			if len(m.Filtered) > 0 && m.Cursor < len(m.Filtered) {
				section := m.Filtered[m.Cursor].Name
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

		default:
			handled, cmd := m.HandleKeyMsg(msg)
			if handled {
				return m, cmd
			}
		}
	}

	return m, nil
}

// View renders the section picker.
func (m *SectionPickerModel) View() string {
	boxWidth := 50
	if m.Width() > 0 && m.Width() < boxWidth+10 {
		boxWidth = m.Width() - 10
	}

	var content strings.Builder

	// Search bar
	content.WriteString(m.RenderSearchBar())

	// Current section
	currentSection := m.task.Section
	if currentSection == "" {
		currentSection = "(no section)"
	}
	content.WriteString(MutedStyle.Render(fmt.Sprintf("Current: %s", currentSection)))
	content.WriteString("\n\n")

	// List
	maxVisible := 12
	if m.Height() > 0 {
		maxVisible = m.Height() - 16
	}
	content.WriteString(m.RenderList(maxVisible))

	// Footer
	content.WriteString("\n")
	content.WriteString(m.RenderFooter())

	subtitle := fmt.Sprintf("Task #%d: %s", m.task.ShortID, util.Truncate(m.task.Title, 30))

	return renderOverlayCard(content.String(), m.Width(), m.Height(), OverlayCardConfig{
		Title:    "Assign Section",
		Subtitle: subtitle,
		Width:    boxWidth,
		Centered: true,
	})
}
