package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

const newSectionSentinel = "+ New Section"

// SectionPickerModel allows selecting a single section to assign to a task.
type SectionPickerModel struct {
	*BasePicker[SectionInfo]
	task     *models.Task
	project  *models.Project
	addMode  bool
	addInput textinput.Model
}

// ShowSectionPickerMsg is sent to open the section picker for assignment.
type ShowSectionPickerMsg struct {
	Task    *models.Task
	Project *models.Project
}

// SectionPickerDoneMsg is sent when the section picker is done.
type SectionPickerDoneMsg struct {
	Task     *models.Task
	Project  *models.Project
	Section  string
	Canceled bool
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
		BoxWidth:       50,
	})

	// Pre-select current section (check CustomSection for non-GTD sections)
	currentSection := task.Section
	if currentSection == "" && task.CustomSection != "" {
		currentSection = task.CustomSection
	}
	picker.SetCursorToID(currentSection)

	return &SectionPickerModel{
		BasePicker: picker,
		task:       task,
		project:    project,
	}
}

// buildSectionPickerList builds a sorted list of sections with counts for assignment.
func buildSectionPickerList(project *models.Project) []SectionInfo {
	sectionCounts := make(map[string]int)

	// Count tasks per section (including empty string for Active)
	for _, task := range project.Tasks {
		sectionCounts[task.Section]++
		if task.CustomSection != "" {
			sectionCounts[task.CustomSection]++
		}
	}

	// Include all standard kanban sections, even if empty
	for _, section := range KanbanColumnOrder {
		if _, exists := sectionCounts[section]; !exists {
			sectionCounts[section] = 0
		}
	}

	// Include all project-defined sections, even if empty
	for _, section := range project.GetSections() {
		if _, exists := sectionCounts[section]; !exists {
			sectionCounts[section] = 0
		}
	}

	sections := make([]SectionInfo, 0, len(sectionCounts))
	for name, count := range sectionCounts {
		colorName := project.GetSectionColor(name)
		isCustom := name != "" && !models.IsGTDSection(name)
		sections = append(sections, SectionInfo{Name: name, TaskCount: count, ColorName: colorName, IsCustom: isCustom})
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

	// Append sentinel for creating a new section
	sections = append(sections, SectionInfo{Name: newSectionSentinel})

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
		// Handle add mode (text input for new section name)
		if m.addMode {
			switch msg.String() {
			case "esc":
				m.addMode = false
				return m, nil
			case "enter":
				name := strings.TrimSpace(m.addInput.Value())
				if name == "" {
					return m, nil
				}
				// Check for duplicates against existing sections
				for _, item := range m.Items {
					if item.Name == name {
						return m, nil
					}
				}
				m.addMode = false
				return m, func() tea.Msg {
					return SectionPickerDoneMsg{
						Task:     m.task,
						Project:  m.project,
						Section:  name,
						Canceled: false,
					}
				}
			default:
				var cmd tea.Cmd
				m.addInput, cmd = m.addInput.Update(msg)
				return m, cmd
			}
		}

		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg {
				return SectionPickerDoneMsg{
					Task:     m.task,
					Project:  m.project,
					Canceled: true,
				}
			}

		case "enter":
			if len(m.Filtered) > 0 && m.Cursor < len(m.Filtered) {
				selected := m.Filtered[m.Cursor]
				// If sentinel selected, switch to add mode
				if selected.Name == newSectionSentinel {
					m.addMode = true
					m.addInput = textinput.New()
					m.addInput.Prompt = ""
					m.addInput.Placeholder = ""
					m.addInput.Focus()
					m.addInput.CharLimit = 50
					return m, textinput.Blink
				}
				section := selected.Name
				return m, func() tea.Msg {
					return SectionPickerDoneMsg{
						Task:     m.task,
						Project:  m.project,
						Section:  section,
						Canceled: false,
					}
				}
			}
			return m, func() tea.Msg {
				return SectionPickerDoneMsg{
					Task:     m.task,
					Project:  m.project,
					Canceled: true,
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
	if currentSection == "" && m.task.CustomSection != "" {
		currentSection = m.task.CustomSection
	}
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
	if maxVisible < 5 {
		maxVisible = 5
	}
	content.WriteString(m.renderSectionList(maxVisible, boxWidth))

	// Footer
	content.WriteString("\n")
	if m.addMode {
		content.WriteString("New section: ")
		content.WriteString(m.addInput.View())
		content.WriteString("\n")
		content.WriteString(MutedStyle.Render("enter save | esc cancel"))
	} else {
		content.WriteString(m.RenderFooter())
	}

	subtitle := fmt.Sprintf("Task #%d: %s", m.task.ShortID, util.Truncate(m.task.Title, 30))

	return renderOverlayCard(content.String(), m.Width(), m.Height(), OverlayCardConfig{
		Title:    "Assign Section",
		Subtitle: subtitle,
		Width:    boxWidth,
		Centered: true,
	})
}

// renderSectionList renders the section list with gutter symbols for custom sections.
func (m *SectionPickerModel) renderSectionList(maxVisible, boxWidth int) string {
	var b strings.Builder

	if len(m.Filtered) == 0 {
		if len(m.Items) == 0 {
			b.WriteString(MutedStyle.Render(m.Config.EmptyMessage))
		} else {
			b.WriteString(MutedStyle.Render(m.Config.NoMatchMessage))
		}
		b.WriteString("\n")
		return b.String()
	}

	start := 0
	if m.Cursor >= maxVisible {
		start = m.Cursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(m.Filtered) {
		end = len(m.Filtered)
	}

	rowWidth := boxWidth - 6

	for i := start; i < end; i++ {
		item := m.Filtered[i]
		selected := i == m.Cursor

		// Determine gutter symbol: @ for custom, + for sentinel, space for GTD
		var gutter string
		if item.Name == newSectionSentinel {
			gutter = "+"
		} else if item.IsCustom {
			gutter = "@"
		} else {
			gutter = " "
		}

		// Build prefix: "> @ " or "  @ " or ">   " or "    "
		var prefix string
		if selected {
			prefix = SelectionCursorStyle.Render(">") + " " + gutter + " "
		} else {
			prefix = "  " + gutter + " "
		}

		// Display text (without @ prefix, that's in the gutter)
		displayText := item.DisplayText()
		if item.Name == newSectionSentinel {
			displayText = "New Section"
		}
		countStr := TagManagerCountStyle.Render(fmt.Sprintf(" (%d)", item.CountValue()))

		row := prefix + displayText + countStr
		if selected {
			b.WriteString(SelectionRowStyle.Width(rowWidth).Render(row))
		} else {
			b.WriteString(row)
		}
		b.WriteString("\n")
	}

	if len(m.Filtered) > maxVisible {
		remaining := len(m.Filtered) - end
		if remaining > 0 {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("\n... and %d more (use j/k to scroll)", remaining)))
			b.WriteString("\n")
		}
	}

	return b.String()
}

// ShouldCaptureEsc returns true when in add mode so esc is handled by the view.
func (m *SectionPickerModel) ShouldCaptureEsc() bool {
	return m.addMode
}

// HasTextInput returns true when in add mode.
func (m *SectionPickerModel) HasTextInput() bool {
	return m.addMode
}
