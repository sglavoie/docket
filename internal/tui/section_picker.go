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
const removeCustomSectionSentinel = "(remove custom section)"

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

// ShowBulkSectionPickerMsg is sent to open the section picker for bulk assignment.
type ShowBulkSectionPickerMsg struct {
	Tasks   []*models.Task
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
		EmptyMessage:   "No sections in project.",
		NoMatchMessage: "No matching sections.",
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

	// Count GTD sections from task.Section, custom sections from task.CustomSection
	for _, task := range project.Tasks {
		sectionCounts[task.Section]++
	}
	customCounts := make(map[string]int)
	for _, task := range project.Tasks {
		if task.CustomSection != "" {
			customCounts[task.CustomSection]++
		}
	}
	// Merge custom counts into sectionCounts (custom names won't collide with GTD names)
	for name, count := range customCounts {
		sectionCounts[name] = count
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

	// Append sentinel for removing custom section (only if any task has one)
	hasCustom := false
	for _, task := range project.Tasks {
		if task.CustomSection != "" {
			hasCustom = true
			break
		}
	}
	if hasCustom {
		sections = append(sections, SectionInfo{Name: removeCustomSectionSentinel})
	}

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
				// Block section names with spaces
				if err := models.ValidateSectionName(name); err != nil {
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
				// If remove-custom sentinel selected, emit it directly
				if selected.Name == removeCustomSectionSentinel {
					return m, func() tea.Msg {
						return SectionPickerDoneMsg{
							Task:     m.task,
							Project:  m.project,
							Section:  removeCustomSectionSentinel,
							Canceled: false,
						}
					}
				}
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
	var b strings.Builder

	subtitle := fmt.Sprintf("Task #%d: %s", m.task.ShortID, util.Truncate(m.task.Title, 30))
	b.WriteString(renderFullscreenTitle("Assign Section", subtitle))
	b.WriteString("\n\n")

	// Search bar
	b.WriteString(m.RenderSearchBar())

	// Current section — show both GTD and custom when both are set
	var currentParts []string
	if m.task.Section != "" {
		currentParts = append(currentParts, m.task.Section)
	}
	if m.task.CustomSection != "" {
		currentParts = append(currentParts, "@"+m.task.CustomSection)
	}
	currentDisplay := "(no section)"
	if len(currentParts) > 0 {
		currentDisplay = strings.Join(currentParts, " + ")
	}
	b.WriteString(MutedStyle.Render(fmt.Sprintf("Current: %s", currentDisplay)))
	b.WriteString("\n\n")

	// List
	maxVisible := 12
	if m.Height() > 0 {
		maxVisible = m.Height() - 10
	}
	if maxVisible < 5 {
		maxVisible = 5
	}
	rowWidth := m.Width() - 4
	if rowWidth < 30 {
		rowWidth = 30
	}
	b.WriteString(m.renderSectionList(maxVisible, rowWidth))

	// Add mode input
	if m.addMode {
		b.WriteString("\n")
		b.WriteString("New section: ")
		b.WriteString(m.addInput.View())
		b.WriteString("\n")
	}

	// Footer
	var footerLeft string
	if m.addMode {
		footerLeft = HelpStyle.Render("enter save · esc cancel")
	} else {
		footerLeft = HelpStyle.Render(m.RenderFooter())
	}
	right := MutedStyle.Render(fmt.Sprintf("%d items", len(m.Filtered)))
	b.WriteString(renderStatusBarFooter(footerLeft, right, m.Width()))

	return b.String()
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

		// Determine gutter symbol: @ for custom, + for sentinel, - for remove, space for GTD
		var gutter string
		if item.Name == newSectionSentinel {
			gutter = "+"
		} else if item.Name == removeCustomSectionSentinel {
			gutter = "-"
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
		var displayText string
		switch item.Name {
		case newSectionSentinel:
			displayText = "New Section"
		case removeCustomSectionSentinel:
			displayText = "Remove Custom Section"
		default:
			displayText = item.DisplayText()
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

func (m *SectionPickerModel) StatusBarLabel() string { return "Assign Section" }
